#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

"""Validate DF0 SPDX 3.0.1 offline against pinned official resources."""

import argparse
import copy
import hashlib
import importlib.metadata
import json
from pathlib import Path
import sys

from jsonschema import Draft202012Validator, FormatChecker
from pyshacl import validate
from rdflib import Graph


RESOURCE_HASHES = {
    "schema": "582c64e809d5b3ef9bd0c4de13a32391b47b0284a3e8d199569fb96f649234b1",
    "context": "c72b0928f094c83e5c127784edb1ebca2af74a104fcacc007c332b23cbc788bd",
    "model": "30ebb4af2d70a9809044ef46f44cc3dc5125226d70f818a50ed2e1d5f404c593",
}


def load_resource(path, kind):
    data = Path(path).read_bytes()
    if hashlib.sha256(data).hexdigest() != RESOURCE_HASHES[kind]:
        raise ValueError(f"unreviewed {kind} resource hash")
    return data


def check(document, schema, context, model):
    errors = list(Draft202012Validator(schema, format_checker=FormatChecker()).iter_errors(document))
    if errors:
        return {"accepted": False, "stage": "schema", "errors": len(errors)}
    nodes = document["@graph"]
    identifiers = [n.get("spdxId", n.get("@id")) for n in nodes]
    if None in identifiers or len(identifiers) != len(set(identifiers)):
        return {"accepted": False, "stage": "identity"}
    # Replace only the validated global context with its pinned local bytes.
    # JSON-LD expansion therefore does not fetch a context over the network.
    expanded = copy.deepcopy(document)
    expanded["@context"] = context["@context"]
    graph = Graph().parse(data=json.dumps(expanded), format="json-ld")
    # RDFS materialization creates explicit abstract rdf:types that the model
    # forbids. SHACL uses the ontology hierarchy without adding those types.
    conforms, _, _ = validate(
        graph, shacl_graph=model, ont_graph=model,
        inference="none", do_owl_imports=False,
    )
    return {"accepted": bool(conforms), "stage": "shacl", "triples": len(graph)}


def controls(document, schema, context, model):
    results = []
    for name, stage in [
        ("missing-relationship-target", "schema"),
        ("invalid-hash-algorithm", "schema"),
        ("duplicate-identity", "identity"),
        ("wrong-creator-class", "shacl"),
    ]:
        changed = copy.deepcopy(document)
        nodes = changed["@graph"]
        if name == "missing-relationship-target":
            del next(n for n in nodes if "relationshipType" in n)["to"]
        elif name == "invalid-hash-algorithm":
            next(n for n in nodes if n["type"] == "software_File")["verifiedUsing"][0]["algorithm"] = "not-a-hash"
        elif name == "duplicate-identity":
            nodes.append(copy.deepcopy(next(n for n in nodes if n["type"] == "Agent")))
        else:
            next(n for n in nodes if n["type"] == "Agent")["type"] = "software_File"
        outcome = check(changed, schema, context, model)
        if outcome["accepted"] or outcome["stage"] != stage:
            raise ValueError(f"control {name} was not rejected by {stage}: {outcome}")
        results.append({"control": name, **outcome})
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("document")
    for kind in RESOURCE_HASHES:
        parser.add_argument(f"--{kind}", required=True)
    parser.add_argument("--negative-controls", action="store_true")
    args = parser.parse_args()
    schema = json.loads(load_resource(args.schema, "schema"))
    context = json.loads(load_resource(args.context, "context"))
    model = Graph().parse(data=load_resource(args.model, "model").decode(), format="turtle")
    document = json.loads(Path(args.document).read_bytes())
    result = check(document, schema, context, model)
    if result["accepted"] and args.negative_controls:
        result["negative_controls"] = controls(document, schema, context, model)
    result["resource_sha256"] = RESOURCE_HASHES
    result["validators"] = {p: importlib.metadata.version(p) for p in ["jsonschema", "pyshacl", "rdflib"]}
    print(json.dumps(result, indent=2))
    return 0 if result["accepted"] else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError) as error:
        print(f"validate_spdx3: {error}", file=sys.stderr)
        sys.exit(1)
