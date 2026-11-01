// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//! df0-worker: the ADR-012 DF0-C DataFusion worker.
//!
//! The Go host launches one worker per job and writes a length-prefixed
//! `Bootstrap` on stdin. The worker connects back to the host's private
//! Unix socket as a gRPC client, proves it is the launched child with the
//! one-time credential, and opens one bidirectional stream. All business
//! data travels on that stream. There is no TCP connector and no other
//! business channel.
//!
//! Exit codes: 0 finished, 2 bad bootstrap, 3 transport failure,
//! 4 protocol violation by the host, 9 injected exit.

mod pb {
    include!("pb/gopmgr.df0ipc.v1.rs");
}
mod read_source;

use std::io::Read;
use std::path::PathBuf;
use std::process::ExitCode;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use datafusion::arrow::array::{
    Array, ArrayRef, AsArray, BinaryViewArray, FixedSizeListArray, GenericListArray, MapArray,
    OffsetSizeTrait, RecordBatch, RecordBatchOptions, StringBuilder, StringViewArray, StructArray,
    make_array,
};
use datafusion::arrow::buffer::{NullBuffer, OffsetBuffer};
use datafusion::arrow::compute::concat;
use datafusion::arrow::datatypes::{DataType, Field, Int64Type, Schema};
use datafusion::arrow::ipc::writer::StreamWriter;
use hyper_util::rt::TokioIo;
use prost::Message;
use sha2::{Digest, Sha256};
use tokio::net::UnixStream;
use tokio::sync::{Notify, mpsc};
use tokio_stream::StreamExt;
use tokio_stream::wrappers::ReceiverStream;
use tonic::metadata::MetadataValue;
use tonic::transport::{Endpoint, Uri};

use pb::analytics_session_client::AnalyticsSessionClient;
use pb::{
    Batch, Bootstrap, Completed, Execute, Failed, Hello, HostCommand, WorkerEvent, host_command,
    worker_event,
};

const PROTOCOL_MAJOR: u32 = 1;
const PROTOCOL_MINOR: u32 = 2;
const BOOTSTRAP_VERSION: u32 = 1;
const MAX_BOOTSTRAP_BYTES: usize = 64 << 10;
const MIN_FRAME: u32 = 64 << 10;
const MAX_FRAME: u32 = 16 << 20;
/// Headroom for the Batch and WorkerEvent fields around the Arrow bytes.
const FRAME_OVERHEAD: usize = 256;
/// Largest rollup input the worker buffers.
const MAX_ROLLUP_SOURCE: usize = 64 << 20;
/// Outbound queue depth. A full queue means HTTP/2 flow control is holding
/// the worker back; the worker waits rather than buffering more.
const OUT_QUEUE: usize = 4;

fn main() -> ExitCode {
    die_with_parent();
    let boot = match read_bootstrap(std::io::stdin().lock()) {
        Ok(b) => b,
        Err(e) => {
            eprintln!("df0-worker: bootstrap: {e}");
            return ExitCode::from(2);
        }
    };
    let rt = match tokio::runtime::Builder::new_multi_thread()
        .worker_threads(2)
        .enable_io()
        .enable_time()
        .build()
    {
        Ok(rt) => rt,
        Err(e) => {
            eprintln!("df0-worker: runtime: {e}");
            return ExitCode::from(3);
        }
    };
    match rt.block_on(run(boot)) {
        Ok(()) => ExitCode::SUCCESS,
        Err(Exit(code, msg)) => {
            eprintln!("df0-worker: {msg}");
            ExitCode::from(code)
        }
    }
}

/// Ask the kernel to kill this process if the host dies, so a crashed host
/// leaves no orphan worker.
fn die_with_parent() {
    #[cfg(target_os = "linux")]
    {
        // SAFETY: prctl with PR_SET_PDEATHSIG takes a signal number and has
        // no memory effects.
        unsafe {
            libc::prctl(libc::PR_SET_PDEATHSIG, libc::SIGKILL);
        }
        // The host may have died before the call took effect.
        // SAFETY: getppid has no preconditions.
        if unsafe { libc::getppid() } == 1 {
            std::process::exit(3);
        }
    }
}

struct Exit(u8, String);

/// An error and every cause under it: transport errors hide the cause.
fn chain(e: &dyn std::error::Error) -> String {
    let mut s = e.to_string();
    let mut cur = e.source();
    while let Some(c) = cur {
        s.push_str(": ");
        s.push_str(&c.to_string());
        cur = c.source();
    }
    s
}

/// Mutual authentication, worker side: the socket must be served by the
/// process that launched this worker, under the same user, before the
/// worker sends anything (DF-024).
fn verify_host(s: &UnixStream) -> std::io::Result<()> {
    let cred = s.peer_cred()?;
    // SAFETY: getppid and getuid have no preconditions.
    let (ppid, uid) = unsafe { (libc::getppid(), libc::getuid()) };
    if cred.uid() != uid || cred.pid() != Some(ppid) {
        return Err(std::io::Error::new(
            std::io::ErrorKind::PermissionDenied,
            format!(
                "endpoint is served by pid {:?} uid {}, not the launching host {ppid}",
                cred.pid(),
                cred.uid()
            ),
        ));
    }
    Ok(())
}

fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as i64)
        .unwrap_or(0)
}

/// Reads and checks the bootstrap: one uvarint length, exactly that many
/// bytes, then end of input.
fn read_bootstrap(r: impl Read) -> Result<Bootstrap, String> {
    let mut buf = Vec::new();
    r.take(MAX_BOOTSTRAP_BYTES as u64 + 1)
        .read_to_end(&mut buf)
        .map_err(|e| format!("read: {e}"))?;
    if buf.len() > MAX_BOOTSTRAP_BYTES {
        return Err("too large".into());
    }
    let mut rest = buf.as_slice();
    let len = prost::encoding::decode_varint(&mut rest).map_err(|e| format!("length: {e}"))?;
    if len as usize != rest.len() {
        return Err(format!("length {len} but {} bytes follow", rest.len()));
    }
    let b = Bootstrap::decode(rest).map_err(|e| format!("decode: {e}"))?;
    if b.version != BOOTSTRAP_VERSION {
        return Err(format!("version {} is not {BOOTSTRAP_VERSION}", b.version));
    }
    if b.credential.len() != 32 {
        return Err("credential is not 32 bytes".into());
    }
    if b.endpoint.is_empty() || !b.endpoint.starts_with('/') {
        return Err("endpoint is not an absolute socket path".into());
    }
    if b.expires_unix_ms <= now_ms() {
        return Err("expired".into());
    }
    if !(MIN_FRAME..=MAX_FRAME).contains(&b.max_frame_bytes) {
        return Err(format!("frame cap {} out of range", b.max_frame_bytes));
    }
    Ok(b)
}

/// The outbound half of the stream: numbers events and counts flow-control
/// waits.
struct Out {
    tx: mpsc::Sender<WorkerEvent>,
    seq: u64,
    waits: u64,
}

impl Out {
    async fn send(&mut self, event: worker_event::Event) -> Result<(), Exit> {
        self.seq += 1;
        self.send_at(self.seq, event).await
    }

    async fn send_at(&mut self, sequence: u64, event: worker_event::Event) -> Result<(), Exit> {
        let ev = WorkerEvent {
            sequence,
            event: Some(event),
        };
        match self.tx.try_send(ev) {
            Ok(()) => Ok(()),
            Err(mpsc::error::TrySendError::Full(ev)) => {
                self.waits += 1;
                self.tx
                    .send(ev)
                    .await
                    .map_err(|_| Exit(3, "stream closed".into()))
            }
            Err(mpsc::error::TrySendError::Closed(_)) => Err(Exit(3, "stream closed".into())),
        }
    }

    async fn fail(&mut self, code: &str, message: String) -> Result<(), Exit> {
        self.send(worker_event::Event::Failed(Failed {
            code: code.into(),
            message,
        }))
        .await
    }
}

/// Shared between the inbound loop and the running operation.
struct Control {
    cancel: AtomicBool,
    notify: Notify,
    /// Set by the hang fault: ignore Cancel and Finish.
    deaf: AtomicBool,
}

impl Control {
    fn cancelled(&self) -> bool {
        self.cancel.load(Ordering::SeqCst)
    }

    async fn wait_cancel(&self) {
        loop {
            let notified = self.notify.notified();
            tokio::pin!(notified);
            notified.as_mut().enable();
            if self.cancelled() {
                return;
            }
            notified.await;
        }
    }
}

async fn run(boot: Bootstrap) -> Result<(), Exit> {
    let path = PathBuf::from(&boot.endpoint);
    // The URI is required by the API and never resolved: the connector
    // below only opens the Unix socket.
    let channel = Endpoint::from_static("http://df0-worker.invalid")
        .connect_with_connector(tower::service_fn(move |_: Uri| {
            let p = path.clone();
            async move {
                let s = UnixStream::connect(p).await?;
                verify_host(&s)?;
                Ok::<_, std::io::Error>(TokioIo::new(s))
            }
        }))
        .await
        .map_err(|e| Exit(3, format!("connect: {}", chain(&e))))?;
    let cap = boot.max_frame_bytes as usize;
    let mut client = AnalyticsSessionClient::new(channel)
        .max_decoding_message_size(cap)
        .max_encoding_message_size(cap);

    let (tx, rx) = mpsc::channel(OUT_QUEUE);
    let mut out = Out {
        tx,
        seq: 0,
        waits: 0,
    };
    let mut env_names: Vec<String> = std::env::vars_os()
        .map(|(k, _)| k.to_string_lossy().into_owned())
        .collect();
    env_names.sort();
    out.send(worker_event::Event::Hello(Hello {
        protocol_major: PROTOCOL_MAJOR,
        protocol_minor: PROTOCOL_MINOR,
        pid: std::process::id() as i64,
        engine: "datafusion 55.2.0".into(),
        env_names,
    }))
    .await?;

    let mut req = tonic::Request::new(ReceiverStream::new(rx));
    let proof = MetadataValue::try_from(hex::encode(&boot.credential))
        .map_err(|e| Exit(2, format!("proof: {e}")))?;
    req.metadata_mut().insert("x-df0-proof", proof);
    let operation_id = boot.operation_id.clone();
    drop(boot);
    let mut inbound = client
        .connect(req)
        .await
        .map_err(|e| Exit(3, format!("open stream: {} {}", e.code(), e.message())))?
        .into_inner();

    let ctl = Arc::new(Control {
        cancel: AtomicBool::new(false),
        notify: Notify::new(),
        deaf: AtomicBool::new(false),
    });
    let mut out = Some(out);
    let mut src_tx: Option<mpsc::Sender<Vec<u8>>> = None;
    let mut read_tx: Option<mpsc::Sender<pb::SourceReadResponse>> = None;
    let mut ops = tokio::task::JoinSet::new();
    let result = async {
        let mut want = 1u64;
        loop {
            let cmd: HostCommand = match inbound.message().await {
                Ok(Some(c)) => c,
                Ok(None) => return Err(Exit(3, "host closed the stream before Finish".into())),
                Err(e) => return Err(Exit(3, format!("receive: {} {}", e.code(), e.message()))),
            };
            if cmd.sequence != want {
                return Err(Exit(
                    4,
                    format!("host sequence {} where {want} was due", cmd.sequence),
                ));
            }
            want += 1;
            if ctl.deaf.load(Ordering::SeqCst) {
                continue;
            }
            match cmd.command {
                Some(host_command::Command::Execute(ex)) => {
                    let Some(o) = out.take() else {
                        return Err(Exit(4, "second Execute".into()));
                    };
                    let (stx, srx) = mpsc::channel(4);
                    let (rtx, rrx) = mpsc::channel(1);
                    src_tx = Some(stx);
                    read_tx = Some(rtx);
                    let inputs = read_source::Inputs {
                        chunks: srx,
                        replies: rrx,
                        operation_id: operation_id.clone(),
                    };
                    ops.spawn(execute(ex, o, inputs, ctl.clone(), cap));
                }
                Some(host_command::Command::Source(chunk)) => {
                    let Some(stx) = src_tx.as_ref() else {
                        return Err(Exit(4, "source before Execute".into()));
                    };
                    let last = chunk.last;
                    // A full channel holds the host back through flow control.
                    let _ = stx.send(chunk.data).await;
                    if last {
                        src_tx = None;
                    }
                }
                Some(host_command::Command::Cancel(_)) => {
                    ctl.cancel.store(true, Ordering::SeqCst);
                    ctl.notify.notify_waiters();
                }
                Some(host_command::Command::ReadAt(response)) => {
                    let Some(tx) = read_tx.as_ref() else {
                        return Err(Exit(4, "source response before Execute".into()));
                    };
                    tx.try_send(response)
                        .map_err(|_| Exit(4, "unsolicited or excess source response".into()))?;
                }
                Some(host_command::Command::Finish(_)) => break,
                None => return Err(Exit(4, "empty command".into())),
            }
        }
        drop(src_tx);
        if !ops.is_empty() {
            ctl.cancel.store(true, Ordering::SeqCst);
            ctl.notify.notify_waiters();
            match ops.join_next().await {
                Some(Ok(r)) => r?,
                Some(Err(e)) => return Err(Exit(3, format!("operation task: {e}"))),
                None => return Err(Exit(3, "operation task missing".into())),
            }
        }
        Ok(())
    }
    .await;
    ctl.cancel.store(true, Ordering::SeqCst);
    ctl.notify.notify_waiters();
    ops.shutdown().await;
    result
}

async fn execute(
    ex: Execute,
    mut out: Out,
    src: read_source::Inputs,
    ctl: Arc<Control>,
    cap: usize,
) -> Result<(), Exit> {
    let r = match ex.operation.as_str() {
        "generate" => generate(&ex, &mut out, None, &ctl, cap).await,
        "duplex" => generate(&ex, &mut out, Some(src.chunks), &ctl, cap).await,
        "rollup" => rollup(&mut out, src.chunks, &ctl).await,
        "parquet_range" | "parquet_stream" => {
            read_source::execute(&ex, &mut out, src, &ctl, cap).await
        }
        other => Err(("INPUT_INVALID", format!("unknown operation {other:?}"))),
    };
    match r {
        Ok(()) => Ok(()),
        Err((code, msg)) => out.fail(code, msg).await,
    }
}

type OpResult = Result<(), (&'static str, String)>;

/// Payload byte `k` of row `i`: the host checks the same rule.
fn payload(i: i64, width: usize, buf: &mut String) {
    buf.clear();
    for k in 0..width {
        buf.push((b'a' + ((i as usize + k) % 26) as u8) as char);
    }
}

fn with_payload(batch: &RecordBatch, width: usize) -> Result<RecordBatch, String> {
    let i = batch.column(0).as_primitive::<Int64Type>();
    let mut b = StringBuilder::with_capacity(batch.num_rows(), batch.num_rows() * width);
    let mut s = String::with_capacity(width);
    for r in 0..batch.num_rows() {
        payload(i.value(r), width, &mut s);
        b.append_value(&s);
    }
    let schema = Arc::new(Schema::new(vec![
        Field::new("i", DataType::Int64, false),
        Field::new("v", DataType::Int64, false),
        Field::new("s", DataType::Utf8, false),
    ]));
    let cols: Vec<ArrayRef> = vec![
        batch.column(0).clone(),
        batch.column(1).clone(),
        Arc::new(b.finish()),
    ];
    RecordBatch::try_new(schema, cols).map_err(|e| e.to_string())
}

fn selected_list_values<O: OffsetSizeTrait>(
    offsets: OffsetBuffer<O>,
    values: &ArrayRef,
    nulls: Option<&NullBuffer>,
) -> Result<(OffsetBuffer<O>, ArrayRef), String> {
    let first = offsets[0];
    if !nulls.is_some_and(|n| n.null_count() > 0) {
        let start = first.as_usize();
        let end = offsets[offsets.len() - 1].as_usize();
        return Ok((
            OffsetBuffer::new(
                offsets
                    .iter()
                    .map(|v| *v - first)
                    .collect::<Vec<_>>()
                    .into(),
            ),
            values.slice(start, end - start),
        ));
    }
    let mut selected = Vec::new();
    let mut rebased = Vec::with_capacity(offsets.len());
    let mut end = O::default();
    rebased.push(end);
    for i in 0..offsets.len() - 1 {
        if !nulls.is_some_and(|n| n.is_null(i)) {
            let len = offsets[i + 1] - offsets[i];
            if len.as_usize() != 0 {
                selected.push(values.slice(offsets[i].as_usize(), len.as_usize()));
                end += len;
            }
        }
        rebased.push(end);
    }
    let values = match selected.as_slice() {
        [] => values.slice(0, 0),
        [only] => only.clone(),
        slices => concat(&slices.iter().map(|a| a.as_ref()).collect::<Vec<_>>())
            .map_err(|e| e.to_string())?,
    };
    Ok((OffsetBuffer::new(rebased.into()), values))
}

fn mask_parent_nulls(column: &ArrayRef, parent: Option<&NullBuffer>) -> Result<ArrayRef, String> {
    if !parent.is_some_and(|n| n.null_count() > 0)
        || matches!(
            column.data_type(),
            DataType::Null | DataType::Union(_, _) | DataType::RunEndEncoded(_, _)
        )
    {
        return Ok(column.clone());
    }
    let data = column
        .to_data()
        .into_builder()
        .nulls(NullBuffer::union(column.nulls(), parent))
        .build()
        .map_err(|e| e.to_string())?;
    Ok(make_array(data))
}

fn map_list_views<O: OffsetSizeTrait>(
    list: &GenericListArray<O>,
    map: &mut impl FnMut(&ArrayRef) -> ArrayRef,
) -> Result<ArrayRef, String> {
    let (field, offsets, values, nulls) = list.clone().into_parts();
    let (offsets, selected) = selected_list_values(offsets, &values, nulls.as_ref())?;
    let values = map_view_arrays(&selected, map)?;
    GenericListArray::<O>::try_new(field, offsets, values, nulls)
        .map(|a| Arc::new(a) as ArrayRef)
        .map_err(|e| e.to_string())
}

// Preflight and compaction must traverse the same physical child spans. List
// slices retain values outside their selected offsets; trim those before copying.
fn map_view_arrays(
    column: &ArrayRef,
    map: &mut impl FnMut(&ArrayRef) -> ArrayRef,
) -> Result<ArrayRef, String> {
    let mapped: ArrayRef = match column.data_type() {
        DataType::Utf8View | DataType::BinaryView => map(column),
        DataType::Struct(_) => {
            let a = column.as_struct();
            let children = a
                .columns()
                .iter()
                .map(|c| map_view_arrays(&mask_parent_nulls(c, a.nulls())?, map))
                .collect::<Result<Vec<_>, _>>()?;
            Arc::new(
                StructArray::try_new_with_length(
                    a.fields().clone(),
                    children,
                    a.nulls().cloned(),
                    a.len(),
                )
                .map_err(|e| e.to_string())?,
            )
        }
        DataType::List(_) => return map_list_views(column.as_list::<i32>(), map),
        DataType::LargeList(_) => return map_list_views(column.as_list::<i64>(), map),
        DataType::FixedSizeList(_, _) => {
            let a = column.as_fixed_size_list();
            let (field, size, values, nulls) = a.clone().into_parts();
            let expanded = nulls.as_ref().map(|n| n.expand(size as usize));
            let values = map_view_arrays(&mask_parent_nulls(&values, expanded.as_ref())?, map)?;
            Arc::new(
                FixedSizeListArray::try_new_with_length(field, size, values, nulls, a.len())
                    .map_err(|e| e.to_string())?,
            )
        }
        DataType::Map(_, _) => {
            let a = column.as_map();
            let (field, offsets, entries, nulls, ordered) = a.clone().into_parts();
            let entries: ArrayRef = Arc::new(entries);
            let (offsets, selected) = selected_list_values(offsets, &entries, nulls.as_ref())?;
            let entries = map_view_arrays(&selected, map)?;
            Arc::new(
                MapArray::try_new(field, offsets, entries.as_struct().clone(), nulls, ordered)
                    .map_err(|e| e.to_string())?,
            )
        }
        _ => column.clone(),
    };
    Ok(mapped)
}

fn ipc_bytes(batch: &RecordBatch) -> Result<Vec<u8>, String> {
    let columns = batch
        .columns()
        .iter()
        .map(|c| {
            map_view_arrays(c, &mut |view| match view.data_type() {
                // Rebuild only logical values: gc also copies descriptors hidden
                // by nulls, which would bypass the non-null payload preflight.
                DataType::Utf8View => {
                    Arc::new(StringViewArray::from_iter(view.as_string_view().iter())) as ArrayRef
                }
                DataType::BinaryView => {
                    Arc::new(BinaryViewArray::from_iter(view.as_binary_view().iter())) as ArrayRef
                }
                _ => unreachable!("mapper receives only view leaves"),
            })
        })
        .collect::<Result<Vec<_>, _>>()?;
    let batch = RecordBatch::try_new_with_options(
        batch.schema(),
        columns,
        &RecordBatchOptions::new().with_row_count(Some(batch.num_rows())),
    )
    .map_err(|e| e.to_string())?;
    let mut w = StreamWriter::try_new(Vec::new(), &batch.schema()).map_err(|e| e.to_string())?;
    w.write(&batch).map_err(|e| e.to_string())?;
    w.finish().map_err(|e| e.to_string())?;
    w.into_inner().map_err(|e| e.to_string())
}

/// Encodes one frame at a time, retaining only cheap batch slices between sends.
struct Frames {
    pending: Vec<RecordBatch>,
    cap: usize,
    #[cfg(test)]
    largest_compacted_view_bytes: usize,
}

fn view_payload_bytes(batch: &RecordBatch) -> Result<usize, String> {
    let mut bytes = 0usize;
    for column in batch.columns() {
        map_view_arrays(column, &mut |column| {
            match column.data_type() {
                DataType::Utf8View => {
                    let values = column.as_string_view();
                    for i in 0..column.len() {
                        if !column.is_null(i) {
                            bytes = bytes.saturating_add(values.value(i).len());
                        }
                    }
                }
                DataType::BinaryView => {
                    let values = column.as_binary_view();
                    for i in 0..column.len() {
                        if !column.is_null(i) {
                            bytes = bytes.saturating_add(values.value(i).len());
                        }
                    }
                }
                _ => unreachable!("mapper receives only view leaves"),
            }
            column.clone()
        })?;
    }
    Ok(bytes)
}

impl Iterator for Frames {
    type Item = Result<(usize, Vec<u8>), String>;
    fn next(&mut self) -> Option<Self::Item> {
        while let Some(batch) = self.pending.pop() {
            let visible = match view_payload_bytes(&batch) {
                Ok(visible) => visible,
                Err(error) => {
                    self.pending.clear();
                    return Some(Err(error));
                }
            };
            // Preflight before copying: shared views can expand far beyond their
            // physical backing allocation when materialized.
            let bytes = if visible <= self.cap.saturating_sub(FRAME_OVERHEAD) {
                #[cfg(test)]
                {
                    self.largest_compacted_view_bytes =
                        self.largest_compacted_view_bytes.max(visible);
                }
                match ipc_bytes(&batch) {
                    Ok(bytes) => Some(bytes),
                    Err(error) => {
                        self.pending.clear();
                        return Some(Err(error));
                    }
                }
            } else {
                None
            };
            if bytes
                .as_ref()
                .is_some_and(|b| b.len().saturating_add(FRAME_OVERHEAD) <= self.cap)
            {
                return Some(Ok((batch.num_rows(), bytes.expect("frame checked above"))));
            }
            if batch.num_rows() <= 1 {
                self.pending.clear();
                return Some(Err(format!(
                    "one row needs at least {} bytes, over the frame cap",
                    bytes.as_ref().map_or(visible, Vec::len)
                )));
            }
            let half = batch.num_rows() / 2;
            self.pending
                .push(batch.slice(half, batch.num_rows() - half));
            self.pending.push(batch.slice(0, half));
        }
        None
    }
}

fn frames(batch: &RecordBatch, cap: usize) -> Frames {
    Frames {
        pending: vec![batch.clone()],
        cap,
        #[cfg(test)]
        largest_compacted_view_bytes: 0,
    }
}

fn peak_rss() -> u64 {
    std::fs::read_to_string("/proc/self/status")
        .ok()
        .and_then(|s| {
            s.lines()
                .find(|l| l.starts_with("VmHWM:"))
                .and_then(|l| l.split_whitespace().nth(1)?.parse::<u64>().ok())
        })
        .map(|kb| kb * 1024)
        .unwrap_or(0)
}

async fn hash_source(mut src: mpsc::Receiver<Vec<u8>>) -> (u64, Vec<u8>) {
    let mut h = Sha256::new();
    let mut n = 0u64;
    while let Some(chunk) = src.recv().await {
        n += chunk.len() as u64;
        h.update(&chunk);
    }
    (n, h.finalize().to_vec())
}

async fn generate(
    ex: &Execute,
    out: &mut Out,
    src: Option<mpsc::Receiver<Vec<u8>>>,
    ctl: &Control,
    cap: usize,
) -> OpResult {
    let internal = |e: String| ("INTERNAL", e);
    let hasher = src.map(|s| tokio::spawn(hash_source(s)));
    let mut stream = df0_engine::generate(ex.rows, ex.batch_rows as usize)
        .await
        .map_err(internal)?;
    let width = ex.payload_bytes as usize;
    let mut digest = Sha256::new();
    let (mut batches, mut rows) = (0u64, 0u64);
    loop {
        let next = tokio::select! {
            b = stream.next() => b,
            _ = ctl.notify.notified() => None,
        };
        if ctl.cancelled() {
            return Err(("USER_CANCELLED", "cancelled by the host".into()));
        }
        let Some(batch) = next else { break };
        let batch =
            with_payload(&batch.map_err(|e| internal(e.to_string()))?, width).map_err(internal)?;
        for frame in frames(&batch, cap) {
            if ctl.cancelled() {
                return Err(("USER_CANCELLED", "cancelled by the host".into()));
            }
            let (n, mut bytes) = frame.map_err(|e| ("RESOURCE_LIMIT", e))?;
            match ex.fault.as_str() {
                "bad_schema" if batches == 1 => {
                    let alt = RecordBatch::try_new(
                        Arc::new(Schema::new(vec![Field::new("i", DataType::Utf8, false)])),
                        vec![Arc::new(datafusion::arrow::array::StringArray::from(vec![
                            "x",
                        ]))],
                    )
                    .map_err(|e| internal(e.to_string()))?;
                    bytes = ipc_bytes(&alt).map_err(internal)?;
                }
                "exit_mid_stream" if batches == 3 => std::process::exit(9),
                "hang" if batches == 2 => {
                    ctl.deaf.store(true, Ordering::SeqCst);
                    std::future::pending::<()>().await;
                }
                _ => {}
            }
            digest.update(&bytes);
            let ev = worker_event::Event::Batch(Batch {
                index: batches,
                rows: n as u64,
                arrow_ipc: bytes,
            });
            let r = match ex.fault.as_str() {
                "gap" if batches == 2 => {
                    out.seq += 2;
                    out.send_at(out.seq, ev).await
                }
                "duplicate" if batches == 2 => out.send_at(out.seq, ev).await,
                _ => out.send(ev).await,
            };
            r.map_err(|Exit(_, m)| ("INTERNAL", m))?;
            batches += 1;
            rows += n as u64;
        }
    }
    // The source upload can outlast the generated rows (duplex). Waiting
    // for its digest must still honor Cancel, and must not need the rest
    // of the upload to arrive.
    let (source_bytes, source_sha256) = match hasher {
        Some(mut h) => {
            let cancelled = ctl.notify.notified();
            tokio::pin!(cancelled);
            cancelled.as_mut().enable(); // registered before the check below
            if ctl.cancelled() {
                h.abort();
                return Err(("USER_CANCELLED", "cancelled by the host".into()));
            }
            tokio::select! {
                r = &mut h => r.map_err(|e| internal(e.to_string()))?,
                _ = &mut cancelled => {
                    h.abort();
                    return Err(("USER_CANCELLED", "cancelled by the host".into()));
                }
            }
        }
        None => (0, Vec::new()),
    };
    let mut sum = digest.finalize().to_vec();
    if ex.fault == "bad_hash" {
        sum[0] ^= 0xff;
    }
    let waits = out.waits;
    out.send(worker_event::Event::Completed(Completed {
        batches,
        rows,
        batches_sha256: sum,
        source_bytes,
        source_sha256,
        result_json: String::new(),
        send_waits: waits,
        peak_rss_bytes: peak_rss(),
        source_version: String::new(),
    }))
    .await
    .map_err(|Exit(_, m)| ("INTERNAL", m))
}

async fn rollup(out: &mut Out, mut src: mpsc::Receiver<Vec<u8>>, ctl: &Control) -> OpResult {
    let mut buf = Vec::new();
    loop {
        // Cancel must not wait for the rest of the upload.
        let cancelled = ctl.notify.notified();
        tokio::pin!(cancelled);
        cancelled.as_mut().enable();
        if ctl.cancelled() {
            return Err(("USER_CANCELLED", "cancelled by the host".into()));
        }
        let chunk = tokio::select! {
            c = src.recv() => c,
            _ = &mut cancelled => return Err(("USER_CANCELLED", "cancelled by the host".into())),
        };
        let Some(chunk) = chunk else { break };
        if buf.len() + chunk.len() > MAX_ROLLUP_SOURCE {
            return Err(("RESOURCE_LIMIT", "rollup input over 64 MiB".into()));
        }
        buf.extend_from_slice(&chunk);
    }
    let projects: Vec<df0_engine::Project> =
        serde_json::from_slice(&buf).map_err(|e| ("INPUT_INVALID", format!("projects: {e}")))?;
    let result = df0_engine::rollup(&projects).await;
    let json = serde_json::to_string(&result).map_err(|e| ("INTERNAL", e.to_string()))?;
    let source_sha256 = Sha256::digest(&buf).to_vec();
    let waits = out.waits;
    out.send(worker_event::Event::Completed(Completed {
        batches: 0,
        rows: projects.len() as u64,
        batches_sha256: Vec::new(),
        source_bytes: buf.len() as u64,
        source_sha256,
        result_json: json,
        send_waits: waits,
        peak_rss_bytes: peak_rss(),
        source_version: String::new(),
    }))
    .await
    .map_err(|Exit(_, m)| ("INTERNAL", m))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn framed(b: &Bootstrap) -> Vec<u8> {
        let body = b.encode_to_vec();
        let mut v = Vec::new();
        prost::encoding::encode_varint(body.len() as u64, &mut v);
        v.extend(body);
        v
    }

    fn valid() -> Bootstrap {
        Bootstrap {
            version: 1,
            operation_id: "op".into(),
            endpoint: "/tmp/x/w.sock".into(),
            credential: vec![7; 32],
            expires_unix_ms: now_ms() + 60_000,
            max_frame_bytes: 4 << 20,
        }
    }

    type Edit = Box<dyn Fn(&mut Bootstrap)>;

    #[test]
    fn bootstrap_checks() {
        let cases: Vec<(&str, Edit, bool)> = vec![
            ("valid", Box::new(|_| {}), true),
            ("wrong version", Box::new(|b| b.version = 2), false),
            (
                "short credential",
                Box::new(|b| b.credential.truncate(31)),
                false,
            ),
            (
                "relative endpoint",
                Box::new(|b| b.endpoint = "w.sock".into()),
                false,
            ),
            (
                "expired",
                Box::new(|b| b.expires_unix_ms = now_ms() - 1),
                false,
            ),
            (
                "frame too small",
                Box::new(|b| b.max_frame_bytes = 1024),
                false,
            ),
            (
                "frame too large",
                Box::new(|b| b.max_frame_bytes = 64 << 20),
                false,
            ),
        ];
        for (name, edit, ok) in cases {
            let mut b = valid();
            edit(&mut b);
            assert_eq!(read_bootstrap(framed(&b).as_slice()).is_ok(), ok, "{name}");
        }
    }

    #[test]
    fn bootstrap_framing_is_exact() {
        let mut v = framed(&valid());
        v.push(0);
        assert!(read_bootstrap(v.as_slice()).is_err(), "trailing byte");
        let v = framed(&valid());
        assert!(read_bootstrap(&v[..v.len() - 1]).is_err(), "short body");
        assert!(read_bootstrap(&[][..]).is_err(), "empty");
    }

    #[test]
    fn frames_fit_the_cap() {
        let rows = 1000;
        let i: ArrayRef = Arc::new(datafusion::arrow::array::Int64Array::from_iter_values(
            0..rows,
        ));
        let base = RecordBatch::try_from_iter(vec![("i", i.clone()), ("v", i)]).unwrap();
        let batch = with_payload(&base, 1000).unwrap();
        let cap = 128 << 10;
        let fs = frames(&batch, cap).collect::<Result<Vec<_>, _>>().unwrap();
        assert!(fs.len() > 1);
        assert_eq!(fs.iter().map(|(n, _)| *n).sum::<usize>(), rows as usize);
        assert!(fs.iter().all(|(_, b)| b.len() + FRAME_OVERHEAD <= cap));
    }

    #[test]
    fn frames_compact_sliced_views_without_changing_values_or_types() {
        use datafusion::arrow::array::{BinaryViewBuilder, StringViewBuilder};
        use datafusion::arrow::ipc::reader::StreamReader;
        let visible = "selected".repeat(128);
        let other = "unselected".repeat(256 << 10);
        let mut strings = StringViewBuilder::new();
        strings.append_value(&visible);
        strings.append_value(&other);
        let mut binary = BinaryViewBuilder::new();
        binary.append_value(visible.as_bytes());
        binary.append_value(other.as_bytes());
        let s: ArrayRef = Arc::new(strings.finish());
        let b: ArrayRef = Arc::new(binary.finish());
        let batch =
            RecordBatch::try_from_iter(vec![("s", s.slice(0, 1)), ("b", b.slice(0, 1))]).unwrap();
        let cap = 64 << 10;
        let output = frames(&batch, cap).collect::<Result<Vec<_>, _>>().unwrap();
        assert_eq!(output.len(), 1);
        assert_eq!(output[0].0, 1);
        assert!(output[0].1.len() + FRAME_OVERHEAD <= cap);
        assert!(
            !output[0]
                .1
                .windows(b"unselected".len())
                .any(|w| w == b"unselected")
        );
        let mut reader = StreamReader::try_new(std::io::Cursor::new(&output[0].1), None).unwrap();
        let decoded = reader.next().unwrap().unwrap();
        assert_eq!(decoded.schema(), batch.schema());
        assert_eq!(decoded.column(0).as_string_view().value(0), visible);
        assert_eq!(
            decoded.column(1).as_binary_view().value(0),
            visible.as_bytes()
        );
        assert!(reader.next().is_none());
    }

    #[test]
    fn frames_compact_nested_sliced_views() {
        use datafusion::arrow::array::{Array, ListArray, StringViewBuilder, StructArray};
        use datafusion::arrow::buffer::OffsetBuffer;
        use datafusion::arrow::ipc::reader::StreamReader;

        let selected = "selected nested value".repeat(32);
        let unselected = "UNSELECTED_NESTED_SENTINEL".repeat(128 << 10);
        let mut strings = StringViewBuilder::new();
        strings.append_value(&unselected);
        strings.append_value(&selected);
        strings.append_value(&unselected);
        let values: ArrayRef = Arc::new(strings.finish());
        let list: ArrayRef = Arc::new(ListArray::new(
            Arc::new(Field::new("item", DataType::Utf8View, true)),
            OffsetBuffer::new(vec![0i32, 1, 2, 3].into()),
            values,
            None,
        ));
        let nested: ArrayRef = Arc::new(StructArray::from(vec![(
            Arc::new(Field::new("list", list.data_type().clone(), true)),
            list,
        )]));
        let batch = RecordBatch::try_from_iter(vec![("nested", nested.slice(1, 1))]).unwrap();
        let output = frames(&batch, 64 << 10)
            .collect::<Result<Vec<_>, _>>()
            .unwrap();
        assert_eq!(output.len(), 1);
        assert_eq!(output[0].0, 1);
        assert!(output[0].1.len() + FRAME_OVERHEAD <= 64 << 10);
        assert!(
            !output[0]
                .1
                .windows(26)
                .any(|w| w == b"UNSELECTED_NESTED_SENTINEL")
        );
        let mut reader = StreamReader::try_new(std::io::Cursor::new(&output[0].1), None).unwrap();
        let decoded = reader.next().unwrap().unwrap();
        assert_eq!(decoded.schema(), batch.schema());
        let list = decoded.column(0).as_struct().column(0).as_list::<i32>();
        assert_eq!(list.value(0).as_string_view().value(0), selected);
        assert!(reader.next().is_none());
    }

    #[test]
    fn frames_preserve_nested_container_slices_and_nulls() {
        use datafusion::arrow::array::{BinaryViewBuilder, StringArray, StringViewBuilder};
        use datafusion::arrow::buffer::NullBuffer;
        use datafusion::arrow::ipc::reader::StreamReader;

        let needle = b"UNSELECTED_NESTED_SENTINEL";
        let hidden = "UNSELECTED_NESTED_SENTINEL".repeat(128 << 10);
        let selected = "selected \u{96ea}".repeat(32);
        let mut strings = StringViewBuilder::new();
        strings.append_value(&hidden);
        strings.append_value(&selected);
        strings.append_null();
        strings.append_value(&hidden);
        let strings: ArrayRef = Arc::new(strings.finish());
        let mut binary = BinaryViewBuilder::new();
        let bytes = [selected.as_bytes(), &[0, 255]].concat();
        binary.append_value(hidden.as_bytes());
        binary.append_value(&bytes);
        binary.append_null();
        binary.append_value(hidden.as_bytes());
        let binary: ArrayRef = Arc::new(binary.finish());
        let field = Arc::new(Field::new("item", DataType::Utf8View, true).with_metadata(
            std::collections::HashMap::from([("origin".into(), "fixture".into())]),
        ));
        let nulls = Some(NullBuffer::from(vec![true, true, false, true, true]));
        let list: ArrayRef = Arc::new(GenericListArray::<i32>::new(
            field.clone(),
            OffsetBuffer::new(vec![0, 1, 2, 3, 3, 4].into()),
            strings.clone(),
            nulls.clone(),
        ));
        let large: ArrayRef = Arc::new(GenericListArray::<i64>::new(
            field.clone(),
            OffsetBuffer::new(vec![0, 1, 2, 3, 3, 4].into()),
            strings.clone(),
            nulls.clone(),
        ));
        let fixed: ArrayRef = Arc::new(FixedSizeListArray::new(
            field,
            1,
            strings.clone(),
            Some(NullBuffer::from(vec![true, true, false, true])),
        ));
        let structure: ArrayRef = Arc::new(StructArray::new(
            vec![
                Arc::new(Field::new("s", DataType::Utf8View, true)),
                Arc::new(Field::new("b", DataType::BinaryView, true)),
            ]
            .into(),
            vec![strings, binary],
            Some(NullBuffer::from(vec![true, true, false, true])),
        ));
        let entries = StructArray::from(vec![
            (
                Arc::new(Field::new("key", DataType::Utf8, false)),
                Arc::new(StringArray::from(vec!["old", "keep", "null", "old"])) as ArrayRef,
            ),
            (
                Arc::new(Field::new("value", DataType::BinaryView, true)),
                structure.as_struct().column(1).clone(),
            ),
        ]);
        let map: ArrayRef = Arc::new(MapArray::new(
            Arc::new(Field::new("entries", entries.data_type().clone(), false)),
            OffsetBuffer::new(vec![0, 1, 2, 3, 3, 4].into()),
            entries,
            nulls,
            true,
        ));
        for (name, array, count) in [
            ("struct", structure, 2),
            ("list", list, 3),
            ("large-list", large, 3),
            ("fixed-list", fixed, 2),
            ("map", map, 3),
        ] {
            let schema = Arc::new(Schema::new_with_metadata(
                vec![Field::new(name, array.data_type().clone(), true)],
                std::collections::HashMap::from([("source".into(), "literal fixture".into())]),
            ));
            let batch = RecordBatch::try_new(schema, vec![array.slice(1, count)]).unwrap();
            let output = frames(&batch, 64 << 10)
                .collect::<Result<Vec<_>, _>>()
                .unwrap();
            assert_eq!(output.len(), 1, "{name}");
            assert_eq!(output[0].0, count, "{name}");
            assert!(output[0].1.len() + FRAME_OVERHEAD <= 64 << 10, "{name}");
            assert!(
                !output[0].1.windows(needle.len()).any(|w| w == needle),
                "{name}"
            );
            let mut reader =
                StreamReader::try_new(std::io::Cursor::new(&output[0].1), None).unwrap();
            let decoded = reader.next().unwrap().unwrap();
            assert_eq!(decoded, batch, "{name}");
            assert!(reader.next().is_none(), "{name}");
        }
    }

    #[test]
    fn frames_preflight_nested_shared_views_and_refuse_oversize_rows() {
        use datafusion::arrow::array::{Int64Array, StringViewBuilder, UInt32Array};
        use datafusion::arrow::compute::take;
        use datafusion::arrow::ipc::reader::StreamReader;
        let text = "nested shared".repeat(4096);
        let mut builder = StringViewBuilder::new();
        builder.append_value(&text);
        let shared = take(&builder.finish(), &UInt32Array::from(vec![0; 128]), None).unwrap();
        let list: ArrayRef = Arc::new(GenericListArray::<i32>::new(
            Arc::new(Field::new("item", DataType::Utf8View, false)),
            OffsetBuffer::new((0i32..=128).collect::<Vec<_>>().into()),
            shared,
            None,
        ));
        let ids: ArrayRef = Arc::new(Int64Array::from_iter_values(0..128));
        let batch = RecordBatch::try_from_iter(vec![("id", ids), ("nested", list)]).unwrap();
        assert_eq!(view_payload_bytes(&batch).unwrap(), 128 * text.len());
        let cap = 64 << 10;
        let mut output = frames(&batch, cap);
        let mut expected_id = 0;
        for (rows, bytes) in output.by_ref().map(Result::unwrap) {
            assert_eq!(rows, 1);
            assert!(bytes.len() + FRAME_OVERHEAD <= cap);
            let mut reader = StreamReader::try_new(std::io::Cursor::new(bytes), None).unwrap();
            let decoded = reader.next().unwrap().unwrap();
            assert_eq!(
                decoded.column(0).as_primitive::<Int64Type>().value(0),
                expected_id
            );
            assert_eq!(
                decoded
                    .column(1)
                    .as_list::<i32>()
                    .value(0)
                    .as_string_view()
                    .value(0),
                text
            );
            expected_id += 1;
        }
        assert_eq!(expected_id, 128);
        assert!(
            output.largest_compacted_view_bytes > 0
                && output.largest_compacted_view_bytes <= cap - FRAME_OVERHEAD
        );
        let mut builder = StringViewBuilder::new();
        builder.append_value("x".repeat(cap));
        let child: ArrayRef = Arc::new(builder.finish());
        let nested: ArrayRef = Arc::new(StructArray::from(vec![(
            Arc::new(Field::new("value", DataType::Utf8View, false)),
            child,
        )]));
        let oversized = RecordBatch::try_from_iter(vec![("nested", nested)]).unwrap();
        let mut output = frames(&oversized, cap);
        assert!(
            output
                .next()
                .unwrap()
                .unwrap_err()
                .contains("one row needs at least")
        );
        assert_eq!(
            output.largest_compacted_view_bytes, 0,
            "oversize row compacted before preflight"
        );
        assert!(output.next().is_none());
    }

    #[test]
    fn frames_do_not_materialize_view_values_hidden_by_nulls() {
        use datafusion::arrow::array::{BinaryViewArray, StringViewArray};
        use datafusion::arrow::buffer::NullBuffer;
        use datafusion::arrow::ipc::reader::StreamReader;

        let hidden = "HIDDEN_NULL_SENTINEL".repeat(4096);
        let strings = StringViewArray::from(vec![hidden.as_str(); 128]);
        let strings: ArrayRef = Arc::new(
            StringViewArray::try_new(
                strings.views().clone(),
                strings.data_buffers().to_vec(),
                Some(NullBuffer::from(vec![false; 128])),
            )
            .unwrap(),
        );
        let binary = BinaryViewArray::from_iter_values(vec![hidden.as_bytes(); 128]);
        let binary: ArrayRef = Arc::new(
            BinaryViewArray::try_new(
                binary.views().clone(),
                binary.data_buffers().to_vec(),
                Some(NullBuffer::from(vec![false; 128])),
            )
            .unwrap(),
        );
        let nested: ArrayRef = Arc::new(StructArray::from(vec![
            (Arc::new(Field::new("s", DataType::Utf8View, true)), strings),
            (
                Arc::new(Field::new("b", DataType::BinaryView, true)),
                binary,
            ),
        ]));
        let batch = RecordBatch::try_from_iter(vec![("nested", nested)]).unwrap();
        assert_eq!(view_payload_bytes(&batch).unwrap(), 0);
        let output = frames(&batch, 64 << 10)
            .collect::<Result<Vec<_>, _>>()
            .unwrap();
        assert_eq!(
            output.len(),
            1,
            "null views must not be expanded into hidden payloads and split"
        );
        assert_eq!(output[0].0, 128);
        assert!(
            !output[0]
                .1
                .windows(b"HIDDEN_NULL_SENTINEL".len())
                .any(|w| w == b"HIDDEN_NULL_SENTINEL")
        );
        let mut reader = StreamReader::try_new(std::io::Cursor::new(&output[0].1), None).unwrap();
        assert_eq!(reader.next().unwrap().unwrap(), batch);
    }

    #[test]
    fn frames_discard_payloads_hidden_by_null_containers() {
        use datafusion::arrow::array::StringArray;
        use datafusion::arrow::buffer::NullBuffer;
        use datafusion::arrow::ipc::reader::StreamReader;

        let hidden = "HIDDEN_PARENT_SENTINEL".repeat(128 << 10);
        let values: ArrayRef = Arc::new(StringViewArray::from(vec![
            "first selected value",
            hidden.as_str(),
            "last selected value",
        ]));
        let nulls = Some(NullBuffer::from(vec![true, false, true]));
        let field = Arc::new(Field::new("item", DataType::Utf8View, false));
        let list: ArrayRef = Arc::new(GenericListArray::<i32>::new(
            field.clone(),
            OffsetBuffer::new(vec![0, 1, 2, 3].into()),
            values.clone(),
            nulls.clone(),
        ));
        let large: ArrayRef = Arc::new(GenericListArray::<i64>::new(
            field.clone(),
            OffsetBuffer::new(vec![0, 1, 2, 3].into()),
            values.clone(),
            nulls.clone(),
        ));
        let fixed: ArrayRef = Arc::new(FixedSizeListArray::new(
            field.clone(),
            1,
            values.clone(),
            nulls.clone(),
        ));
        let structure: ArrayRef = Arc::new(StructArray::new(
            vec![field].into(),
            vec![values.clone()],
            nulls.clone(),
        ));
        let entries = StructArray::from(vec![
            (
                Arc::new(Field::new("key", DataType::Utf8, false)),
                Arc::new(StringArray::from(vec!["a", "b", "c"])) as ArrayRef,
            ),
            (
                Arc::new(Field::new("value", DataType::Utf8View, false)),
                values,
            ),
        ]);
        let map: ArrayRef = Arc::new(MapArray::new(
            Arc::new(Field::new("entries", entries.data_type().clone(), false)),
            OffsetBuffer::new(vec![0, 1, 2, 3].into()),
            entries,
            nulls,
            false,
        ));
        for (name, array) in [
            ("struct", structure),
            ("list", list),
            ("large-list", large),
            ("fixed-list", fixed),
            ("map", map),
        ] {
            let batch = RecordBatch::try_from_iter(vec![(name, array)]).unwrap();
            let output = frames(&batch, 64 << 10)
                .collect::<Result<Vec<_>, _>>()
                .unwrap();
            assert_eq!(output.len(), 1, "{name}");
            assert_eq!(output[0].0, 3, "{name}");
            assert!(
                !output[0]
                    .1
                    .windows(b"HIDDEN_PARENT_SENTINEL".len())
                    .any(|w| w == b"HIDDEN_PARENT_SENTINEL"),
                "{name}"
            );
            let mut reader =
                StreamReader::try_new(std::io::Cursor::new(&output[0].1), None).unwrap();
            assert_eq!(reader.next().unwrap().unwrap(), batch, "{name}");
        }
    }

    #[test]
    fn frames_preserve_degenerate_nested_lengths() {
        use datafusion::arrow::array::StringViewArray;
        use datafusion::arrow::ipc::reader::StreamReader;
        let empty: ArrayRef = Arc::new(StringViewArray::from(Vec::<&str>::new()));
        let zero_width: ArrayRef = Arc::new(
            FixedSizeListArray::try_new_with_length(
                Arc::new(Field::new("item", DataType::Utf8View, true)),
                0,
                empty,
                None,
                3,
            )
            .unwrap(),
        );
        let empty_struct: ArrayRef = Arc::new(
            StructArray::try_new_with_length(Vec::<Field>::new().into(), Vec::new(), None, 3)
                .unwrap(),
        );
        let batch = RecordBatch::try_from_iter(vec![
            ("zero_width", zero_width),
            ("empty_struct", empty_struct),
        ])
        .unwrap();
        let output = frames(&batch, 64 << 10)
            .collect::<Result<Vec<_>, _>>()
            .unwrap();
        assert_eq!(output[0].0, 3);
        let mut reader = StreamReader::try_new(std::io::Cursor::new(&output[0].1), None).unwrap();
        assert_eq!(reader.next().unwrap().unwrap(), batch);
    }

    #[test]
    fn frames_preflight_shared_view_expansion_and_yield_incrementally() {
        use datafusion::arrow::array::{StringViewBuilder, UInt32Array};
        use datafusion::arrow::compute::take;
        let value = "v".repeat(64 << 10);
        let mut builder = StringViewBuilder::new();
        builder.append_value(&value);
        let shared = take(&builder.finish(), &UInt32Array::from(vec![0; 128]), None).unwrap();
        assert!(shared.get_buffer_memory_size() < 128 << 10);
        let batch = RecordBatch::try_from_iter(vec![("view", shared)]).unwrap();
        assert_eq!(view_payload_bytes(&batch).unwrap(), 128 * value.len());
        let mut output = frames(&batch, 128 << 10);
        let (rows, first) = output.next().unwrap().unwrap();
        assert_eq!(rows, 1);
        assert!(first.len() + FRAME_OVERHEAD <= 128 << 10);
        assert!(output.pending.len() <= 8);
        assert!(
            output.largest_compacted_view_bytes > 0
                && output.largest_compacted_view_bytes <= (128 << 10)
        );
        let (rows, second) = output.next().unwrap().unwrap();
        assert_eq!(rows, 1);
        assert!(second.len() + FRAME_OVERHEAD <= 128 << 10);
    }

    #[test]
    fn payload_rule() {
        let mut s = String::new();
        payload(27, 3, &mut s);
        assert_eq!(s, "bcd");
    }
}
