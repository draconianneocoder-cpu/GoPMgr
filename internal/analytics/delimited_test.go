// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
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

// A fractional value promotes a numeric column to float, but must not round
// another row's integer. Check the concrete cells and the ordinary JSON
// decoding used at the JavaScript boundary, without a float-based oracle.
func TestImportDelimitedMixedNumbersKeepIntegerDigits(t *testing.T) {
	cases := []struct {
		name, integer string
		want          any
	}{
		{"positive safe boundary", "9007199254740991", float64(9007199254740991)},
		{"negative safe boundary", "-9007199254740991", float64(-9007199254740991)},
		{"positive unsafe boundary", "9007199254740992", "9007199254740992"},
		{"negative unsafe boundary", "-9007199254740992", "-9007199254740992"},
		{"positive rounded integer", "9007199254740993", "9007199254740993"},
		{"negative rounded integer", "-9007199254740993", "-9007199254740993"},
		{"int64 maximum", "9223372036854775807", "9223372036854775807"},
		{"int64 minimum", "-9223372036854775808", "-9223372036854775808"},
	}
	formats := []struct{ ext, delimiter string }{
		{"csv", ","}, {"tsv", "\t"}, {"txt", ";"}, {"txt", "|"},
	}
	for _, format := range formats {
		for _, tc := range cases {
			for _, decimal := range []string{"0.5", "5e-1"} {
				for _, reverse := range []bool{false, true} {
					name := format.ext + format.delimiter + "/" + tc.name + "/" + decimal
					if reverse {
						name += "/reversed"
					}
					t.Run(name, func(t *testing.T) {
						values := []string{tc.integer, decimal, "42", ""}
						wantRows := [][]any{{tc.want, "integer"}, {0.5, "decimal"}, {float64(42), "safe"}, {nil, "empty"}}
						labels := []string{"integer", "decimal", "safe", "empty"}
						if reverse {
							values[0], values[1] = values[1], values[0]
							wantRows[0], wantRows[1] = wantRows[1], wantRows[0]
							labels[0], labels[1] = labels[1], labels[0]
						}
						var input strings.Builder
						input.WriteString("value" + format.delimiter + "label\n")
						for i, value := range values {
							input.WriteString(value + format.delimiter + labels[i] + "\n")
						}
						path := filepath.Join(t.TempDir(), "mixed."+format.ext)
						if err := os.WriteFile(path, []byte(input.String()), 0600); err != nil {
							t.Fatal(err)
						}
						ds, err := ImportTabular(context.Background(), path)
						if err != nil {
							t.Fatal(err)
						}
						want := Dataset{Columns: []string{"value", "label"}, Rows: wantRows}
						if !reflect.DeepEqual(ds, want) {
							t.Fatalf("dataset = %#v, want %#v", ds, want)
						}
						encoded, err := json.Marshal(ds)
						if err != nil {
							t.Fatal(err)
						}
						var decoded Dataset
						if err := json.Unmarshal(encoded, &decoded); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(decoded, want) {
							t.Errorf("JSON dataset = %#v, want %#v", decoded, want)
						}
					})
				}
			}
		}
	}
}
