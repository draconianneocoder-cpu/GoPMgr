// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//! DF0-C2: one read-only source, reachable only through its host capability.

use std::fmt::{Debug, Display, Formatter};
use std::ops::Range;
use std::sync::Arc;

use async_trait::async_trait;
use bytes::Bytes;
use datafusion::execution::disk_manager::{DiskManagerBuilder, DiskManagerMode};
use datafusion::execution::object_store::{DefaultObjectStoreRegistry, ObjectStoreRegistry};
use datafusion::execution::runtime_env::RuntimeEnvBuilder;
use datafusion::object_store::path::Path;
use datafusion::object_store::{
    Attributes, CopyOptions, GetOptions, GetResult, GetResultPayload, ListResult, MultipartUpload,
    ObjectMeta, ObjectStore, PutMultipartOptions, PutOptions, PutPayload, PutResult,
};
use datafusion::physical_plan::SendableRecordBatchStream;
use datafusion::prelude::{ParquetReadOptions, SessionConfig, SessionContext, col, lit};
use futures::StreamExt;
use futures::stream::{self, BoxStream};
use url::Url;

const OBJECT: &str = "source.parquet";
const STORE: &str = "gopmgr://job/";
const SOURCE_URL: &str = "gopmgr://job/source.parquet";
pub const MAX_SOURCE: u64 = 64 << 20;

#[async_trait]
pub trait ReadAt: Debug + Send + Sync + 'static {
    async fn read(&self, offset: u64, length: u32) -> Result<Bytes, String>;
}

#[derive(Debug)]
pub struct HostSource {
    reader: Arc<dyn ReadAt>,
    size: u64,
    version: String,
    max_read: u32,
}

impl HostSource {
    pub fn new(
        reader: Arc<dyn ReadAt>,
        size: u64,
        version: String,
        max_read: u32,
    ) -> Result<Self, String> {
        if size > MAX_SOURCE
            || max_read == 0
            || u64::from(max_read) > MAX_SOURCE
            || version.len() != 64
            || !version.bytes().all(|b| b.is_ascii_hexdigit())
        {
            return Err("invalid host source descriptor".into());
        }
        Ok(Self {
            reader,
            size,
            version,
            max_read,
        })
    }
}

fn error(message: impl Into<String>) -> datafusion::object_store::Error {
    datafusion::object_store::Error::Generic {
        store: "DF0 host source",
        source: Box::new(std::io::Error::other(message.into())),
    }
}

#[derive(Debug)]
struct SourceStore(HostSource);

impl Display for SourceStore {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.write_str("DF0 host source")
    }
}

impl SourceStore {
    fn meta(&self) -> ObjectMeta {
        ObjectMeta {
            location: Path::from(OBJECT),
            last_modified: std::time::UNIX_EPOCH.into(),
            size: self.0.size,
            e_tag: Some(self.0.version.clone()),
            version: Some(self.0.version.clone()),
        }
    }

    fn listed(&self, prefix: Option<&Path>) -> Vec<ObjectMeta> {
        if prefix.is_none_or(|p| p.as_ref().is_empty() || p.as_ref() == OBJECT) {
            vec![self.meta()]
        } else {
            vec![]
        }
    }
}

#[async_trait]
impl ObjectStore for SourceStore {
    async fn put_opts(
        &self,
        _: &Path,
        _: PutPayload,
        _: PutOptions,
    ) -> datafusion::object_store::Result<PutResult> {
        Err(error("read-only source"))
    }
    async fn put_multipart_opts(
        &self,
        _: &Path,
        _: PutMultipartOptions,
    ) -> datafusion::object_store::Result<Box<dyn MultipartUpload>> {
        Err(error("read-only source"))
    }

    async fn get_opts(
        &self,
        location: &Path,
        options: GetOptions,
    ) -> datafusion::object_store::Result<GetResult> {
        if location.as_ref() != OBJECT {
            return Err(error("source path refused"));
        }
        let meta = self.meta();
        options.check_preconditions(&meta)?;
        if options
            .version
            .as_ref()
            .is_some_and(|v| v != &self.0.version)
        {
            return Err(error("source version refused"));
        }
        let range = if options.head {
            0..0
        } else {
            match options.range {
                Some(r) => r.as_range(self.0.size).map_err(|e| error(e.to_string()))?,
                None => 0..self.0.size,
            }
        };
        let end = range.end;
        let max_read = self.0.max_read;
        let reader = self.0.reader.clone();
        let payload =
            stream::try_unfold((range.start, reader), move |(offset, reader)| async move {
                if offset >= end {
                    return Ok(None);
                }
                let length = (end - offset).min(u64::from(max_read)) as u32;
                let data = reader.read(offset, length).await.map_err(error)?;
                if data.len() != length as usize {
                    return Err(error("source response length mismatch"));
                }
                Ok(Some((data, (offset + u64::from(length), reader))))
            })
            .boxed();
        Ok(GetResult {
            payload: GetResultPayload::Stream(payload),
            meta,
            range,
            attributes: Attributes::default(),
        })
    }

    fn delete_stream(
        &self,
        _: BoxStream<'static, datafusion::object_store::Result<Path>>,
    ) -> BoxStream<'static, datafusion::object_store::Result<Path>> {
        stream::once(async { Err(error("read-only source")) }).boxed()
    }
    fn list(
        &self,
        prefix: Option<&Path>,
    ) -> BoxStream<'static, datafusion::object_store::Result<ObjectMeta>> {
        stream::iter(self.listed(prefix).into_iter().map(Ok)).boxed()
    }
    async fn list_with_delimiter(
        &self,
        prefix: Option<&Path>,
    ) -> datafusion::object_store::Result<ListResult> {
        Ok(ListResult {
            common_prefixes: vec![],
            objects: self.listed(prefix),
        })
    }
    async fn copy_opts(
        &self,
        _: &Path,
        _: &Path,
        _: CopyOptions,
    ) -> datafusion::object_store::Result<()> {
        Err(error("read-only source"))
    }
}

fn session(store: Arc<dyn ObjectStore>, batch_rows: usize) -> Result<SessionContext, String> {
    let registry = DefaultObjectStoreRegistry::default();
    registry
        .deregister_store(&Url::parse("file:///").map_err(|e| e.to_string())?)
        .map_err(|e| e.to_string())?;
    registry.register_store(&Url::parse(STORE).map_err(|e| e.to_string())?, store);
    let runtime = RuntimeEnvBuilder::new()
        .with_object_store_registry(Arc::new(registry))
        .with_disk_manager_builder(
            DiskManagerBuilder::default().with_mode(DiskManagerMode::Disabled),
        )
        .with_memory_limit(64 << 20, 1.0)
        .build()
        .map_err(|e| e.to_string())?;
    let config = SessionConfig::new()
        .with_target_partitions(1)
        .with_information_schema(false)
        .with_batch_size(batch_rows.clamp(1, 8192));
    Ok(SessionContext::new_with_config_rt(
        config,
        Arc::new(runtime),
    ))
}

#[derive(Debug, Default)]
pub struct Query {
    pub projection: Vec<String>,
    pub id_range: Option<Range<i64>>,
    pub batch_rows: usize,
}

pub async fn parquet(
    source: HostSource,
    query: Query,
) -> Result<SendableRecordBatchStream, String> {
    let ctx = session(Arc::new(SourceStore(source)), query.batch_rows)?;
    let mut df = ctx
        .read_parquet(SOURCE_URL, ParquetReadOptions::default())
        .await
        .map_err(|e| e.to_string())?;
    if let Some(range) = query.id_range {
        if range.start > range.end {
            return Err("invalid ID range".into());
        }
        df = df
            .filter(
                col("id")
                    .gt_eq(lit(range.start))
                    .and(col("id").lt(lit(range.end))),
            )
            .map_err(|e| e.to_string())?;
    }
    if !query.projection.is_empty() {
        let columns: Vec<&str> = query.projection.iter().map(String::as_str).collect();
        df = df.select_columns(&columns).map_err(|e| e.to_string())?;
    }
    df.limit(0, Some(crate::PREVIEW_ROWS + 1))
        .map_err(|e| e.to_string())?
        .execute_stream()
        .await
        .map_err(|e| e.to_string())
}

pub struct MemoryReadAt(pub Bytes);

impl Debug for MemoryReadAt {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("MemoryReadAt")
            .field("bytes", &self.0.len())
            .finish()
    }
}

#[async_trait]
impl ReadAt for MemoryReadAt {
    async fn read(&self, offset: u64, length: u32) -> Result<Bytes, String> {
        if offset > self.0.len() as u64 || u64::from(length) > self.0.len() as u64 - offset {
            return Err("source range refused".into());
        }
        Ok(self
            .0
            .slice(offset as usize..offset as usize + length as usize))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use datafusion::object_store::{GetRange, ObjectStoreExt};
    use std::sync::atomic::{AtomicUsize, Ordering};

    #[derive(Debug)]
    struct Reader {
        calls: AtomicUsize,
        short: bool,
    }
    #[async_trait]
    impl ReadAt for Reader {
        async fn read(&self, _: u64, length: u32) -> Result<Bytes, String> {
            self.calls.fetch_add(1, Ordering::SeqCst);
            Ok(Bytes::from(vec![
                b'x';
                length as usize - usize::from(self.short)
            ]))
        }
    }
    fn store(reader: Arc<dyn ReadAt>) -> SourceStore {
        SourceStore(HostSource::new(reader, 10, "a".repeat(64), 3).unwrap())
    }

    #[tokio::test]
    async fn metadata_ranges_and_read_only_boundary() {
        let reader = Arc::new(Reader {
            calls: AtomicUsize::new(0),
            short: false,
        });
        let store = store(reader.clone());
        let path = Path::from(OBJECT);
        assert_eq!(store.head(&path).await.unwrap().size, 10);
        assert_eq!(reader.calls.load(Ordering::SeqCst), 0);
        let opts = GetOptions {
            range: Some(GetRange::Bounded(2..9)),
            ..Default::default()
        };
        let result = store.get_opts(&path, opts).await.unwrap();
        assert_eq!(result.range, 2..9);
        assert_eq!(
            result.bytes().await.unwrap(),
            Bytes::from_static(b"xxxxxxx")
        );
        assert_eq!(reader.calls.load(Ordering::SeqCst), 3);
        assert!(store.get(&Path::from("another.parquet")).await.is_err());
        assert!(
            store
                .get_opts(
                    &path,
                    GetOptions {
                        version: Some("b".repeat(64)),
                        ..Default::default()
                    }
                )
                .await
                .is_err()
        );
        assert!(store.put(&path, Bytes::new().into()).await.is_err());
        assert!(store.put_multipart(&path).await.is_err());
        assert!(store.delete(&path).await.is_err());
        assert!(store.copy(&path, &path).await.is_err());
        assert!(
            store
                .list(Some(&Path::from("another")))
                .collect::<Vec<_>>()
                .await
                .is_empty()
        );
        assert_eq!(reader.calls.load(Ordering::SeqCst), 3);
    }

    #[tokio::test]
    async fn refuses_short_source_and_other_store_providers() {
        let store = Arc::new(store(Arc::new(Reader {
            calls: AtomicUsize::new(0),
            short: true,
        })));
        assert!(store.get_range(&Path::from(OBJECT), 0..2).await.is_err());
        let ctx = session(store, 128).unwrap();
        for url in [
            "file:///tmp/private.parquet",
            "https://example.invalid/data.parquet",
            "s3://bucket/key",
        ] {
            assert!(
                ctx.runtime_env()
                    .object_store_registry
                    .get_store(&Url::parse(url).unwrap())
                    .is_err(),
                "provider enabled: {url}"
            );
        }
    }

    #[tokio::test]
    async fn memory_boundaries_and_job_isolation() {
        let a = MemoryReadAt(Bytes::from_static(b"secretA"));
        let b = MemoryReadAt(Bytes::from_static(b"secretB"));
        assert_eq!(a.read(6, 1).await.unwrap(), Bytes::from_static(b"A"));
        assert_eq!(b.read(6, 1).await.unwrap(), Bytes::from_static(b"B"));
        assert!(a.read(u64::MAX, 2).await.is_err());
        assert!(a.read(6, 2).await.is_err());
        assert_eq!(a.read(7, 0).await.unwrap().len(), 0);
        assert!(!format!("{a:?}").contains("secret"));
        assert!(HostSource::new(Arc::new(a), MAX_SOURCE + 1, "a".repeat(64), 1).is_err());
        assert!(HostSource::new(Arc::new(b), 7, "a".repeat(64), 0).is_err());
    }
}
