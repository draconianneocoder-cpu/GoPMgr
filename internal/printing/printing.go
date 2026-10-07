// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package printing sends a short plain-text page to a printer through the
// operating system's print service: CUPS's lp on macOS and Linux, and
// PowerShell's Out-Printer on Windows. The text goes on standard input, so
// GoPMgr writes no file; the print service still holds the job until it
// prints. It exists for the administrator key's recovery code (ADR-005
// part 3b).
//
// Only printers that print on paper are offered: ones that save to a file
// (PDF, XPS, file:, OneNote, fax) are left out, since a file would put the
// text on disk. The Windows path is built and unit-tested here but has not
// been run on Windows.
package printing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Connection says where a printer is.
type Connection string

// Printer connections.
const (
	// Local is a printer connected to this computer (USB, parallel,
	// serial).
	Local Connection = "local"
	// Network is a printer reached over the network, including one shared
	// from another computer.
	Network Connection = "network"
)

// Printer is a print destination that prints on paper.
type Printer struct {
	Name       string
	Connection Connection
	Default    bool
}

// ErrNoSuchPrinter is returned for a printer that is not in the listing.
var ErrNoSuchPrinter = errors.New("printing: no such printer")

// ErrUnsupported is returned where no print service was found.
var ErrUnsupported = errors.New("printing: no print service on this computer")

// timeout bounds every print-service command.
const timeout = 30 * time.Second

// Fixed paths: the print service is never looked up on PATH.
const (
	lpPath     = "/usr/bin/lp"
	lpstatPath = "/usr/bin/lpstat"
)

func powershellPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return root + `\System32\WindowsPowerShell\v1.0\powershell.exe`
}

// command is one print-service invocation.
type command struct {
	path  string
	args  []string
	env   []string // added to the environment
	stdin string
}

// windowsListScript prints one tab-separated line per printer. The tab is
// joined as [char]9: PowerShell's `t escape works only in double quotes.
const windowsListScript = `Get-CimInstance Win32_Printer | ForEach-Object { ($_.Name, $_.PortName, $_.Default, $_.Network) -join [char]9 }`

// runCommand runs c and returns its standard output. Tests replace it so
// nothing is printed.
var runCommand = func(ctx context.Context, c command) (string, error) {
	cmd := exec.CommandContext(ctx, c.path, c.args...) //nolint:gosec // fixed absolute path; arguments are a validated printer name and a fixed title
	cmd.Env = append(os.Environ(), c.env...)
	cmd.Stdin = strings.NewReader(c.stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("printing: %s", msg)
		}
		return "", fmt.Errorf("printing: %w", err)
	}
	return stdout.String(), nil
}

// statPath is os.Stat, replaced in tests.
var statPath = os.Stat

// Printers lists the printers this computer can print on paper with, the
// default first when it is one of them.
func Printers(ctx context.Context) ([]Printer, error) {
	return printersFor(ctx, runtime.GOOS)
}

// Print prints text on printer, which must be in Printers' listing, with a
// job title that must not contain anything secret: print queues show it.
func Print(ctx context.Context, printer, title, text string) error {
	return printFor(ctx, runtime.GOOS, printer, title, text)
}

func printersFor(ctx context.Context, goos string) ([]Printer, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if goos == "windows" {
		out, err := runCommand(ctx, command{
			path: powershellPath(),
			args: []string{"-NoProfile", "-NonInteractive", "-Command", windowsListScript},
		})
		if err != nil {
			return nil, err
		}
		return sortDefaultFirst(parseWindowsPrinters(out)), nil
	}
	if _, err := statPath(lpstatPath); err != nil {
		return nil, ErrUnsupported
	}
	// lpstat's wording is translated; C keeps it parseable.
	devices, err := runCommand(ctx, command{path: lpstatPath, args: []string{"-v"}, env: []string{"LC_ALL=C"}})
	if err != nil {
		return nil, err
	}
	def, err := runCommand(ctx, command{path: lpstatPath, args: []string{"-d"}, env: []string{"LC_ALL=C"}})
	if err != nil {
		def = "" // no default destination is not an error
	}
	return sortDefaultFirst(parseCUPSPrinters(devices, def)), nil
}

func printFor(ctx context.Context, goos, printer, title, text string) error {
	printers, err := printersFor(ctx, goos)
	if err != nil {
		return err
	}
	known := false
	for _, p := range printers {
		if p.Name == printer {
			known = true
			break
		}
	}
	if !known {
		return ErrNoSuchPrinter
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err = runCommand(ctx, printCommand(goos, printer, title, text))
	return err
}

// printCommand builds the command that prints text: the printer as its own
// argument (or, on Windows, in the environment, never in the script) and
// the text only on standard input.
func printCommand(goos, printer, title, text string) command {
	if goos == "windows" {
		return command{
			path:  powershellPath(),
			args:  []string{"-NoProfile", "-NonInteractive", "-Command", `$input | Out-Printer -Name $env:GOPMGR_PRINTER`},
			env:   []string{"GOPMGR_PRINTER=" + printer},
			stdin: text,
		}
	}
	return command{
		path:  lpPath,
		args:  []string{"-d", printer, "-t", title},
		env:   []string{"LC_ALL=C"},
		stdin: text,
	}
}

// parseCUPSPrinters reads `lpstat -v` ("device for NAME: URI") and
// `lpstat -d` ("system default destination: NAME"), leaving out printers
// that write to a file.
func parseCUPSPrinters(devices, def string) []Printer {
	defName := ""
	if name, ok := strings.CutPrefix(strings.TrimSpace(def), "system default destination: "); ok {
		defName = strings.TrimSpace(name)
	}
	var out []Printer
	for line := range strings.SplitSeq(devices, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "device for ")
		if !ok {
			continue
		}
		name, uri, ok := strings.Cut(rest, ": ")
		if !ok || name == "" {
			continue
		}
		conn, ok := cupsConnection(strings.TrimSpace(uri))
		if !ok {
			continue
		}
		out = append(out, Printer{Name: name, Connection: conn, Default: name == defName})
	}
	return out
}

// cupsConnection classifies a CUPS device URI; false means it writes to a
// file and is not offered.
func cupsConnection(uri string) (Connection, bool) {
	u, err := url.Parse(uri)
	scheme := uri
	if err == nil && u.Scheme != "" {
		scheme = u.Scheme
	} else if i := strings.Index(uri, ":"); i > 0 {
		scheme = uri[:i]
	}
	scheme = strings.ToLower(scheme)
	switch {
	case scheme == "file" || scheme == "hpfax" || strings.Contains(scheme, "pdf") || strings.HasPrefix(uri, "/dev/null"):
		return "", false
	case scheme == "usb" || scheme == "ippusb" || scheme == "parallel" || scheme == "serial":
		return Local, true
	case scheme == "hp" && strings.Contains(uri, "/usb/"):
		return Local, true
	default:
		return Network, true
	}
}

// parseWindowsPrinters reads tab-separated Name, PortName, Default, Network
// lines, leaving out printers that save to a file or are not printers.
func parseWindowsPrinters(out string) []Printer {
	var printers []Printer
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) != 4 || fields[0] == "" {
			continue
		}
		name, port := fields[0], strings.ToUpper(fields[1])
		if windowsSavesToFile(name, port) {
			continue
		}
		conn := Local
		if strings.EqualFold(fields[3], "True") || strings.HasPrefix(port, `\\`) ||
			strings.HasPrefix(port, "WSD") || strings.HasPrefix(port, "IP_") || strings.HasPrefix(port, "HTTP") {
			conn = Network
		}
		printers = append(printers, Printer{Name: name, Connection: conn, Default: strings.EqualFold(fields[2], "True")})
	}
	return printers
}

func windowsSavesToFile(name, port string) bool {
	switch port {
	case "PORTPROMPT:", "FILE:", "NUL:", "XPSPORT:", "SHRFAX:":
		return true
	}
	lower := strings.ToLower(name)
	for _, s := range []string{"pdf", "xps", "onenote", "fax"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return strings.Contains(port, "ONENOTE")
}

// sortDefaultFirst orders printers so the first is the safest choice: the
// default if it is connected to this computer, then other connected ones,
// then the default on the network, then other network printers, keeping
// the listing's order otherwise.
func sortDefaultFirst(printers []Printer) []Printer {
	rank := func(p Printer) int {
		r := 2
		if p.Connection == Local {
			r = 0
		}
		if !p.Default {
			r++
		}
		return r
	}
	out := make([]Printer, 0, len(printers))
	for r := 0; r <= 3; r++ {
		for _, p := range printers {
			if rank(p) == r {
				out = append(out, p)
			}
		}
	}
	return out
}
