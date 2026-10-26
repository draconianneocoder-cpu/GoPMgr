// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import (
	"crypto/subtle"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gopmgr/spikes/adr012-datafusion/internal/df0pb"
)

// ProofHeader carries the hex-encoded launch credential.
const ProofHeader = "x-df0-proof"

// session is the host side of one job: one endpoint, one expected worker,
// one stream.
type session struct {
	df0pb.UnimplementedAnalyticsSessionServer

	proof   []byte // hex text of the credential
	expires time.Time
	now     func() time.Time

	mu      sync.Mutex
	used    bool
	reasons []string
	refused atomic.Int64

	streams chan df0pb.AnalyticsSession_ConnectServer
	done    chan struct{}
}

func newSession(proof []byte, expires time.Time) *session {
	return &session{
		proof:   proof,
		expires: expires,
		now:     time.Now,
		streams: make(chan df0pb.AnalyticsSession_ConnectServer, 1),
		done:    make(chan struct{}),
	}
}

func (s *session) note(reason string) {
	s.refused.Add(1)
	s.mu.Lock()
	s.reasons = append(s.reasons, reason)
	s.mu.Unlock()
}

func (s *session) refusals() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.reasons...)
}

// authorize runs before any handler: the proof must match, be unexpired,
// and be used once only (DF-024).
func (s *session) authorize(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if s.now().After(s.expires) {
		s.note("expired")
		return status.Error(codes.Unauthenticated, "launch credential expired")
	}
	md, _ := metadata.FromIncomingContext(ss.Context())
	got := md.Get(ProofHeader)
	if len(got) != 1 || subtle.ConstantTimeCompare([]byte(got[0]), s.proof) != 1 {
		s.note("bad proof")
		return status.Error(codes.Unauthenticated, "launch proof did not verify")
	}
	s.mu.Lock()
	if s.used {
		s.mu.Unlock()
		s.note("proof reused")
		return status.Error(codes.Unauthenticated, "launch proof already used")
	}
	s.used = true
	s.mu.Unlock()
	return handler(srv, ss)
}

// Connect hands the stream to the job driver and holds it open until the
// driver is done with it.
func (s *session) Connect(stream df0pb.AnalyticsSession_ConnectServer) error {
	s.streams <- stream
	<-s.done
	return nil
}

func newServer(s *session, maxFrame int) *grpc.Server {
	srv := grpc.NewServer(
		grpc.StreamInterceptor(s.authorize),
		grpc.MaxRecvMsgSize(maxFrame),
		grpc.MaxSendMsgSize(maxFrame),
		// Bound per-connection concurrency: this endpoint serves one stream.
		grpc.MaxConcurrentStreams(1),
	)
	df0pb.RegisterAnalyticsSessionServer(srv, s)
	return srv
}
