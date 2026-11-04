// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// probeService qualifies actual dispatch and lifetime cancellation without
// importing the production App or opening account/project data.
type probeService struct {
	app   *application.App
	owner *ownedFile
	ctx   context.Context
}

func (s *probeService) Work() (string, error) {
	release, err := s.owner.begin()
	if err != nil {
		return "", err
	}
	defer release()
	fmt.Println("probe:work-started")
	s.app.Event.Emit("probe:work-started")
	<-s.ctx.Done()
	// The domain context is canceled while the native loop is still running.
	if _, err := s.owner.file.Stat(); err != nil {
		fmt.Println("probe:error", err)
		return "", err
	}
	fmt.Println("probe:work-drained")
	return "completed work", nil
}

func (s *probeService) QuitToInstallUpdate() {
	fmt.Println("probe:update-quit")
	s.app.Quit()
}

func (s *probeService) ServiceShutdown() error {
	fmt.Println("probe:service-stopped")
	return nil
}
