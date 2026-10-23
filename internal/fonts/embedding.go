// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Embedding bits of the OpenType OS/2 fsType field, in which a font records
// what its license allows.
const (
	fsTypeRestricted   = 0x0002 // must not be embedded
	fsTypePreviewPrint = 0x0004 // may be embedded in read-only documents
	fsTypeEditable     = 0x0008 // may be embedded in editable documents
	fsTypeNoSubsetting = 0x0100 // must be embedded whole, never as a subset
	fsTypeBitmapOnly   = 0x0200 // only its bitmaps may be embedded
)

// ErrEmbeddingNotAllowed reports a font whose license, as recorded in its
// OS/2 fsType field, does not allow the way GoPMgr embeds fonts: every PDF
// embeds a subset of the font's outlines.
var ErrEmbeddingNotAllowed = errors.New("its license doesn't allow embedding it in PDFs")

// checkEmbeddingAllowed returns ErrEmbeddingNotAllowed, with the reason,
// for a font that is marked restricted, that may not be subset, or that
// may embed only bitmaps. When several usage bits are set, the least
// restrictive applies, as the OpenType specification says. A font without
// an OS/2 table records no restriction.
func checkEmbeddingAllowed(b []byte) error {
	fsType, ok := os2FsType(b)
	if !ok {
		return nil
	}
	switch {
	case fsType&fsTypeRestricted != 0 && fsType&(fsTypePreviewPrint|fsTypeEditable) == 0:
		return fmt.Errorf("%w (it is marked restricted-license)", ErrEmbeddingNotAllowed)
	case fsType&fsTypeNoSubsetting != 0:
		return fmt.Errorf("%w (it may not be subset, and PDFs embed only the characters they use)", ErrEmbeddingNotAllowed)
	case fsType&fsTypeBitmapOnly != 0:
		return fmt.Errorf("%w (it allows embedding only bitmaps)", ErrEmbeddingNotAllowed)
	}
	return nil
}

// os2FsType reads the fsType field of a TrueType font's OS/2 table.
func os2FsType(b []byte) (uint16, bool) {
	if len(b) < 12 {
		return 0, false
	}
	numTables := int(binary.BigEndian.Uint16(b[4:6]))
	for i := 0; i < numTables; i++ {
		rec := 12 + 16*i
		if rec+16 > len(b) {
			return 0, false
		}
		if string(b[rec:rec+4]) != "OS/2" {
			continue
		}
		off := uint64(binary.BigEndian.Uint32(b[rec+8:]))
		if off+10 > uint64(len(b)) {
			return 0, false
		}
		return binary.BigEndian.Uint16(b[off+8:]), true
	}
	return 0, false
}
