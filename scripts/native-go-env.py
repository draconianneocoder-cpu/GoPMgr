#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Apply the native macOS test baseline, then replace this process with its caller."""
import json
import os
import re
import shlex
import subprocess
import sys

FLOOR = "26.0"


def require_floor(value):
    if not re.fullmatch(r"26(?:\.0){0,2}", value):
        raise ValueError(f"deployment target {value!r} conflicts with macOS {FLOOR}")


def inspect_flags(value):
    has_floor = False
    for word in shlex.split(value):
        if word.startswith(("-mmacosx-version-min=", "-mmacos-version-min=")):
            require_floor(word.split("=", 1)[1])
            has_floor = True
        elif (word.startswith(("@", "--target", "-target", "-mtargetos", "-miphoneos", "-mios", "-darwin-target"))
              or word in ("-mmacosx-version-min", "-mmacos-version-min", "-triple")
              or any(part.startswith("@") or part in ("-platform_version", "-macosx_version_min", "-ios_version_min")
                     for part in word.split(","))):
            raise ValueError(f"ambiguous target control {word!r}; use -mmacosx-version-min={FLOOR}")
    return has_floor


def main(args):
    if len(args) < 3 or args[1] != "--":
        print("native-go-env: usage: <go executable> -- <command> [arguments]", file=sys.stderr)
        return 2
    try:
        result = subprocess.run(shlex.split(args[0]) + ["env", "-json", "GOOS", "GOHOSTOS"],
                                capture_output=True, text=True, check=True, timeout=30)
        identity = json.loads(result.stdout)
        if not isinstance(identity, dict) or any(
                not isinstance(identity.get(key), str) or not identity[key].strip()
                for key in ("GOOS", "GOHOSTOS")):
            raise ValueError("Go did not identify both target and host OS")
        env = dict(os.environ)
        if identity["GOOS"] == identity["GOHOSTOS"] == "darwin":
            if env.get("MACOSX_DEPLOYMENT_TARGET"):
                require_floor(env["MACOSX_DEPLOYMENT_TARGET"])
            # CPP flags can override a deployment target too, but do not need
            # an added floor: the C/C++ compiler flags already supply it.
            inspect_flags(env.get("CGO_CPPFLAGS", ""))
            for key in ("CGO_CFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS"):
                value = env.get(key, "-O2 -g")
                if not inspect_flags(value):
                    value += (" " if value else "") + f"-mmacosx-version-min={FLOOR}"
                env[key] = value
            env["MACOSX_DEPLOYMENT_TARGET"] = FLOOR
        os.execvpe(args[2], args[2:], env)
    except (OSError, ValueError, subprocess.SubprocessError) as exc:
        print(f"native-go-env: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
