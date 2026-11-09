#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Read product metadata from the JSON subset used by build/config.yml."""

import json
import pathlib
import sys


def read_info(path: pathlib.Path, field: str) -> str:
    text = "\n".join(line for line in path.read_text().splitlines() if not line.startswith("#"))
    config = json.loads(text)
    if config.get("version") != "3":
        raise ValueError("expected Wails build config version 3")
    value = config["info"][field]
    if not isinstance(value, str) or not value.strip() or "\n" in value:
        raise ValueError(f"info.{field} must be a nonempty single-line string")
    return value


if __name__ == "__main__":
    try:
        if len(sys.argv) != 2 or sys.argv[1] not in ("version", "productName", "productIdentifier"):
            raise ValueError("usage: wails-build-config.py version|productName|productIdentifier")
        root = pathlib.Path(__file__).resolve().parent.parent
        print(read_info(root / "build/config.yml", sys.argv[1]))
    except (OSError, ValueError, KeyError) as error:
        sys.exit(f"wails-build-config: {error}")
