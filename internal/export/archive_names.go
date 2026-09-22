// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrUnsafeArchiveName means an archive entry name could escape the
// extraction directory, be refused or rewritten by a common filesystem, or
// overwrite another entry when extracted onto a case-insensitive one.
var ErrUnsafeArchiveName = errors.New("unsafe archive entry name")

const (
	// portableSegmentBudget bounds PortableArchiveSegment's output, leaving
	// callers room to append a disambiguating suffix within maxSegmentBytes.
	portableSegmentBudget = 200
	// maxSegmentBytes is the per-component limit shared by NTFS, APFS, and
	// ext4.
	maxSegmentBytes   = 255
	maxExtensionBytes = 32
)

// windowsReservedStems are the device names Windows refuses as a file name,
// with any extension ("NUL.txt" and "NUL.tar.gz" both mean NUL), per
// https://learn.microsoft.com/windows/win32/fileio/naming-a-file.
var windowsReservedStems = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"COM¹": true, "COM²": true, "COM³": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	"LPT¹": true, "LPT²": true, "LPT³": true,
}

// PortableArchiveSegment turns a stored filename into one archive path
// segment that every common extractor writes as a plain file inside the
// extraction directory. The stored name is host-sanitized only: on macOS and
// Linux a name such as `..\..\x.exe` survives filepath.Base intact, and a
// project may carry rows created on another operating system.
//
// It replaces invalid UTF-8, Windows-reserved characters, and control
// characters with "_", trims surrounding spaces and trailing dots, prefixes
// reserved device names with "_", and keeps the result within
// portableSegmentBudget bytes, preserving a short extension. It never returns
// an empty segment.
func PortableArchiveSegment(name string) string {
	name = strings.ToValidUTF8(name, "_")
	name = strings.Map(func(r rune) rune {
		if isUnportableRune(r) {
			return '_'
		}
		return r
	}, name)
	// One byte is held back for the reserved-name prefix below.
	name = boundSegment(strings.TrimSpace(name), portableSegmentBudget-1)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		return "attachment"
	}
	if hasReservedStem(name) {
		name = "_" + name
	}
	return name
}

// validateArchiveEntryNames checks every entry name, plus uniqueness across
// them all, before anything is fetched or written.
func validateArchiveEntryNames(sources []AttachmentZIPSource) error {
	seen := map[string]bool{attachmentsManifestName: true}
	for _, source := range sources {
		name := source.ZipEntryName
		if err := validateArchiveEntryName(name); err != nil {
			return err
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("archive entry %q: %w: collides with another entry on a case-insensitive filesystem", name, ErrUnsafeArchiveName)
		}
		seen[key] = true
	}
	return nil
}

// validateArchiveEntryName checks the safety properties directly rather than
// requiring PortableArchiveSegment's exact output, so a caller may extend a
// portable segment (for example with an ID suffix) up to maxSegmentBytes.
func validateArchiveEntryName(name string) error {
	unsafeName := func(reason string) error {
		return fmt.Errorf("archive entry %q: %w: %s", name, ErrUnsafeArchiveName, reason)
	}
	if !utf8.ValidString(name) {
		return unsafeName("not valid UTF-8")
	}
	for _, segment := range strings.Split(name, "/") {
		switch {
		case segment == "":
			return unsafeName("empty path segment or absolute path")
		case segment == "." || segment == "..":
			return unsafeName("relative path segment")
		case len(segment) > maxSegmentBytes:
			return unsafeName("path segment is too long")
		case strings.IndexFunc(segment, isUnportableRune) >= 0:
			return unsafeName("reserved or control character")
		case strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " "):
			return unsafeName("path segment ends with a dot or space")
		case hasReservedStem(segment):
			return unsafeName("reserved device name")
		}
	}
	return nil
}

func isUnportableRune(r rune) bool {
	return r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r)
}

// hasReservedStem reports whether the part before the first dot names a
// Windows device. Trailing spaces are ignored because Windows strips them
// before matching.
func hasReservedStem(segment string) bool {
	stem, _, _ := strings.Cut(segment, ".")
	return windowsReservedStems[strings.ToUpper(strings.TrimRight(stem, " "))]
}

// boundSegment shortens name to at most limit bytes on a rune boundary,
// keeping an extension of up to maxExtensionBytes.
func boundSegment(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	ext := ""
	if i := strings.LastIndexByte(name, '.'); i > 0 && len(name)-i <= maxExtensionBytes {
		ext = name[i:]
	}
	stem := name[:len(name)-len(ext)]
	end := limit - len(ext)
	for end > 0 && !utf8.RuneStart(stem[end]) {
		end--
	}
	return stem[:end] + ext
}
