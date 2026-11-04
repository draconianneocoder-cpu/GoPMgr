// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"gopmgr/internal/desktop/runtimehost"
	"gopmgr/spikes/wails-v3-shell/internal/closeguard"
)

//go:embed assets
var assets embed.FS

var errInjectedStartup = errors.New("injected startup failure")

type failingService struct{}

func (*failingService) ServiceStartup(context.Context, application.ServiceOptions) error {
	fmt.Println("probe:service-starting")
	return errInjectedStartup
}

func main() {
	resource, err := os.OpenFile(os.Getenv("SHELL_RESOURCE"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("probe:resource-open")
	owner := newOwnedFile(resource)
	life := runtimehost.New()
	defer func() {
		if err := life.FinishAfterRun(); err != nil {
			fmt.Println("probe:error", err)
			return
		}
		if err := owner.Close(); err != nil {
			fmt.Println("probe:error", err)
		}
	}()
	if _, err := resource.WriteString("saved draft"); err != nil {
		fmt.Println("probe:error", err)
		return
	}
	scenario := os.Getenv("SHELL_SCENARIO")
	var gate closeguard.Gate
	var app *application.App
	drainQuit := func() error {
		return life.BeginShutdown(func() {
			fmt.Println("probe:bindings-drained")
			application.InvokeAsync(app.Quit)
		})
	}
	request := func() bool {
		allow, prompt := gate.Request()
		if prompt {
			fmt.Println("probe:prompt")
			app.Event.Emit("probe:prompt")
		}
		if !allow {
			return false
		}
		if life.Ready() {
			return true
		}
		if err := drainQuit(); err != nil {
			fmt.Println("probe:error", err)
		}
		return false
	}
	options := application.Options{
		Name:       "GoPMgr Wails v3 qualification",
		Assets:     application.AssetOptions{Handler: application.BundledAssetFileServer(assets), Middleware: life.Middleware},
		Mac:        application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		ShouldQuit: request,
		OnShutdown: func() {
			fmt.Println("probe:shutdown")
			if !life.Ready() {
				fmt.Println("probe:error shutdown before bridge drainage")
				return
			}
			if err := owner.Close(); err != nil {
				fmt.Println("probe:error", err)
			}
		},
		PostShutdown: func() { fmt.Println("probe:post-shutdown") },
	}
	if scenario == "startup-failure" {
		options.Services = []application.Service{application.NewService(&failingService{})}
	}
	var service *probeService
	if scenario == "inflight" || scenario == "update-quit" {
		service = &probeService{owner: owner, ctx: life.Context()}
		options.Services = []application.Service{application.NewService(service)}
	}
	app = application.New(options)
	if service != nil {
		service.app = app
	}
	app.Menu.Set(application.DefaultApplicationMenu())
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "GoPMgr Wails v3 qualification", Width: 800, Height: 600})
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !request() {
			e.Cancel()
		}
	})
	var savedMu sync.Mutex
	var saveAttempts int
	saved, discarded := false, false
	dispatchAttempts := 0
	app.Event.On("probe:ready", func(*application.CustomEvent) {
		if scenario != "bootstrap" {
			gate.Enable()
		}
		fmt.Println("probe:ready")
		app.Event.Emit("probe:scenario", scenario)
	})
	app.Event.On("probe:open-dialog", func(*application.CustomEvent) {
		fmt.Println("probe:dialog-opened")
		if err := cancelDialog(app, window, resource.Name(), scenario); err != nil {
			fmt.Println("probe:error", err)
			return
		}
		fmt.Println("probe:dialog-cancelled")
		window.Close()
	})
	app.Event.On("probe:work-error", func(*application.CustomEvent) { fmt.Println("probe:error bridge call failed") })
	app.Event.On("probe:waiting-os", func(*application.CustomEvent) {
		fmt.Println("probe:waiting-os")
	})
	app.Event.On("probe:dispatch-recovery", func(*application.CustomEvent) {
		fmt.Println("probe:dispatch-recovery")
		app.Event.Emit("probe:retry-close")
	})
	app.Event.On("probe:request", func(*application.CustomEvent) {
		if scenario == "quit" {
			app.Quit()
		} else {
			window.Close()
		}
	})
	app.Event.On("probe:cancel", func(*application.CustomEvent) {
		gate.Cancel()
		fmt.Println("probe:cancelled")
		app.Event.Emit("probe:cancelled")
	})
	app.Event.On("probe:save", func(e *application.CustomEvent) {
		savedMu.Lock()
		saveAttempts++
		fmt.Println("probe:save-called")
		if scenario == "retry" && saveAttempts == 1 {
			savedMu.Unlock()
			fmt.Println("probe:save-failed")
			app.Event.Emit("probe:save-result", false)
			return
		}
		data, ok := e.Data.(string)
		if !ok {
			savedMu.Unlock()
			fmt.Println("probe:error invalid draft")
			return
		}
		release, err := owner.begin()
		if err == nil {
			_, err = resource.WriteAt([]byte(data), 0)
			if err == nil {
				err = resource.Truncate(int64(len(data)))
			}
			if err == nil {
				err = resource.Sync()
			}
			release()
		}
		saved = err == nil
		savedMu.Unlock()
		if err != nil {
			fmt.Println("probe:error", err)
			return
		}
		fmt.Println("probe:saved")
		app.Event.Emit("probe:save-result", true)
	})
	app.Event.On("probe:discard", func(*application.CustomEvent) {
		savedMu.Lock()
		discarded = true
		savedMu.Unlock()
		fmt.Println("probe:discarded")
		app.Event.Emit("probe:discard-result")
	})
	app.Event.On("probe:draft-preserved", func(*application.CustomEvent) { fmt.Println("probe:draft-preserved"); app.Event.Emit("probe:retry") })
	app.Event.On("probe:approve", func(*application.CustomEvent) {
		savedMu.Lock()
		mayApprove := saved || discarded
		savedMu.Unlock()
		if !mayApprove {
			fmt.Println("probe:error approval before save/discard")
			return
		}
		if err := gate.Approve(); err != nil {
			fmt.Println("probe:error", err)
			return
		}
		fmt.Println("probe:approved")
		dispatchAttempts++
		if scenario == "dispatch-failure" && dispatchAttempts == 1 {
			fmt.Println("probe:dispatch-failed")
			gate.Cancel()
			app.Event.Emit("probe:recover-dispatch")
			return
		}
		if err := drainQuit(); err != nil {
			fmt.Println("probe:error", err)
		}
	})
	if err := app.Run(); err != nil {
		if scenario == "startup-failure" && errors.Is(err, errInjectedStartup) {
			fmt.Println("probe:startup-failed")
		} else {
			fmt.Println("probe:error", err)
		}
	}
}
