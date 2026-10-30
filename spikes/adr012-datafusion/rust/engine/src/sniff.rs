// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//! A small delimited-text dialect sniffer. DataFusion's CSV reader infers
//! column types but not the delimiter or whether there is a header, both of
//! which DuckDB's `read_csv_auto` detects. Without this, `.txt` files and
//! headerless files would not import as they do today.

use std::fs::File;
use std::io::Read;

pub struct Dialect {
    pub delimiter: u8,
    pub has_header: bool,
}

pub fn read_prefix(path: &str, limit: usize) -> std::io::Result<Vec<u8>> {
    let mut f = File::open(path)?;
    let mut buf = vec![0u8; limit];
    let n = f.read(&mut buf)?;
    buf.truncate(n);
    Ok(buf)
}

/// Split one sample into records and fields, honoring double quotes.
pub fn records(sample: &[u8], delim: u8) -> Vec<Vec<String>> {
    let mut out = vec![];
    let mut rec = vec![];
    let mut field = vec![];
    let mut quoted = false;
    let mut i = 0;
    while i < sample.len() {
        let c = sample[i];
        if quoted {
            if c == b'"' {
                if sample.get(i + 1) == Some(&b'"') {
                    field.push(b'"');
                    i += 1;
                } else {
                    quoted = false;
                }
            } else {
                field.push(c);
            }
        } else if c == b'"' && field.is_empty() {
            quoted = true;
        } else if c == delim {
            rec.push(String::from_utf8_lossy(&field).into_owned());
            field.clear();
        } else if c == b'\n' || c == b'\r' {
            if c == b'\r' && sample.get(i + 1) == Some(&b'\n') {
                i += 1;
            }
            rec.push(String::from_utf8_lossy(&field).into_owned());
            field.clear();
            out.push(std::mem::take(&mut rec));
        } else {
            field.push(c);
        }
        i += 1;
    }
    if !field.is_empty() || !rec.is_empty() {
        rec.push(String::from_utf8_lossy(&field).into_owned());
        out.push(rec);
    }
    out
}

fn looks_numeric(s: &str) -> bool {
    let t = s.trim();
    !t.is_empty() && t.parse::<f64>().is_ok()
}

/// Choose the delimiter that splits the sample into the most consistent,
/// widest records; then decide the header with [`first_is_header`].
pub fn dialect(sample: &[u8], format: &str) -> Dialect {
    let sample = sample.strip_prefix(b"\xef\xbb\xbf").unwrap_or(sample);
    let candidates: &[u8] = if format == "tsv" { b"\t" } else { b",\t;|" };
    let mut best = (b',', 0usize, 0usize); // delimiter, consistent rows, width
    for &d in candidates {
        let recs = records(sample, d);
        let recs: Vec<_> = recs.into_iter().take(50).collect();
        if recs.is_empty() {
            continue;
        }
        let width = recs[0].len();
        let consistent = recs.iter().filter(|r| r.len() == width).count();
        if width > 1 && (consistent > best.1 || (consistent == best.1 && width > best.2)) {
            best = (d, consistent, width);
        }
    }
    let delimiter = if best.1 == 0 {
        if format == "tsv" { b'\t' } else { b',' }
    } else {
        best.0
    };
    let recs = records(sample, delimiter);
    let has_header = first_is_header(&recs);
    Dialect {
        delimiter,
        has_header,
    }
}

/// Decide whether the first record is a header by comparing it with the
/// records after it, column by column, as DuckDB's sniffer does: a column
/// whose later values are all numeric marks the first record as a header
/// when the first record's value there is not numeric. So `1,Alpha,10.50`
/// followed by `2,Beta,20.00` is data, while `id,name` followed by `1,a` is
/// a header. With no numeric column to compare, or a single record, the
/// first record is a header unless all of its fields are numeric.
fn first_is_header(recs: &[Vec<String>]) -> bool {
    let Some(first) = recs.first() else {
        return true;
    };
    let all_numeric = !first.is_empty() && first.iter().all(|f| looks_numeric(f));
    let later: Vec<&Vec<String>> = recs.iter().skip(1).take(1000).collect();
    let mut numeric_columns = 0;
    for (j, head) in first.iter().enumerate() {
        let mut values = later
            .iter()
            .filter_map(|r| r.get(j))
            .filter(|v| !v.trim().is_empty())
            .peekable();
        if values.peek().is_none() || !values.all(|v| looks_numeric(v)) {
            continue; // text, or no data to compare
        }
        numeric_columns += 1;
        if !head.trim().is_empty() && !looks_numeric(head) {
            return true; // a name above a numeric column
        }
    }
    if numeric_columns > 0 {
        return false; // the first record fits every numeric column
    }
    !all_numeric
}

/// Columns whose sampled values include a zero-padded integer such as
/// `007`. Typing those as integers would drop the padding, so DuckDB keeps
/// them as text; so does this candidate.
pub fn zero_padded_columns(sample: &[u8], d: &Dialect) -> Vec<usize> {
    let sample = sample.strip_prefix(b"\xef\xbb\xbf").unwrap_or(sample);
    let recs = records(sample, d.delimiter);
    let skip = usize::from(d.has_header);
    let mut cols = vec![];
    for rec in recs.iter().skip(skip).take(1000) {
        for (j, f) in rec.iter().enumerate() {
            let digits = f.strip_prefix('-').unwrap_or(f);
            if digits.len() > 1
                && digits.starts_with('0')
                && digits.bytes().all(|b| b.is_ascii_digit())
                && !cols.contains(&j)
            {
                cols.push(j);
            }
        }
    }
    cols.sort_unstable();
    cols
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn picks_delimiters_and_headers() {
        let d = dialect(b"id|name\n1|a\n2|b\n", "txt");
        assert_eq!((d.delimiter, d.has_header), (b'|', true));
        let d = dialect(b"id;amount\n1;1,5\n2;-2,25\n", "csv");
        assert_eq!(d.delimiter, b';');
        let d = dialect(b"1,2,3\n4,5,6\n", "csv");
        assert_eq!((d.delimiter, d.has_header), (b',', false));
        let d = dialect(b"id,text\n1,\"a, b\"\n", "csv");
        assert_eq!(d.delimiter, b',');
        let d = dialect(b"\xef\xbb\xbfid,name\n1,a\n", "csv");
        assert!(d.has_header);
        // Headerless records that mix text and numbers stay data (Codex
        // review of #153); a header above numeric columns is still found.
        let d = dialect(b"1,Alpha,10.50\n2,Beta,20.00\n", "csv");
        assert_eq!((d.delimiter, d.has_header), (b',', false));
        let d = dialect(b"id,name,amount\n1,Alpha,10.50\n2,Beta,20.00\n", "csv");
        assert!(d.has_header);
        // Only text columns: keep treating the first record as a header.
        let d = dialect(b"name,city\nBob,Paris\nAda,Rome\n", "csv");
        assert!(d.has_header);
        // A single record falls back to the all-numeric rule.
        assert!(!dialect(b"1,2,3\n", "csv").has_header);
        assert!(dialect(b"id,name\n", "csv").has_header);
        let s = b"code,qty,neg\n007,1,-05\n12,2,3\n";
        let d = dialect(s, "csv");
        assert_eq!(zero_padded_columns(s, &d), vec![0, 2]);
    }
}
