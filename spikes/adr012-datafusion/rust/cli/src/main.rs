// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//! df0-cli: run the DF0-B DataFusion candidate on one input and print one
//! JSON result on stdout.
//!
//!   df0-cli import FORMAT PATH
//!   df0-cli rollup PROJECTS.json      (minor units, normalized by the host)
//!   df0-cli probe-sum-overflow

use std::process::ExitCode;

#[tokio::main(flavor = "multi_thread", worker_threads = 2)]
async fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let out = match args
        .iter()
        .map(String::as_str)
        .collect::<Vec<_>>()
        .as_slice()
    {
        ["import", format, path] => serde_json::to_string(&df0_engine::import(path, format).await),
        ["rollup", path] => {
            let bytes = match std::fs::read(path) {
                Ok(b) => b,
                Err(e) => {
                    eprintln!("df0-cli: read {path}: {e}");
                    return ExitCode::from(2);
                }
            };
            let projects: Vec<df0_engine::Project> = match serde_json::from_slice(&bytes) {
                Ok(p) => p,
                Err(e) => {
                    eprintln!("df0-cli: parse {path}: {e}");
                    return ExitCode::from(2);
                }
            };
            serde_json::to_string(&df0_engine::rollup(&projects).await)
        }
        ["probe-sum-overflow"] => serde_json::to_string(&df0_engine::probe_sum_overflow().await),
        _ => {
            eprintln!(
                "usage: df0-cli import FORMAT PATH | rollup PROJECTS.json | probe-sum-overflow"
            );
            return ExitCode::from(2);
        }
    };
    match out {
        Ok(s) => {
            println!("{s}");
            ExitCode::SUCCESS
        }
        Err(e) => {
            eprintln!("df0-cli: encode: {e}");
            ExitCode::from(1)
        }
    }
}
