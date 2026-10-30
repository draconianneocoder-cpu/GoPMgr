# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Independent reader for the DF0 fixture self-check (ADR-012 DF-A11).

Reads every Parquet fixture with pyarrow (Arrow's C++ implementation, which
shares no code with the arrow-go writer, DuckDB or arrow-rs) and every JSON
and NDJSON fixture with Python's json module, and prints one JSON line per
file with cells in the corpus's Kind/value form. cmd/df0-selfcheck compares
them with the generator's truth.

Usage: python -I independent_read.py CORPUS_DIR > reads.jsonl
"""

import datetime
import decimal
import json
import math
import os
import sys

import pyarrow as pa
import pyarrow.parquet as pq

PREVIEW = 10_000
EPOCH = datetime.datetime(1970, 1, 1)
UNIT = {"s": 1, "ms": 1_000, "us": 1_000_000, "ns": 1_000_000_000}


def float_text(f):
    if math.isnan(f):
        return "NaN"
    if math.isinf(f):
        return "+Inf" if f > 0 else "-Inf"
    if f == 0 and math.copysign(1, f) < 0:
        return "-0"
    return repr(f)


def ts_text(raw, unit, tz):
    per = UNIT[unit]
    secs, frac = divmod(raw, per)
    t = EPOCH + datetime.timedelta(seconds=secs)
    s = t.strftime("%Y-%m-%dT%H:%M:%S")
    if t.year < 1000:
        s = "%04d" % t.year + s[s.index("-"):]
    if frac:
        digits = str(frac * (1_000_000_000 // per)).rjust(9, "0").rstrip("0")
        s += "." + digits
    return s + ("Z" if tz else "")


def typed(v, typ):
    """Convert a pyarrow Python value by its declared Arrow type."""
    if v is None:
        return None
    if pa.types.is_map(typ):
        return {str(k): typed(x, typ.item_type) for k, x in v}
    if pa.types.is_list(typ) or pa.types.is_large_list(typ):
        return [typed(e, typ.value_type) for e in v]
    if pa.types.is_struct(typ):
        return {typ.field(i).name: typed(v[typ.field(i).name], typ.field(i).type) for i in range(typ.num_fields)}
    if isinstance(v, decimal.Decimal):
        return NumberText(format(v, "f"))
    if isinstance(v, bytes):
        return v.hex()
    return v


class NumberText(str):
    """A decimal kept as written."""


class Obj(list):
    """A JSON object, as its (key, value) pairs in document order."""


def dump_json(v):
    def enc(o):
        if isinstance(o, NumberText):
            return str(o)
        if isinstance(o, bool) or o is None:
            return json.dumps(o)
        if isinstance(o, (int, float)):
            return json.dumps(o)
        if isinstance(o, str):
            return json.dumps(o, ensure_ascii=False)
        if isinstance(o, list):
            return "[" + ",".join(enc(e) for e in o) + "]"
        if isinstance(o, dict):
            return "{" + ",".join(json.dumps(k, ensure_ascii=False) + ":" + enc(o[k]) for k in sorted(o)) + "}"
        raise TypeError(type(o))
    return enc(v)


def cell(value, typ):
    if value is None:
        return {"k": "null", "v": ""}
    if pa.types.is_boolean(typ):
        return {"k": "bool", "v": "true" if value else "false"}
    if pa.types.is_integer(typ):
        return {"k": "int", "v": str(value)}
    if pa.types.is_floating(typ):
        return {"k": "float", "v": float_text(value)}
    if pa.types.is_decimal(typ):
        return {"k": "dec", "v": format(value, "f")}
    if pa.types.is_string(typ) or pa.types.is_large_string(typ):
        return {"k": "str", "v": value}
    if pa.types.is_binary(typ) or pa.types.is_large_binary(typ):
        return {"k": "bytes", "v": value.hex()}
    if pa.types.is_date32(typ):
        return {"k": "date", "v": value.isoformat()}
    if pa.types.is_nested(typ):
        return {"k": "json", "v": dump_json(typed(value, typ))}
    raise TypeError(str(typ))


def read_parquet(path):
    table = pq.read_table(path)
    cols = table.column_names
    n = min(table.num_rows, PREVIEW)
    columns = []
    for name in cols:
        col = table.column(name).combine_chunks().slice(0, n)
        typ = col.type
        if pa.types.is_timestamp(typ):
            raw = col.cast(pa.int64()).to_pylist()
            columns.append([{"k": "null", "v": ""} if r is None else {"k": "time", "v": ts_text(r, typ.unit, typ.tz)} for r in raw])
        else:
            columns.append([cell(v, typ) for v in col.to_pylist()])
    rows = [[columns[j][i] for j in range(len(cols))] for i in range(n)]
    return {"columns": cols, "rows": rows, "total_rows": table.num_rows}


def json_cell(v):
    if v is None:
        return {"k": "null", "v": ""}
    if isinstance(v, bool):
        return {"k": "bool", "v": "true" if v else "false"}
    if isinstance(v, NumberText):
        return {"k": "dec" if any(c in v for c in ".eE") else "int", "v": str(v)}
    if isinstance(v, str):
        return {"k": "str", "v": v}
    return {"k": "json", "v": dump_json(v)}


def read_json(path):
    text = open(path, encoding="utf-8").read()
    dec = json.JSONDecoder(parse_float=NumberText, parse_int=NumberText, object_pairs_hook=Obj)
    records, i = [], 0
    while True:
        while i < len(text) and text[i] in " \t\r\n":
            i += 1
        if i >= len(text):
            break
        v, i = dec.raw_decode(text, i)
        if isinstance(v, Obj):
            records.append(v)
        elif isinstance(v, list) and all(isinstance(e, Obj) for e in v):
            records.extend(v)
        else:
            raise ValueError("top-level value is not an object or an array of objects")
    cols = []
    for r in records:
        for k, _ in r:
            if k not in cols:
                cols.append(k)
    rows = []
    for r in records[:PREVIEW]:
        d = {}
        for k, v in r:
            d[k] = v  # the last duplicate wins
        rows.append([json_cell(pairs_to_plain(d[c])) if c in d else {"k": "null", "v": ""} for c in cols])
    return {"columns": cols, "rows": rows, "total_rows": len(records)}


def pairs_to_plain(v):
    if isinstance(v, Obj):
        return {k: pairs_to_plain(x) for k, x in v}
    if isinstance(v, list):
        return [pairs_to_plain(e) for e in v]
    return v


def main():
    corpus = sys.argv[1]
    manifest = json.load(open(os.path.join(corpus, "manifest.json"), encoding="utf-8"))
    print(json.dumps({"reader": "pyarrow " + pa.__version__ + "; python json " + sys.version.split()[0]}))
    for e in manifest["entries"]:
        if e["kind"] != "tabular" or e.get("large") or e["format"] not in ("parquet", "json", "ndjson"):
            continue
        path = os.path.join(corpus, e["file"])
        out = {"id": e["id"]}
        try:
            out.update(read_parquet(path) if e["format"] == "parquet" else read_json(path))
        except Exception as err:  # noqa: BLE001 -- recorded, compared with the case's Accept
            # Keep the message, not the machine-specific corpus path.
            out["error"] = type(err).__name__ + ": " + str(err).replace(corpus, "<corpus>")[:200]
        print(json.dumps(out, ensure_ascii=False))


if __name__ == "__main__":
    main()
