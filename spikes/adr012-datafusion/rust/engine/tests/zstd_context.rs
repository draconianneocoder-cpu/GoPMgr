// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

use std::io::{Read, Write};

#[test]
fn encoder_context_reuse_excludes_abandoned_payload() {
    let mut context = zstd::zstd_safe::CCtx::create();
    zstd::stream::write::Encoder::with_context(Vec::new(), &mut context)
        .write_all(b"abandoned private payload")
        .unwrap();
    let mut next = zstd::stream::write::Encoder::with_context(Vec::new(), &mut context);
    next.write_all(b"selected payload").unwrap();
    let compressed = next.finish().unwrap();
    assert_eq!(
        zstd::decode_all(compressed.as_slice()).unwrap(),
        b"selected payload"
    );
}

#[test]
fn decoder_context_reuse_excludes_abandoned_payload() {
    let abandoned = zstd::encode_all(&b"abandoned private payload"[..], 1).unwrap();
    let selected = zstd::encode_all(&b"selected payload"[..], 1).unwrap();
    let mut context = zstd::zstd_safe::DCtx::create();
    zstd::stream::read::Decoder::with_context(abandoned.as_slice(), &mut context)
        .read_exact(&mut [0u8; 3])
        .unwrap();
    let mut actual = Vec::new();
    zstd::stream::read::Decoder::with_context(selected.as_slice(), &mut context)
        .read_to_end(&mut actual)
        .unwrap();
    assert_eq!(actual, b"selected payload");
}
