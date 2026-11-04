// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gopmgr/internal/desktop/runtimehost"
)

// wailsRuntime owns one native application/window and its domain lifetime.
// Pointers are installed before Run and are immutable during dispatch.
type wailsRuntime struct {
	life   *runtimehost.Lifetime
	app    *application.App
	window *application.WebviewWindow
}

func newWailsRuntime() *wailsRuntime { return &wailsRuntime{life: runtimehost.New()} }

func (r *wailsRuntime) ready() bool { return r != nil && r.app != nil && r.window != nil }

func (r *wailsRuntime) emit(name string, data ...any) {
	if r.ready() {
		r.app.Event.Emit(name, data...)
	}
}

func (r *wailsRuntime) requestQuit() {
	if r.ready() {
		application.InvokeAsync(r.app.Quit)
	}
}

func (r *wailsRuntime) beginQuit() {
	if err := r.life.BeginShutdown(r.requestQuit); err != nil {
		// CompleteNativeClose reserved native UI admission before this call.
		log.Printf("native quit admission: %v", err)
	}
}

func (r *wailsRuntime) openFileDialog(_ context.Context, options application.OpenFileDialogOptions) (string, error) {
	if !r.ready() {
		return "", errors.New("native file dialog is not ready")
	}
	release, err := r.life.BeginUI()
	if err != nil {
		return "", err
	}
	defer release()
	options.Window = r.window
	options.CanChooseFiles = true
	return r.app.Dialog.OpenFileWithOptions(&options).PromptForSingleSelection()
}

func (r *wailsRuntime) saveFileDialog(_ context.Context, options application.SaveFileDialogOptions) (string, error) {
	if !r.ready() {
		return "", errors.New("native save dialog is not ready")
	}
	release, err := r.life.BeginUI()
	if err != nil {
		return "", err
	}
	defer release()
	options.Window = r.window
	return r.app.Dialog.SaveFileWithOptions(&options).PromptForSingleSelection()
}

func (r *wailsRuntime) withUI(action func()) {
	if !r.ready() {
		return
	}
	release, err := r.life.BeginUI()
	if err != nil {
		return
	}
	defer release()
	action()
}

func (r *wailsRuntime) withDismissibleUI(show func(func())) error {
	release, err := r.life.BeginUI()
	if err != nil {
		return err
	}
	done := sync.OnceFunc(release)
	launched := false
	defer func() {
		if !launched {
			done()
		}
	}()
	// MessageDialog.Show returns before dismissal on macOS and GTK 4.
	// Its explicit default/cancel button owns the lease after launch.
	show(done)
	launched = true
	return nil
}

func (r *wailsRuntime) showAbout(message string) {
	if !r.ready() {
		return
	}
	if err := r.withDismissibleUI(func(done func()) {
		dialog := r.app.Dialog.Info().SetTitle("About GoPMgr").SetMessage(message).AttachToWindow(r.window)
		dialog.AddButton("OK").SetAsDefault().OnClick(done)
		dialog.AddButton("Cancel").SetAsCancel().OnClick(done)
		dialog.Show()
	}); err != nil {
		log.Printf("native About dialog admission: %v", err)
	}
}
