// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package wbs

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
)

// Parse rejects ambiguous or unsupported writable payloads and never returns a
// partially decoded aggregate. Unknown fields must not be silently discarded.
func Parse(raw []byte) (Aggregate, error) {
	if len(raw) > MaxPayloadBytes {
		return Aggregate{}, fmt.Errorf("%w: payload", ErrLimit)
	}
	if err := preflight(raw); err != nil {
		return Aggregate{}, err
	}
	var a Aggregate
	if err := json.Unmarshal(raw, &a, json.RejectUnknownMembers(true)); err != nil {
		return Aggregate{}, fmt.Errorf("%w: decode: %w", ErrInvalid, err)
	}
	if err := Validate(a); err != nil {
		return Aggregate{}, err
	}
	return a, nil
}

func preflight(raw []byte) error {
	d := jsontext.NewDecoder(bytes.NewReader(raw))
	objects := 0
	first := true
	for {
		token, err := d.ReadToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: JSON: %w", ErrInvalid, err)
		}
		if first && token.Kind() != '{' {
			return fmt.Errorf("%w: expected object", ErrInvalid)
		}
		first = false
		// A legal level-64 node uses 128 containers; its dictionary adds one.
		if d.StackDepth() > 2*MaxLevels+2 {
			return fmt.Errorf("%w: JSON nesting", ErrLimit)
		}
		if token.Kind() == 'n' {
			return fmt.Errorf("%w: null member", ErrInvalid)
		}
		if token.Kind() == '"' && len(token.String()) > MaxTextBytes {
			return fmt.Errorf("%w: decoded string", ErrLimit)
		}
		if token.Kind() == '{' {
			objects++
			// At most an envelope, a dictionary for each node and link objects.
			if objects > 1+2*MaxNodes+MaxAssociations {
				return fmt.Errorf("%w: JSON objects", ErrLimit)
			}
		}
		if d.StackDepth() == 0 {
			if _, err := d.ReadToken(); !errors.Is(err, io.EOF) {
				return fmt.Errorf("%w: trailing JSON", ErrInvalid)
			}
			return nil
		}
	}
	return fmt.Errorf("%w: missing object", ErrInvalid)
}

// sizeWriter counts bytes without retaining a serialized copy. The model has
// already passed topology/field checks before the encoder traverses it.
type sizeWriter struct{ remaining int }

func (w *sizeWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, ErrLimit
	}
	w.remaining -= len(p)
	return len(p), nil
}

func encodedSize(a Aggregate) error {
	if err := json.MarshalWrite(&sizeWriter{MaxPayloadBytes}, &a); err != nil {
		return fmt.Errorf("wbs: encode: %w", err)
	}
	return nil
}
