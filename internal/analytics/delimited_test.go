// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// A candidate delimiter that cannot parse the sample must lose, even when it
// splits the header into more fields.
func TestSniffDelimiterRejectsCandidatesThatFailToParse(t *testing.T) {
	ds, err := readDelimited(context.Background(), strings.NewReader("id,name|type|unit\n1,\"Alpha\"\n2,\"Beta\"\n"), false)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	want := Dataset{Columns: []string{"id", "name|type|unit"}, Rows: [][]any{{int64(1), "Alpha"}, {int64(2), "Beta"}}}
	if !reflect.DeepEqual(ds, want) {
		t.Errorf("got %#v, want %#v", ds, want)
	}

	// A quoted field cut off by the end of a truncated sample is not a parse
	// failure of the candidate; in a complete file it is.
	sample := []byte("a;b\n1;\"x\n")
	if got := sniffDelimiter(sample, false, false); got != ';' {
		t.Errorf("truncated sample: delimiter %q, want ';'", got)
	}
	if got := sniffDelimiter(sample, false, true); got != ',' {
		t.Errorf("complete sample: delimiter %q, want the ',' fallback", got)
	}
}

// One row may not make the reader consume much more than the cap, however
// the row is split into lines. Measured in bytes read, not only by the error.
func TestRecordCapBoundsBytesRead(t *testing.T) {
	const total = 2*maxRecordBytes + 1<<20
	line := append(bytes.Repeat([]byte("x"), 1023), '\n')
	cases := []struct {
		name string
		read func(io.Reader) (Dataset, error)
		head string
		line []byte
	}{
		{"csv multi-line quoted field", func(r io.Reader) (Dataset, error) {
			return readDelimited(context.Background(), r, false)
		}, "a\n\"", line},
		{"json value over many lines", func(r io.Reader) (Dataset, error) {
			return readJSON(context.Background(), r, false)
		}, "[[", []byte("1,\n")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := &repeatReader{chunk: c.line, left: total}
			counted := &countingReader{r: io.MultiReader(strings.NewReader(c.head), body)}
			if _, err := c.read(counted); !errors.Is(err, ErrRecordTooLarge) {
				t.Fatalf("err = %v, want ErrRecordTooLarge", err)
			}
			if limit := int64(maxRecordBytes + 1<<20); counted.n > limit {
				t.Errorf("read %d bytes before refusing, want at most %d", counted.n, limit)
			}
		})
	}
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// repeatReader yields chunk repeatedly until left bytes have been produced.
type repeatReader struct {
	chunk []byte
	left  int
	off   int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.left <= 0 {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && r.left > 0 {
		k := copy(p[n:], r.chunk[r.off:])
		if k > r.left {
			k = r.left
		}
		n += k
		r.left -= k
		r.off = (r.off + k) % len(r.chunk)
	}
	return n, nil
}
