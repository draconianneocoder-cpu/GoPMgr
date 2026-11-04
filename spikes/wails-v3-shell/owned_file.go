// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// ownedFile models a resource acquired before Wails service startup. Work
// admission and shutdown share a lock; no work may join after draining starts.
// Active work must not wait for native/UI callbacks while Close joins it.
type ownedFile struct {
	file     *os.File
	mu       sync.Mutex
	active   sync.WaitGroup
	closing  bool
	once     sync.Once
	stopping chan struct{}
	closeErr error
}

func newOwnedFile(f *os.File) *ownedFile { return &ownedFile{file: f, stopping: make(chan struct{})} }
func (o *ownedFile) begin() (func(), error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing {
		return nil, errors.New("resource is shutting down")
	}
	o.active.Add(1)
	return o.active.Done, nil
}
func (o *ownedFile) Close() error {
	o.once.Do(func() {
		o.mu.Lock()
		o.closing = true
		close(o.stopping)
		o.mu.Unlock()
		o.active.Wait()
		o.closeErr = o.file.Close()
		fmt.Println("probe:resource-closed")
	})
	return o.closeErr
}
