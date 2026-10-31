// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//! ADR-012 DF0-B candidate engine: Apache DataFusion running GoPMgr's two
//! analytics operations, `ImportTabular` and `PortfolioRollup`, with the
//! output in the corpus's engine-neutral cell form so the Go harness can
//! score it against the same truth as DuckDB.
//!
//! This is evidence code: it reads files directly from paths (DF0-B). The
//! production design (DF-09, DF-A07) has the host stream bytes instead.

mod cells;
pub mod host_source;
mod sniff;

use std::io::Cursor;
use std::sync::Arc;

use datafusion::arrow::array::{ArrayRef, BooleanArray, Int64Array, RecordBatch};
use datafusion::arrow::datatypes::{DataType, Field, Schema};
use datafusion::arrow::json::ReaderBuilder;
use datafusion::arrow::json::reader::infer_json_schema;
use datafusion::datasource::MemTable;
use datafusion::execution::context::SQLOptions;
use datafusion::prelude::{
    CsvReadOptions, JsonReadOptions, ParquetReadOptions, SessionConfig, SessionContext,
};
use serde::{Deserialize, Serialize};

pub use cells::Cell;

/// The shipped preview cap.
pub const PREVIEW_ROWS: usize = 10_000;

/// Largest file this spike will load into memory for the JSON-array adapter.
const MAX_JSON_ARRAY_BYTES: usize = 256 << 20;

#[derive(Debug, Serialize)]
pub struct ImportOutput {
    pub columns: Vec<String>,
    pub column_types: Vec<String>,
    pub rows: Vec<Vec<Cell>>,
    /// Rows read, up to PREVIEW_ROWS + 1; more than PREVIEW_ROWS means truncated.
    pub rows_seen: usize,
    pub truncated: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    /// How the file was read, for the parity ledger.
    pub reader: String,
}

fn session() -> SessionContext {
    // One partition: a preview must be the first rows in file order, and a
    // parallel scan returns row groups in any order (found in DF0-B). The
    // host owns concurrency across jobs (DF-A09).
    let config = SessionConfig::new()
        .with_target_partitions(1)
        .with_information_schema(false);
    SessionContext::new_with_config(config)
}

/// Only read-only queries: no DDL, DML or statements (DF-032, DF-A08).
fn read_only() -> SQLOptions {
    SQLOptions::new()
        .with_allow_ddl(false)
        .with_allow_dml(false)
        .with_allow_statements(false)
}

/// DF0-C transport load: a DataFusion plan that streams `rows`
/// deterministic rows `(i, v)` with `v = i * 7919 mod 1000003`, in batches
/// of at most `batch_rows`. The host recomputes every value independently.
/// The plan streams, so the worker never holds all rows at once.
pub async fn generate(
    rows: u64,
    batch_rows: usize,
) -> Result<datafusion::physical_plan::SendableRecordBatchStream, String> {
    let config = SessionConfig::new()
        .with_target_partitions(1)
        .with_information_schema(false)
        .with_batch_size(batch_rows.max(1));
    let ctx = SessionContext::new_with_config(config);
    // `rows` is an integer, so the statement text cannot be injected.
    let sql = format!("SELECT value AS i, (value * 7919) % 1000003 AS v FROM range(0, {rows})");
    let df = ctx
        .sql_with_options(&sql, read_only())
        .await
        .map_err(|e| format!("plan: {e}"))?;
    df.execute_stream()
        .await
        .map_err(|e| format!("execute: {e}"))
}

fn extension(path: &str) -> String {
    match path.rfind('.') {
        Some(i) if !path[i..].contains('/') => path[i..].to_string(),
        _ => String::new(),
    }
}

/// Import a file as the shipped `ImportTabular` does: at most PREVIEW_ROWS
/// rows. `format` is csv, tsv, txt, json, ndjson or parquet, chosen by the
/// caller from the extension, as the shipped engine does.
pub async fn import(path: &str, format: &str) -> ImportOutput {
    let mut out = ImportOutput {
        columns: vec![],
        column_types: vec![],
        rows: vec![],
        rows_seen: 0,
        truncated: false,
        error: None,
        reader: String::new(),
    };
    match import_inner(path, format, &mut out).await {
        Ok(()) => {}
        Err(e) => out.error = Some(e),
    }
    out
}

async fn import_inner(path: &str, format: &str, out: &mut ImportOutput) -> Result<(), String> {
    let ctx = session();
    let ext = extension(path);
    match format {
        "csv" | "tsv" | "txt" => {
            let sample = sniff::read_prefix(path, 64 << 10).map_err(|e| format!("read: {e}"))?;
            let dialect = sniff::dialect(&sample, format);
            out.reader = format!(
                "datafusion csv (delimiter {:?}, header {}) after the spike's dialect sniffer",
                dialect.delimiter as char, dialect.has_header
            );
            let opts = || {
                CsvReadOptions::new()
                    .has_header(dialect.has_header)
                    .delimiter(dialect.delimiter)
                    .newlines_in_values(true)
                    .file_extension(&ext)
            };
            ctx.register_csv("t", path, opts())
                .await
                .map_err(|e| e.to_string())?;
            let padded = sniff::zero_padded_columns(&sample, &dialect);
            if !padded.is_empty() {
                // Re-register with those columns typed as text.
                let inferred = ctx
                    .table("t")
                    .await
                    .map_err(|e| e.to_string())?
                    .schema()
                    .as_arrow()
                    .clone();
                let fields: Vec<Field> = inferred
                    .fields()
                    .iter()
                    .enumerate()
                    .map(|(j, f)| {
                        if padded.contains(&j) {
                            Field::new(f.name(), DataType::Utf8, true)
                        } else {
                            f.as_ref().clone()
                        }
                    })
                    .collect();
                let schema = Schema::new(fields);
                ctx.deregister_table("t").map_err(|e| e.to_string())?;
                ctx.register_csv("t", path, opts().schema(&schema))
                    .await
                    .map_err(|e| e.to_string())?;
                out.reader
                    .push_str(&format!("; zero-padded columns {padded:?} kept as text"));
            }
        }
        "json" | "ndjson" => {
            let bytes = std::fs::read(path).map_err(|e| format!("read: {e}"))?;
            let first = bytes.iter().find(|b| !b" \t\r\n".contains(b)).copied();
            let adapter = std::env::var("DF0_JSON_ARRAY").as_deref() == Ok("adapter");
            if first == Some(b'[') && !adapter {
                // DataFusion 55 reads a top-level JSON array natively.
                out.reader = "datafusion json (array)".into();
                let opts = JsonReadOptions::default()
                    .file_extension(&ext)
                    .newline_delimited(false);
                ctx.register_json("t", path, opts)
                    .await
                    .map_err(|e| e.to_string())?;
            } else if first == Some(b'[') {
                // Comparison path: split the array with serde_json RawValue,
                // keeping each element's text (numbers are not reformatted).
                out.reader = "json-array adapter (serde_json RawValue) then arrow-json".into();
                if bytes.len() > MAX_JSON_ARRAY_BYTES {
                    return Err("json array larger than the spike's adapter limit".into());
                }
                let items: Vec<Box<serde_json::value::RawValue>> =
                    serde_json::from_slice(&bytes).map_err(|e| format!("json: {e}"))?;
                let mut ndjson = Vec::with_capacity(bytes.len());
                for item in &items {
                    let raw = item.get();
                    if !raw.trim_start().starts_with('{') {
                        return Err("json array element is not an object".into());
                    }
                    ndjson.extend_from_slice(raw.as_bytes());
                    ndjson.push(b'\n');
                }
                let (schema, _) =
                    infer_json_schema(Cursor::new(&ndjson), None).map_err(|e| e.to_string())?;
                let schema = Arc::new(schema);
                let reader = ReaderBuilder::new(schema.clone())
                    .build(Cursor::new(&ndjson))
                    .map_err(|e| e.to_string())?;
                let mut batches = vec![];
                for b in reader {
                    batches.push(b.map_err(|e| e.to_string())?);
                }
                let table = MemTable::try_new(schema, vec![batches]).map_err(|e| e.to_string())?;
                ctx.register_table("t", Arc::new(table))
                    .map_err(|e| e.to_string())?;
            } else {
                // arrow-json's inference keeps first-appearance column order;
                // DataFusion's own sorts the columns, so pass the schema in.
                out.reader = "datafusion ndjson (arrow-json schema in file order)".into();
                let (schema, _) = infer_json_schema(Cursor::new(&bytes), Some(1000))
                    .map_err(|e| e.to_string())?;
                let opts = JsonReadOptions::default()
                    .file_extension(&ext)
                    .schema(&schema);
                ctx.register_json("t", path, opts)
                    .await
                    .map_err(|e| e.to_string())?;
            }
        }
        "parquet" => {
            out.reader = "datafusion parquet".into();
            let opts = ParquetReadOptions::default().file_extension(&ext);
            ctx.register_parquet("t", path, opts)
                .await
                .map_err(|e| e.to_string())?;
        }
        other => return Err(format!("unsupported format {other:?}")),
    }
    let df = ctx
        .sql_with_options(
            &format!("SELECT * FROM t LIMIT {}", PREVIEW_ROWS + 1),
            read_only(),
        )
        .await
        .map_err(|e| e.to_string())?;
    let schema = df.schema().as_arrow().clone();
    out.columns = schema.fields().iter().map(|f| f.name().clone()).collect();
    out.column_types = schema
        .fields()
        .iter()
        .map(|f| f.data_type().to_string())
        .collect();
    let batches = df.collect().await.map_err(|e| e.to_string())?;
    for batch in &batches {
        for row in 0..batch.num_rows() {
            out.rows_seen += 1;
            if out.rows.len() < PREVIEW_ROWS {
                out.rows.push(
                    batch
                        .columns()
                        .iter()
                        .map(|c| cells::cell(c, row))
                        .collect(),
                );
            }
        }
    }
    out.truncated = out.rows_seen > PREVIEW_ROWS;
    Ok(())
}

/// One portfolio row, already normalized by the Go host: minor units only.
/// Amounts are strings so no JSON reader rounds a value beyond 2^53.
#[derive(Debug, Deserialize)]
pub struct Project {
    pub budgeted_cost_minor_units: String,
    pub committed_cost_minor_units: String,
    pub actual_cost_minor_units: String,
    pub earned_value_minor_units: String,
    pub planned_value_minor_units: String,
    pub evm_available: bool,
}

#[derive(Debug, Serialize, Default)]
pub struct RollupOutput {
    pub project_count: i64,
    pub evm_project_count: i64,
    pub evm_unavailable_project_count: i64,
    /// Exact totals in minor units, as strings.
    pub totals: Vec<String>,
    pub spi: f64,
    pub cpi: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Portfolio rollup in DataFusion SQL. Sums run over DECIMAL(38,0) so they
/// cannot wrap; a total outside int64 is returned as a typed overflow
/// error, matching the shipped behavior of never wrapping.
pub async fn rollup(projects: &[Project]) -> RollupOutput {
    match rollup_inner(projects).await {
        Ok(o) => o,
        Err(e) => RollupOutput {
            error: Some(e),
            ..Default::default()
        },
    }
}

async fn rollup_inner(projects: &[Project]) -> Result<RollupOutput, String> {
    let parse = |s: &str| s.parse::<i64>().map_err(|e| format!("amount {s:?}: {e}"));
    let mut cols: [Vec<i64>; 5] = Default::default();
    let mut evm = Vec::with_capacity(projects.len());
    for p in projects {
        for (i, s) in [
            &p.budgeted_cost_minor_units,
            &p.committed_cost_minor_units,
            &p.actual_cost_minor_units,
            &p.earned_value_minor_units,
            &p.planned_value_minor_units,
        ]
        .into_iter()
        .enumerate()
        {
            cols[i].push(parse(s)?);
        }
        evm.push(p.evm_available);
    }
    let names = ["budget", "committed", "ac", "ev", "pv"];
    let mut fields: Vec<Field> = names
        .iter()
        .map(|n| Field::new(*n, DataType::Int64, false))
        .collect();
    fields.push(Field::new("evm", DataType::Boolean, false));
    let schema = Arc::new(Schema::new(fields));
    let mut arrays: Vec<ArrayRef> = cols
        .into_iter()
        .map(|c| Arc::new(Int64Array::from(c)) as ArrayRef)
        .collect();
    arrays.push(Arc::new(BooleanArray::from(evm)));
    let batch = RecordBatch::try_new(schema.clone(), arrays).map_err(|e| e.to_string())?;
    let ctx = session();
    ctx.register_table(
        "portfolio",
        Arc::new(MemTable::try_new(schema, vec![vec![batch]]).map_err(|e| e.to_string())?),
    )
    .map_err(|e| e.to_string())?;
    let dec = |c: &str| format!("CAST({c} AS DECIMAL(38,0))");
    let evm_dec = |c: &str| {
        format!("CASE WHEN evm THEN CAST({c} AS DECIMAL(38,0)) ELSE CAST(0 AS DECIMAL(38,0)) END")
    };
    let sql = format!(
        "SELECT count(*) AS n, sum(CASE WHEN evm THEN 1 ELSE 0 END) AS n_evm, \
         sum({}) AS budget, sum({}) AS committed, sum({}) AS ac, sum({}) AS ev, sum({}) AS pv FROM portfolio",
        dec("budget"),
        dec("committed"),
        evm_dec("ac"),
        evm_dec("ev"),
        evm_dec("pv")
    );
    let batches = ctx
        .sql_with_options(&sql, read_only())
        .await
        .map_err(|e| e.to_string())?
        .collect()
        .await
        .map_err(|e| e.to_string())?;
    let b = batches.first().ok_or("no result")?;
    let int = |i: usize| -> Result<i128, String> {
        let c = cells::cell(b.column(i), 0);
        if c.k == "null" {
            return Ok(0); // SUM over zero rows is NULL; the shipped SQL coalesces it to 0
        }
        c.v.parse::<i128>()
            .map_err(|e| format!("column {i}: {e} ({})", c.v))
    };
    let mut out = RollupOutput {
        project_count: int(0)? as i64,
        evm_project_count: int(1)? as i64,
        ..Default::default()
    };
    out.evm_unavailable_project_count = out.project_count - out.evm_project_count;
    let mut totals = [0i64; 5];
    for (j, t) in totals.iter_mut().enumerate() {
        let v = int(2 + j)?;
        *t = i64::try_from(v)
            .map_err(|_| format!("overflow: {} total {} is outside int64", names[j], v))?;
        out.totals.push(t.to_string());
    }
    // The indices exactly as the shipped code computes them: from float64
    // major units (minor / 100), 0 meaning n/a.
    let major = |m: i64| m as f64 / 100.0;
    if major(totals[4]) != 0.0 {
        out.spi = major(totals[3]) / major(totals[4]);
    }
    if major(totals[2]) != 0.0 {
        out.cpi = major(totals[3]) / major(totals[2]);
    }
    Ok(out)
}

/// What a naive DataFusion SUM over BIGINT does on overflow (DF-005 probe).
pub async fn probe_sum_overflow() -> String {
    let ctx = session();
    let r = async {
        let df = ctx
            .sql_with_options(
                "SELECT sum(x) AS s FROM (VALUES (CAST(9223372036854775807 AS BIGINT)), (CAST(1 AS BIGINT))) AS v(x)",
                read_only(),
            )
            .await?;
        df.collect().await
    }
    .await;
    match r {
        Ok(b) => format!("returned {}", cells::cell(b[0].column(0), 0).v),
        Err(e) => format!("error: {e}"),
    }
}

#[cfg(test)]
mod order_tests {
    use super::*;

    // serde_json's preserve_order feature keeps object keys in file order;
    // without it arrow-json's inference sorts them (found in DF0-B).
    #[test]
    fn json_inference_keeps_file_order() {
        let doc = b"{\"id\":1,\"name\":\"a\",\"amount\":1.5,\"ok\":true}\n";
        let (schema, _) = infer_json_schema(Cursor::new(&doc[..]), Some(10)).unwrap();
        let names: Vec<_> = schema.fields().iter().map(|f| f.name().clone()).collect();
        assert_eq!(names, ["id", "name", "amount", "ok"]);
    }

    #[tokio::test]
    async fn ndjson_import_keeps_file_order() {
        let path = std::env::temp_dir().join(format!("df0-order-{}.ndjson", std::process::id()));
        std::fs::write(
            &path,
            b"{\"id\":1,\"name\":\"a\",\"amount\":1.5,\"ok\":true}\n",
        )
        .unwrap();
        let out = import(path.to_str().unwrap(), "ndjson").await;
        let _ = std::fs::remove_file(&path);
        assert_eq!(out.error, None);
        assert_eq!(out.columns, ["id", "name", "amount", "ok"]);
    }

    #[tokio::test]
    async fn rollup_overflow_is_an_error_not_a_wrap() {
        let p = |b: &str| Project {
            budgeted_cost_minor_units: b.into(),
            committed_cost_minor_units: "0".into(),
            actual_cost_minor_units: "0".into(),
            earned_value_minor_units: "0".into(),
            planned_value_minor_units: "0".into(),
            evm_available: false,
        };
        let out = rollup(&[p("9223372036854775807"), p("1")]).await;
        assert!(
            out.error.as_deref().unwrap_or("").starts_with("overflow"),
            "{:?}",
            out.error
        );
    }

    #[tokio::test]
    async fn generate_streams_bounded_batches_of_exact_values() {
        use datafusion::arrow::array::AsArray;
        use datafusion::arrow::datatypes::Int64Type;
        use tokio_stream::StreamExt;

        let mut stream = generate(10, 4).await.expect("plan");
        let mut sizes = Vec::new();
        let mut next = 0i64;
        while let Some(batch) = stream.next().await {
            let batch = batch.expect("batch");
            sizes.push(batch.num_rows());
            let i = batch.column(0).as_primitive::<Int64Type>();
            let v = batch.column(1).as_primitive::<Int64Type>();
            for r in 0..batch.num_rows() {
                assert_eq!(i.value(r), next);
                assert_eq!(v.value(r), next * 7919 % 1_000_003);
                next += 1;
            }
        }
        assert_eq!(next, 10);
        assert!(sizes.iter().all(|&n| n <= 4), "{sizes:?}");
    }
}
