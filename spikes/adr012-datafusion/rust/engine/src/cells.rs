// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//! Arrow values to the corpus's engine-neutral cells: kind, exact text, and
//! the Arrow type name. Decimals keep their scale, integers their digits,
//! timestamps their full precision; nested values become canonical JSON.

use datafusion::arrow::array::{Array, ArrayRef, AsArray};
use datafusion::arrow::compute::cast;
use datafusion::arrow::datatypes::*;
use datafusion::arrow::temporal_conversions::{date32_to_datetime, date64_to_datetime};
use datafusion::arrow::util::display::{ArrayFormatter, FormatOptions};
use serde::Serialize;

#[derive(Debug, Serialize, Clone)]
pub struct Cell {
    pub k: &'static str,
    pub v: String,
    pub t: String,
}

fn mk(k: &'static str, v: String, t: &DataType) -> Cell {
    Cell {
        k,
        v,
        t: t.to_string(),
    }
}

pub fn float_text(f: f64) -> String {
    if f.is_nan() {
        "NaN".into()
    } else if f.is_infinite() {
        if f > 0.0 {
            "+Inf".into()
        } else {
            "-Inf".into()
        }
    } else if f == 0.0 && f.is_sign_negative() {
        "-0".into()
    } else {
        // Rust's Display for f64 is the shortest text that round-trips.
        format!("{f}")
    }
}

/// Seconds and nanoseconds since the epoch for a timestamp value.
fn split(v: i64, unit: &TimeUnit) -> (i64, u32) {
    let per: i64 = match unit {
        TimeUnit::Second => 1,
        TimeUnit::Millisecond => 1_000,
        TimeUnit::Microsecond => 1_000_000,
        TimeUnit::Nanosecond => 1_000_000_000,
    };
    let secs = v.div_euclid(per);
    let frac = v.rem_euclid(per);
    (secs, (frac * (1_000_000_000 / per)) as u32)
}

fn ts_text(v: i64, unit: &TimeUnit, tz: bool) -> String {
    let (secs, nanos) = split(v, unit);
    match datafusion::arrow::temporal_conversions::timestamp_s_to_datetime(secs) {
        Some(dt) => {
            let mut s = dt.format("%Y-%m-%dT%H:%M:%S").to_string();
            if nanos != 0 {
                let frac = format!("{nanos:09}");
                s.push('.');
                s.push_str(frac.trim_end_matches('0'));
            }
            if tz {
                s.push('Z');
            }
            s
        }
        None => format!("<out of range timestamp {v}>"),
    }
}

pub fn cell(a: &ArrayRef, i: usize) -> Cell {
    let t = a.data_type();
    if a.is_null(i) {
        return mk("null", String::new(), t);
    }
    match t {
        DataType::Boolean => mk("bool", a.as_boolean().value(i).to_string(), t),
        DataType::Int8 => mk("int", a.as_primitive::<Int8Type>().value(i).to_string(), t),
        DataType::Int16 => mk("int", a.as_primitive::<Int16Type>().value(i).to_string(), t),
        DataType::Int32 => mk("int", a.as_primitive::<Int32Type>().value(i).to_string(), t),
        DataType::Int64 => mk("int", a.as_primitive::<Int64Type>().value(i).to_string(), t),
        DataType::UInt8 => mk("int", a.as_primitive::<UInt8Type>().value(i).to_string(), t),
        DataType::UInt16 => mk(
            "int",
            a.as_primitive::<UInt16Type>().value(i).to_string(),
            t,
        ),
        DataType::UInt32 => mk(
            "int",
            a.as_primitive::<UInt32Type>().value(i).to_string(),
            t,
        ),
        DataType::UInt64 => mk(
            "int",
            a.as_primitive::<UInt64Type>().value(i).to_string(),
            t,
        ),
        DataType::Float16 => mk(
            "float",
            float_text(a.as_primitive::<Float16Type>().value(i).to_f64()),
            t,
        ),
        DataType::Float32 => mk(
            "float",
            float_text(a.as_primitive::<Float32Type>().value(i) as f64),
            t,
        ),
        DataType::Float64 => mk(
            "float",
            float_text(a.as_primitive::<Float64Type>().value(i)),
            t,
        ),
        DataType::Decimal128(_, _) => mk(
            "dec",
            a.as_primitive::<Decimal128Type>().value_as_string(i),
            t,
        ),
        DataType::Decimal256(_, _) => mk(
            "dec",
            a.as_primitive::<Decimal256Type>().value_as_string(i),
            t,
        ),
        DataType::Utf8 => mk("str", a.as_string::<i32>().value(i).to_string(), t),
        DataType::LargeUtf8 => mk("str", a.as_string::<i64>().value(i).to_string(), t),
        DataType::Utf8View => mk("str", a.as_string_view().value(i).to_string(), t),
        DataType::Binary => mk("bytes", hex(a.as_binary::<i32>().value(i)), t),
        DataType::LargeBinary => mk("bytes", hex(a.as_binary::<i64>().value(i)), t),
        DataType::BinaryView => mk("bytes", hex(a.as_binary_view().value(i)), t),
        DataType::FixedSizeBinary(_) => mk("bytes", hex(a.as_fixed_size_binary().value(i)), t),
        DataType::Date32 => match date32_to_datetime(a.as_primitive::<Date32Type>().value(i)) {
            Some(d) => mk("date", d.format("%Y-%m-%d").to_string(), t),
            None => mk("str", "<out of range date>".into(), t),
        },
        DataType::Date64 => match date64_to_datetime(a.as_primitive::<Date64Type>().value(i)) {
            Some(d) => mk("time", d.format("%Y-%m-%dT%H:%M:%SZ").to_string(), t),
            None => mk("str", "<out of range date>".into(), t),
        },
        DataType::Timestamp(unit, tz) => {
            let v = match unit {
                TimeUnit::Second => a.as_primitive::<TimestampSecondType>().value(i),
                TimeUnit::Millisecond => a.as_primitive::<TimestampMillisecondType>().value(i),
                TimeUnit::Microsecond => a.as_primitive::<TimestampMicrosecondType>().value(i),
                TimeUnit::Nanosecond => a.as_primitive::<TimestampNanosecondType>().value(i),
            };
            mk("time", ts_text(v, unit, tz.is_some()), t)
        }
        DataType::Dictionary(_, value) => match cast(a, value) {
            Ok(plain) => {
                let mut c = cell(&plain, i);
                c.t = t.to_string();
                c
            }
            Err(e) => mk("str", format!("<cast error {e}>"), t),
        },
        DataType::List(_)
        | DataType::LargeList(_)
        | DataType::FixedSizeList(_, _)
        | DataType::Struct(_)
        | DataType::Map(_, _) => mk("json", json(a, i), t),
        _ => {
            // Times of day, durations and intervals are not produced by the
            // corpus; render them with Arrow's formatter and say so.
            let f = ArrayFormatter::try_new(a.as_ref(), &FormatOptions::default());
            match f {
                Ok(f) => mk("str", f.value(i).to_string(), t),
                Err(e) => mk("str", format!("<format error {e}>"), t),
            }
        }
    }
}

fn hex(b: &[u8]) -> String {
    b.iter().map(|x| format!("{x:02x}")).collect()
}

fn quote(s: &str) -> String {
    serde_json::to_string(s).unwrap_or_else(|_| "\"\"".into())
}

/// Canonical JSON of a nested value: sorted object keys, numbers as their
/// exact text (decimals with scale), timestamps and bytes as strings.
fn json(a: &ArrayRef, i: usize) -> String {
    if a.is_null(i) {
        return "null".into();
    }
    match a.data_type() {
        DataType::List(_) => list(&a.as_list::<i32>().value(i)),
        DataType::LargeList(_) => list(&a.as_list::<i64>().value(i)),
        DataType::FixedSizeList(_, _) => list(&a.as_fixed_size_list().value(i)),
        DataType::Struct(fields) => {
            let s = a.as_struct();
            let mut parts: Vec<(String, String)> = fields
                .iter()
                .enumerate()
                .map(|(j, f)| (f.name().clone(), json(s.column(j), i)))
                .collect();
            parts.sort();
            format!(
                "{{{}}}",
                parts
                    .iter()
                    .map(|(k, v)| format!("{}:{}", quote(k), v))
                    .collect::<Vec<_>>()
                    .join(",")
            )
        }
        DataType::Map(_, _) => {
            let m = a.as_map();
            let entries = m.value(i);
            let keys = entries.column(0);
            let vals = entries.column(1);
            let mut parts: Vec<(String, String)> = (0..entries.len())
                .map(|j| {
                    let k = cell(keys, j).v;
                    (k, json(vals, j))
                })
                .collect();
            parts.sort();
            format!(
                "{{{}}}",
                parts
                    .iter()
                    .map(|(k, v)| format!("{}:{}", quote(k), v))
                    .collect::<Vec<_>>()
                    .join(",")
            )
        }
        _ => {
            let c = cell(a, i);
            match c.k {
                "int" | "dec" => c.v,
                "float" => match c.v.as_str() {
                    "NaN" | "+Inf" | "-Inf" => quote(&c.v),
                    _ => c.v,
                },
                "bool" => c.v,
                "null" => "null".into(),
                _ => quote(&c.v),
            }
        }
    }
}

fn list(values: &ArrayRef) -> String {
    let parts: Vec<String> = (0..values.len()).map(|j| json(values, j)).collect();
    format!("[{}]", parts.join(","))
}
