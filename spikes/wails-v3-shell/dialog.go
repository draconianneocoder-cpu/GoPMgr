// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func cancelDialog(app *application.App, window *application.WebviewWindow, resourcePath, scenario string) error {
	directory := filepath.Dir(resourcePath)
	var path string
	var err error
	if scenario == "native-open-dialog-cancel" {
		path, err = app.Dialog.OpenFile().SetTitle(fmt.Sprintf("GoPMgr dialog %d", os.Getpid())).AttachToWindow(window).CanChooseFiles(true).SetDirectory(directory).PromptForSingleSelection()
	} else {
		path, err = app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{Title: fmt.Sprintf("GoPMgr dialog %d", os.Getpid())}).AttachToWindow(window).SetDirectory(directory).SetFilename("cancelled-output.txt").PromptForSingleSelection()
	}
	if err != nil {
		return err
	}
	if path != "" {
		return fmt.Errorf("cancel returned a selected path")
	}
	body, err := os.ReadFile(resourcePath)
	if err != nil {
		return err
	}
	if string(body) != "saved draft" {
		return fmt.Errorf("dialog cancel changed the saved draft")
	}
	if _, err := os.Stat(filepath.Join(directory, "cancelled-output.txt")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cancel output must remain absent: %v", err)
	}
	return nil
}
