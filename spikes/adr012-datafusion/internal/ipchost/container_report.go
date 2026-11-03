// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build linux || darwin

package ipchost

import "errors"

// SweepReport says what a container sweep undid. It never names an entry.
type SweepReport struct {
	EntriesRemoved  int    `json:"entries_removed"`
	XattrsRemoved   int    `json:"xattrs_removed"`
	ModesRestored   int    `json:"modes_restored"`
	FlagsCleared    int    `json:"flags_cleared"`
	BaselineChanged int    `json:"baseline_entries_changed"`
	Clean           bool   `json:"clean"`
	Failure         string `json:"failure,omitempty"`
}

// ErrContainerUnclean is the setup failure for a job refused because an
// earlier sweep could not restore the container.
var ErrContainerUnclean = errors.New("worker container is not clean")

// ErrContainerBusy refuses a job or sweep because another process holds the
// container's slot lock. Hosts never queue on another host's slot.
var ErrContainerBusy = errors.New("worker container is in use by another process")
