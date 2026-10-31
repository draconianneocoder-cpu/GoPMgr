// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package source grants one DF0 job bounded access to an immutable host snapshot.
// It is experiment code, not a production source service or OS sandbox.
package source

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const MaxSourceBytes = 64 << 20

var (
	ErrDenied   = errors.New("source grant refused")
	ErrRange    = errors.New("source range refused")
	ErrLimit    = errors.New("source limit exceeded")
	ErrSequence = errors.New("source request sequence refused")
	ErrChanged  = errors.New("source changed during capture")
)

// Snapshot owns captured bytes. No method exposes its mutable backing storage.
type Snapshot struct {
	data []byte
	sha  [32]byte
}

type Limits struct {
	ReadBytes  uint32
	TotalBytes uint64
	Requests   uint64
	InFlight   uint32
}

// Descriptor contains identity and limits, never the host's filename.
type Descriptor struct {
	JobID     string
	SourceID  string
	Version   string
	Size      uint64
	SHA256    [32]byte
	Expires   time.Time
	ReadBytes uint32
}

type Stats struct {
	Requests       uint64 `json:"requests"`
	Rejected       uint64 `json:"rejected"`
	RequestedBytes uint64 `json:"requested_bytes"`
	ActualBytes    uint64 `json:"actual_bytes"`
	// Capture reads the whole source; range pruning does not reduce this count.
	PhysicalBytes uint64 `json:"physical_bytes"`
	PeakInFlight  uint32 `json:"peak_in_flight"`
}

type Grant struct {
	mu         sync.Mutex
	snapshot   *Snapshot
	descriptor Descriptor
	limits     Limits
	now        func() time.Time
	closed     bool
	active     uint32
	next       uint64
	stats      Stats
}

func (s *Snapshot) Bind(jobID string, expires time.Time, limits Limits) (*Grant, error) {
	if s == nil || jobID == "" || !time.Now().Before(expires) {
		return nil, ErrDenied
	}
	if limits.ReadBytes == 0 || limits.ReadBytes > MaxSourceBytes || limits.TotalBytes > 16*MaxSourceBytes || limits.Requests == 0 || limits.Requests > 1<<20 || limits.InFlight == 0 || limits.InFlight > 16 {
		return nil, ErrLimit
	}
	var id [16]byte
	if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
		return nil, fmt.Errorf("source identity: %w", err)
	}
	return &Grant{
		snapshot: s, limits: limits, now: time.Now, next: 1,
		descriptor: Descriptor{JobID: jobID, SourceID: hex.EncodeToString(id[:]), Version: hex.EncodeToString(s.sha[:]), Size: uint64(len(s.data)), SHA256: s.sha, Expires: expires, ReadBytes: limits.ReadBytes},
		stats:      Stats{PhysicalBytes: uint64(len(s.data))},
	}, nil
}

func (g *Grant) Descriptor() Descriptor { return g.descriptor }

// Lease holds an in-flight credit until Close, including the response send.
// Data is a copy and may be handed to the transport without mutating the source.
type Lease struct {
	Data  []byte
	once  sync.Once
	grant *Grant
}

func (l *Lease) Close() {
	l.once.Do(func() { l.grant.mu.Lock(); l.grant.active--; l.grant.mu.Unlock() })
}

func (g *Grant) Read(ctx context.Context, jobID, sourceID, version string, request, offset uint64, length uint32) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	reject := func(err error) (*Lease, error) { g.stats.Rejected++; return nil, err }
	if g.closed || !g.now().Before(g.descriptor.Expires) || jobID != g.descriptor.JobID || sourceID != g.descriptor.SourceID || version != g.descriptor.Version {
		return reject(ErrDenied)
	}
	if request != g.next {
		return reject(ErrSequence)
	}
	if offset > g.descriptor.Size || uint64(length) > g.descriptor.Size-offset {
		return reject(ErrRange)
	}
	if length > g.limits.ReadBytes || g.active >= g.limits.InFlight || g.stats.Requests >= g.limits.Requests || uint64(length) > g.limits.TotalBytes-g.stats.ActualBytes {
		return reject(ErrLimit)
	}
	if err := ctx.Err(); err != nil {
		return reject(err)
	}
	data := append([]byte(nil), g.snapshot.data[int(offset):int(offset)+int(length)]...)
	g.next++
	g.active++
	g.stats.Requests++
	g.stats.RequestedBytes += uint64(length)
	g.stats.ActualBytes += uint64(len(data))
	g.stats.PeakInFlight = max(g.stats.PeakInFlight, g.active)
	return &Lease{Data: data, grant: g}, nil
}

func (g *Grant) Stats() Stats { g.mu.Lock(); defer g.mu.Unlock(); return g.stats }

// Close revokes future reads. Outstanding leases retain their copies and must close.
func (g *Grant) Close() { g.mu.Lock(); g.closed = true; g.snapshot = nil; g.mu.Unlock() }

func snapshotBytes(data []byte) *Snapshot { return &Snapshot{data: data, sha: sha256.Sum256(data)} }
