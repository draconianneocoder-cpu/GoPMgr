// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

use std::sync::Arc;
use std::time::Duration;

use async_trait::async_trait;
use bytes::Bytes;
use datafusion::common::DataFusionError;
use df0_engine::host_source::{HostSource, MAX_SOURCE, MemoryReadAt, Query, ReadAt};
use sha2::{Digest, Sha256};
use tokio::sync::{Mutex, mpsc, oneshot};
use tokio::task::JoinSet;
use tokio_stream::StreamExt;

use super::pb::{Batch, Completed, Execute, SourceReadRequest, SourceReadResponse, worker_event};
use super::{Control, OpResult, Out, frames, now_ms, peak_rss};

pub(super) struct Inputs {
    pub chunks: mpsc::Receiver<Vec<u8>>,
    pub replies: mpsc::Receiver<SourceReadResponse>,
    pub operation_id: String,
}

struct Call {
    offset: u64,
    length: u32,
    reply: oneshot::Sender<Result<Bytes, String>>,
}

struct RemoteReadAt {
    calls: mpsc::Sender<Call>,
    serial: Mutex<()>,
}

impl std::fmt::Debug for RemoteReadAt {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("RemoteReadAt")
    }
}

#[async_trait]
impl ReadAt for RemoteReadAt {
    async fn read(&self, offset: u64, length: u32) -> Result<Bytes, String> {
        let _credit = self.serial.lock().await;
        let (reply, rx) = oneshot::channel();
        self.calls
            .send(Call {
                offset,
                length,
                reply,
            })
            .await
            .map_err(|_| "source job closed".to_owned())?;
        rx.await.map_err(|_| "source job closed".to_owned())?
    }
}

pub(super) async fn execute(
    ex: &Execute,
    out: &mut Out,
    mut inputs: Inputs,
    ctl: &Control,
    cap: usize,
) -> OpResult {
    let invalid = |s: String| ("INPUT_INVALID", s);
    let desc = ex
        .source_descriptor
        .as_ref()
        .ok_or_else(|| invalid("missing source descriptor".into()))?;
    if desc.source_id.len() != 32
        || !desc.source_id.bytes().all(|b| b.is_ascii_hexdigit())
        || desc.size > MAX_SOURCE
        || desc.sha256.len() != 32
        || hex::encode(&desc.sha256) != desc.version
        || desc.max_read_bytes == 0
        || desc.max_read_bytes as usize + 1024 > cap
        || desc.expires_unix_ms <= now_ms()
    {
        return Err(invalid("invalid or expired source descriptor".into()));
    }
    let (calls, mut requests) = mpsc::channel::<Call>(1);
    let mut source_hash = Sha256::new();
    let mut source_bytes = 0u64;
    let range_mode = ex.operation == "parquet_range";
    let reader: Arc<dyn ReadAt> = if range_mode {
        Arc::new(RemoteReadAt {
            calls,
            serial: Mutex::new(()),
        })
    } else {
        drop(calls);
        let mut data = Vec::new();
        loop {
            let chunk = tokio::select! {
                biased;
                _ = ctl.wait_cancel() => return Err(("USER_CANCELLED", "cancelled by the host".into())),
                chunk = inputs.chunks.recv() => chunk,
            };
            let Some(chunk) = chunk else { break };
            if data.len() as u64 + chunk.len() as u64 > desc.size {
                return Err(invalid("source exceeded descriptor size".into()));
            }
            source_hash.update(&chunk);
            data.extend_from_slice(&chunk);
        }
        if data.len() as u64 != desc.size
            || source_hash.clone().finalize().as_slice() != desc.sha256.as_slice()
        {
            return Err(invalid("source identity mismatch".into()));
        }
        source_bytes = data.len() as u64;
        Arc::new(MemoryReadAt(Bytes::from(data)))
    };
    let source = HostSource::new(reader, desc.size, desc.version.clone(), desc.max_read_bytes)
        .map_err(invalid)?;
    if !ex.spill_control_directory.is_empty() && ex.fault != "source_spill_control" {
        return Err(invalid(
            "spill directory requires synthetic positive control".into(),
        ));
    }
    let memory_bytes = usize::try_from(ex.query_memory_bytes)
        .map_err(|_| invalid("source memory budget overflow".into()))?;
    let query = Query {
        projection: ex.projection.clone(),
        id_range: ex.filter_id.then_some(ex.lower_id..ex.upper_id),
        batch_rows: ex.batch_rows as usize,
        memory_bytes,
        sort_source: ex.sort_source,
        spill_control_directory: (!ex.spill_control_directory.is_empty())
            .then(|| ex.spill_control_directory.clone().into()),
    };
    let (batch_tx, mut batch_rx) = mpsc::channel(1);
    let mut tasks = JoinSet::new();
    tasks.spawn(async move {
        let mut execution = df0_engine::host_source::parquet(source, query).await?;
        while let Some(batch) = execution.stream.next().await {
            batch_tx
                .send(batch?)
                .await
                .map_err(|_| DataFusionError::Execution("query receiver closed".into()))?;
        }
        Ok::<_, DataFusionError>(execution.metrics())
    });
    let mut batches = 0u64;
    let mut rows = 0u64;
    let mut batch_hash = Sha256::new();
    let mut request_id = 0u64;
    let mut query_metrics = None;
    let outcome: OpResult = async {
        loop {
            tokio::select! {
                biased;
                _ = ctl.wait_cancel() => return Err(("USER_CANCELLED", "cancelled by the host".into())),
                Some(call) = requests.recv() => {
                    request_id += 1;
                    let mut req = SourceReadRequest {
                        operation_id: inputs.operation_id.clone(),
                        source_id: desc.source_id.clone(),
                        version: desc.version.clone(),
                        request_id,
                        offset: call.offset,
                        length: call.length,
                    };
                    match ex.fault.as_str() {
                        "source_bad_job" => req.operation_id.push('x'),
                        "source_bad_handle" => req.source_id.replace_range(
                            0..1, if desc.source_id.starts_with('a') { "b" } else { "a" }),
                        "source_bad_version" => req.version.replace_range(
                            0..1, if desc.version.starts_with('a') { "b" } else { "a" }),
                        "source_bad_range" => req.offset = u64::MAX,
                        "source_replay" if request_id > 1 => req.request_id = 1,
                        _ => {}
                    }
                    out.send(worker_event::Event::ReadAt(req)).await
                        .map_err(|super::Exit(_, s)| ("INTERNAL", s))?;
                    let remaining = desc.expires_unix_ms.saturating_sub(now_ms());
                    if remaining <= 0 {
                        return Err(("RESOURCE_LIMIT", "source grant expired".into()));
                    }
                    let reply = tokio::select! {
                        biased;
                        _ = ctl.wait_cancel() => return Err(("USER_CANCELLED", "cancelled by the host".into())),
                        reply = tokio::time::timeout(Duration::from_millis(remaining as u64), inputs.replies.recv()) => {
                            reply.map_err(|_| ("RESOURCE_LIMIT", "source grant expired".into()))?
                                .ok_or_else(|| invalid("source response channel closed".into()))?
                        }
                    };
                    validate_reply(&reply, desc, request_id, call.offset, call.length).map_err(invalid)?;
                    source_bytes += reply.data.len() as u64;
                    source_hash.update(&reply.data);
                    let _ = call.reply.send(Ok(Bytes::from(reply.data)));
                },
                Some(batch) = batch_rx.recv() => {
                    for frame in frames(&batch, cap) {
                        if ctl.cancelled() { return Err(("USER_CANCELLED","cancelled by the host".into())); }
                        let (n,bytes)=frame.map_err(|s| ("RESOURCE_LIMIT",s))?;
                        batch_hash.update(&bytes);
                        out.send(worker_event::Event::Batch(Batch {
                            index: batches, rows: n as u64, arrow_ipc: bytes,
                        })).await.map_err(|super::Exit(_, s)| ("INTERNAL", s))?;
                        batches += 1;
                        rows += n as u64;
                    }
                },
                result = tasks.join_next() => {
                    query_metrics = Some(result.ok_or_else(|| ("INTERNAL", "query task missing".into()))?
                        .map_err(|e| ("INTERNAL", e.to_string()))?.map_err(query_error)?);
                    return Ok(());
                }
            }
        }
    }.await;
    // Abort and join before returning on every error/cancel path.
    tasks.shutdown().await;
    outcome?;
    out.send(worker_event::Event::Completed(Completed {
        batches,
        rows: if ex.fault == "source_bad_count" {
            rows + 1
        } else {
            rows
        },
        batches_sha256: batch_hash.finalize().to_vec(),
        source_bytes,
        source_sha256: source_hash.finalize().to_vec(),
        source_version: desc.version.clone(),
        result_json: serde_json::to_string(&query_metrics)
            .map_err(|e| ("INTERNAL", e.to_string()))?,
        send_waits: out.waits,
        peak_rss_bytes: peak_rss(),
    }))
    .await
    .map_err(|super::Exit(_, s)| ("INTERNAL", s))
}

fn query_error(error: DataFusionError) -> (&'static str, String) {
    match error.find_root() {
        DataFusionError::ResourcesExhausted(_) => (
            "RESOURCE_LIMIT",
            "source query resource limit exceeded".into(),
        ),
        DataFusionError::IoError(e) if e.kind() == std::io::ErrorKind::PermissionDenied => {
            ("RESOURCE_LIMIT", "source query file access denied".into())
        }
        _ => ("INPUT_INVALID", error.to_string()),
    }
}

fn validate_reply(
    reply: &SourceReadResponse,
    desc: &super::pb::SourceDescriptor,
    request: u64,
    offset: u64,
    length: u32,
) -> Result<(), String> {
    let hash = Sha256::digest(&reply.data);
    if reply.request_id != request
        || reply.source_id != desc.source_id
        || reply.version != desc.version
        || reply.offset != offset
        || reply.data.len() != length as usize
        || reply.data_sha256.as_slice() != &hash[..]
    {
        return Err("source response identity, bounds or digest mismatch".into());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn query_resource_errors_keep_their_type_and_redact_details() {
        let exhausted = DataFusionError::ResourcesExhausted("private synthetic detail".into());
        let wrapped = DataFusionError::Context("operation".into(), Box::new(exhausted));
        let (code, detail) = query_error(wrapped);
        assert_eq!(code, "RESOURCE_LIMIT");
        assert!(!detail.contains("private"));
        let permission =
            DataFusionError::IoError(std::io::Error::from(std::io::ErrorKind::PermissionDenied));
        assert_eq!(query_error(permission).0, "RESOURCE_LIMIT");
        assert_eq!(
            query_error(DataFusionError::Plan("invalid column".into())).0,
            "INPUT_INVALID"
        );
    }

    #[test]
    fn rejects_independently_corrupted_response_fields() {
        let desc = super::super::pb::SourceDescriptor {
            source_id: "a".repeat(32),
            version: "b".repeat(64),
            ..Default::default()
        };
        let reply = SourceReadResponse {
            request_id: 4,
            source_id: desc.source_id.clone(),
            version: desc.version.clone(),
            offset: 100,
            data: b"abc".to_vec(),
            data_sha256: Sha256::digest(b"abc").to_vec(),
        };
        assert!(validate_reply(&reply, &desc, 4, 100, 3).is_ok());
        for fault in 0..7 {
            let mut bad = reply.clone();
            match fault {
                0 => bad.request_id += 1,
                1 => bad.source_id.replace_range(0..1, "c"),
                2 => bad.version.replace_range(0..1, "c"),
                3 => bad.offset += 1,
                4 => {
                    bad.data.pop();
                }
                5 => bad.data[0] ^= 1,
                _ => bad.data_sha256[0] ^= 1,
            }
            assert!(
                validate_reply(&bad, &desc, 4, 100, 3).is_err(),
                "accepted fault {fault}"
            );
        }
    }
}
