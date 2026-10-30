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

use std::io::Read;
use std::path::PathBuf;
use std::process::ExitCode;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use datafusion::arrow::array::{ArrayRef, AsArray, RecordBatch, StringBuilder};
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
const PROTOCOL_MINOR: u32 = 0;
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
    let mut op: Option<tokio::task::JoinHandle<Result<(), Exit>>> = None;
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
                src_tx = Some(stx);
                op = Some(tokio::spawn(execute(ex, o, srx, ctl.clone(), cap)));
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
            Some(host_command::Command::Finish(_)) => break,
            None => return Err(Exit(4, "empty command".into())),
        }
    }
    drop(src_tx);
    if let Some(h) = op {
        ctl.cancel.store(true, Ordering::SeqCst);
        ctl.notify.notify_waiters();
        match h.await {
            Ok(r) => r?,
            Err(e) => return Err(Exit(3, format!("operation task: {e}"))),
        }
    }
    Ok(())
}

async fn execute(
    ex: Execute,
    mut out: Out,
    src: mpsc::Receiver<Vec<u8>>,
    ctl: Arc<Control>,
    cap: usize,
) -> Result<(), Exit> {
    let r = match ex.operation.as_str() {
        "generate" => generate(&ex, &mut out, None, &ctl, cap).await,
        "duplex" => generate(&ex, &mut out, Some(src), &ctl, cap).await,
        "rollup" => rollup(&mut out, src, &ctl).await,
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

fn ipc_bytes(batch: &RecordBatch) -> Result<Vec<u8>, String> {
    let mut w = StreamWriter::try_new(Vec::new(), &batch.schema()).map_err(|e| e.to_string())?;
    w.write(batch).map_err(|e| e.to_string())?;
    w.finish().map_err(|e| e.to_string())?;
    w.into_inner().map_err(|e| e.to_string())
}

/// Splits a batch until each Arrow IPC frame fits under the gRPC cap.
fn frames(batch: &RecordBatch, cap: usize) -> Result<Vec<(usize, Vec<u8>)>, String> {
    let bytes = ipc_bytes(batch)?;
    if bytes.len() + FRAME_OVERHEAD <= cap {
        return Ok(vec![(batch.num_rows(), bytes)]);
    }
    if batch.num_rows() <= 1 {
        return Err(format!(
            "one row needs {} bytes, over the frame cap",
            bytes.len()
        ));
    }
    let half = batch.num_rows() / 2;
    let mut v = frames(&batch.slice(0, half), cap)?;
    v.extend(frames(&batch.slice(half, batch.num_rows() - half), cap)?);
    Ok(v)
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
        for (n, mut bytes) in frames(&batch, cap).map_err(|e| ("RESOURCE_LIMIT", e))? {
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
        let fs = frames(&batch, cap).unwrap();
        assert!(fs.len() > 1);
        assert_eq!(fs.iter().map(|(n, _)| *n).sum::<usize>(), rows as usize);
        assert!(fs.iter().all(|(_, b)| b.len() + FRAME_OVERHEAD <= cap));
    }

    #[test]
    fn payload_rule() {
        let mut s = String::new();
        payload(27, 3, &mut s);
        assert_eq!(s, "bcd");
    }
}
