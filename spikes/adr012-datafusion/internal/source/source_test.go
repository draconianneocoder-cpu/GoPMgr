// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package source

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func captured(t *testing.T, data string) (*Snapshot, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "literal?[x]*.parquet")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Capture(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func granted(t *testing.T, s *Snapshot, limits Limits) *Grant {
	t.Helper()
	g, err := s.Bind("job", time.Now().Add(time.Hour), limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	return g
}

func read(g *Grant, request, offset uint64, length uint32) (*Lease, error) {
	d := g.Descriptor()
	return g.Read(context.Background(), d.JobID, d.SourceID, d.Version, request, offset, length)
}

func TestCapturedSourceIsStableAfterMutationAndDeletion(t *testing.T) {
	s, path := captured(t, "abcdef")
	g := granted(t, s, Limits{ReadBytes: 6, TotalBytes: 12, Requests: 3, InFlight: 1})
	d := g.Descriptor()
	if d.Size != 6 || d.SHA256 != sha256.Sum256([]byte("abcdef")) {
		t.Fatalf("wrong identity: %+v", d)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	l, err := read(g, 1, 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	if string(l.Data) != "abcdef" {
		t.Fatalf("source changed: %q", l.Data)
	}
	l.Data[0] = 'X'
	l.Close()
	l.Close()
	l, err = read(g, 2, 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if string(l.Data) != "abcdef" {
		t.Fatalf("returned bytes alias the snapshot: %q", l.Data)
	}
	stats := g.Stats()
	if stats.PhysicalBytes != 6 || stats.ActualBytes != 12 || stats.RequestedBytes != 12 || stats.Requests != 2 || stats.PeakInFlight != 1 {
		t.Fatalf("wrong I/O accounting: %+v", stats)
	}
}

func TestSourceAuthorizationAndBounds(t *testing.T) {
	s, _ := captured(t, "abc")
	tests := []struct {
		name                 string
		job, source, version string
		request, offset      uint64
		length               uint32
		want                 error
	}{
		{name: "other job", job: "bad", request: 1, length: 1, want: ErrDenied},
		{name: "other source", source: strings.Repeat("a", 32), request: 1, length: 1, want: ErrDenied},
		{name: "stale version", version: strings.Repeat("a", 64), request: 1, length: 1, want: ErrDenied},
		{name: "gap", request: 2, length: 1, want: ErrSequence},
		{name: "zero sequence", request: 0, length: 1, want: ErrSequence},
		{name: "offset beyond end", request: 1, offset: 4, want: ErrRange},
		{name: "overflowing offset", request: 1, offset: ^uint64(0), length: 2, want: ErrRange},
		{name: "length beyond end", request: 1, offset: 2, length: 2, want: ErrRange},
		{name: "oversize request", request: 1, length: 3, want: ErrLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := granted(t, s, Limits{ReadBytes: 2, TotalBytes: 6, Requests: 4, InFlight: 1})
			d := g.Descriptor()
			job, id, version := d.JobID, d.SourceID, d.Version
			if tt.job != "" {
				job = tt.job
			}
			if tt.source != "" {
				id = tt.source
			}
			if tt.version != "" {
				version = tt.version
			}
			l, err := g.Read(context.Background(), job, id, version, tt.request, tt.offset, tt.length)
			if !errors.Is(err, tt.want) || l != nil {
				t.Fatalf("read = %v, %v; want %v", l, err, tt.want)
			}
			if stats := g.Stats(); stats.ActualBytes != 0 || stats.Requests != 0 || stats.Rejected != 1 {
				t.Fatalf("rejected read returned/accounted data: %+v", stats)
			}
		})
	}
}

func TestCreditsQuotasRetryAndRevocation(t *testing.T) {
	s, _ := captured(t, "abcd")
	g := granted(t, s, Limits{ReadBytes: 2, TotalBytes: 3, Requests: 2, InFlight: 1})
	l, err := read(g, 1, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := read(g, 2, 2, 1); !errors.Is(err, ErrLimit) {
		t.Fatalf("in-flight limit: %v", err)
	}
	l.Close()
	l.Close()
	if _, err := read(g, 2, 2, 2); !errors.Is(err, ErrLimit) {
		t.Fatalf("byte budget: %v", err)
	}
	l, err = read(g, 2, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	if _, err := read(g, 2, 0, 0); !errors.Is(err, ErrSequence) {
		t.Fatalf("replay: %v", err)
	}
	if _, err := read(g, 3, 3, 0); !errors.Is(err, ErrLimit) {
		t.Fatalf("request quota: %v", err)
	}
	g.Close()
	if _, err := read(g, 3, 0, 0); !errors.Is(err, ErrDenied) {
		t.Fatalf("revoked grant: %v", err)
	}
}

func TestExpiryCancellationAndEmptySource(t *testing.T) {
	s, _ := captured(t, "")
	g := granted(t, s, Limits{ReadBytes: 1, Requests: 3, InFlight: 1})
	l, err := read(g, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := g.Descriptor()
	if _, err := g.Read(ctx, d.JobID, d.SourceID, d.Version, 2, 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	g.now = func() time.Time { return d.Expires }
	if _, err := read(g, 2, 0, 0); !errors.Is(err, ErrDenied) {
		t.Fatalf("expiry boundary: %v", err)
	}
	if _, err := s.Bind("job", time.Now().Add(-time.Hour), Limits{}); !errors.Is(err, ErrDenied) {
		t.Fatalf("expired bind: %v", err)
	}
}

func TestCaptureAcceptsSelectedAliasAndRefusesNonregularAndOversize(t *testing.T) {
	_, path := captured(t, "alias data")
	alias := filepath.Join(filepath.Dir(path), "alias")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	s, err := Capture(alias)
	if err != nil || string(s.data) != "alias data" {
		t.Fatalf("selected alias: %v", err)
	}
	if _, err := Capture(filepath.Dir(path)); err == nil {
		t.Fatal("directory accepted")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "large"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxSourceBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(f.Name()); !errors.Is(err, ErrLimit) {
		t.Fatalf("oversize source: %v", err)
	}
}

func TestGrantCannotReadAnotherGrant(t *testing.T) {
	s, _ := captured(t, "abc")
	a := granted(t, s, Limits{ReadBytes: 3, TotalBytes: 3, Requests: 1, InFlight: 1})
	b := granted(t, s, Limits{ReadBytes: 3, TotalBytes: 3, Requests: 1, InFlight: 1})
	d := a.Descriptor()
	if l, err := b.Read(context.Background(), d.JobID, d.SourceID, d.Version, 1, 0, 3); !errors.Is(err, ErrDenied) || l != nil {
		t.Fatalf("cross-grant read: %v, %v", l, err)
	}
}

func TestConcurrentDuplicateRequestGetsOneLease(t *testing.T) {
	s, _ := captured(t, "abc")
	g := granted(t, s, Limits{ReadBytes: 3, TotalBytes: 6, Requests: 2, InFlight: 1})
	start := make(chan struct{})
	results := make(chan error, 2)
	leases := make(chan *Lease, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; l, err := read(g, 1, 0, 3); leases <- l; results <- err }()
	}
	close(start)
	success, refused := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, ErrSequence) {
			refused++
		} else {
			t.Errorf("unexpected rejection: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		if l := <-leases; l != nil {
			l.Close()
		}
	}
	if success != 1 || refused != 1 || g.Stats().Requests != 1 || g.Stats().ActualBytes != 3 {
		t.Fatalf("duplicate executed twice: %d %d %+v", success, refused, g.Stats())
	}
}
