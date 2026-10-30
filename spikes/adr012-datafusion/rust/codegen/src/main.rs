// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//! Writes the worker's client code for proto/df0ipc.proto.
//!
//!   df0-codegen PROTO_DIR OUT_DIR      (PROTOC names the pinned protoc)

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let [proto_dir, out_dir] = args.as_slice() else {
        return Err("usage: df0-codegen PROTO_DIR OUT_DIR".into());
    };
    tonic_prost_build::configure()
        .build_server(false)
        .build_client(true)
        .out_dir(out_dir)
        .compile_protos(&[format!("{proto_dir}/df0ipc.proto")], &[proto_dir.clone()])?;
    Ok(())
}
