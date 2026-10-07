// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package printing

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fakeService replaces the print service: it answers listings with the
// given output and records every command, so nothing is printed.
type fakeService struct {
	devices, def, windows string
	failPrint             error
	ran                   []command
}

func useFake(t *testing.T, f *fakeService) {
	t.Helper()
	oldRun, oldStat := runCommand, statPath
	runCommand = func(_ context.Context, c command) (string, error) {
		f.ran = append(f.ran, c)
		switch {
		case c.path == lpstatPath && slices.Equal(c.args, []string{"-v"}):
			return f.devices, nil
		case c.path == lpstatPath && slices.Equal(c.args, []string{"-d"}):
			return f.def, nil
		case strings.HasSuffix(c.path, "powershell.exe") && c.stdin == "":
			return f.windows, nil
		}
		return "", f.failPrint
	}
	statPath = func(string) (os.FileInfo, error) { return nil, nil }
	t.Cleanup(func() { runCommand, statPath = oldRun, oldStat })
}

const cupsDevices = `device for Brother_HL_L2405W: ippusb://Brother%20HL-L2405W._ipp._tcp.local./?uuid=e3248000
device for Office: ipp://printer.example.com/ipp/print
device for Shared_From_Mac: dnssd://Brother%20%40%20Mac%20mini._ipp._tcp.local./?uuid=1
device for PDF: cups-pdf:/
device for ToFile: file:///tmp/out.prn
device for Laser_USB: usb://HP/LaserJet?serial=1
device for HP_Fax: hpfax:/usb/HP?serial=2
`

func TestCUPSPrintersLeaveOutFilesAndSayWhereEachIs(t *testing.T) {
	got := parseCUPSPrinters(cupsDevices, "system default destination: Office\n")
	want := []Printer{
		{Name: "Brother_HL_L2405W", Connection: Local},
		{Name: "Office", Connection: Network, Default: true},
		{Name: "Shared_From_Mac", Connection: Network},
		{Name: "Laser_USB", Connection: Local},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCUPSPrinters = %+v\nwant %+v", got, want)
	}
	if got := parseCUPSPrinters(cupsDevices, "no system default destination\n"); got[1].Default {
		t.Fatal("a printer was marked default with no default destination")
	}
}

func TestWindowsPrintersLeaveOutFilesAndSayWhereEachIs(t *testing.T) {
	out := "Microsoft Print to PDF\tPORTPROMPT:\tTrue\tFalse\r\n" +
		"Microsoft XPS Document Writer\tPORTPROMPT:\tFalse\tFalse\r\n" +
		"OneNote (Desktop)\tnul:\tFalse\tFalse\r\n" +
		"Fax\tSHRFAX:\tFalse\tFalse\r\n" +
		"Saved\tFILE:\tFalse\tFalse\r\n" +
		"Office Laser\tWSD-1234\tFalse\tFalse\r\n" +
		`\\server\Shared` + "\t" + `\\server\Shared` + "\tFalse\tTrue\r\n" +
		"Desk Laser\tUSB001\tFalse\tFalse\r\n" +
		"garbage line\r\n"
	got := parseWindowsPrinters(out)
	want := []Printer{
		{Name: "Office Laser", Connection: Network},
		{Name: `\\server\Shared`, Connection: Network},
		{Name: "Desk Laser", Connection: Local},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseWindowsPrinters = %+v\nwant %+v", got, want)
	}
}

// Connected printers come before network ones, so the first choice keeps
// the code on this computer; the default leads within each group.
func TestConnectedPrintersComeFirstThenTheDefault(t *testing.T) {
	order := func(in ...Printer) []string {
		var names []string
		for _, p := range sortDefaultFirst(in) {
			names = append(names, p.Name)
		}
		return names
	}
	if got := order(Printer{Name: "net"}, Printer{Name: "usb", Connection: Local}, Printer{Name: "netdef", Default: true}); !slices.Equal(got, []string{"usb", "netdef", "net"}) {
		t.Fatalf("with a network default: %v, want usb, netdef, net", got)
	}
	if got := order(Printer{Name: "usb2", Connection: Local}, Printer{Name: "net"}, Printer{Name: "usbdef", Connection: Local, Default: true}); !slices.Equal(got, []string{"usbdef", "usb2", "net"}) {
		t.Fatalf("with a connected default: %v, want usbdef, usb2, net", got)
	}
}

// PowerShell's `t is not an escape inside single quotes; the listing must
// join with a real tab or no line would parse.
func TestTheWindowsListingJoinsWithARealTab(t *testing.T) {
	if !strings.Contains(windowsListScript, "-join [char]9") || strings.Contains(windowsListScript, "`t") {
		t.Fatalf("windowsListScript = %q; want fields joined with [char]9", windowsListScript)
	}
}

// The text goes only on standard input: never in an argument, the title,
// or the environment.
func TestPrintSendsTheTextOnlyOnStandardInput(t *testing.T) {
	const secret = "ABCDEFGH-IJKLMNOP"
	for _, goos := range []string{"darwin", "linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			f := &fakeService{devices: cupsDevices, windows: "Desk Laser\tUSB001\tTrue\tFalse\n"}
			useFake(t, f)
			printer := "Laser_USB"
			if goos == "windows" {
				printer = "Desk Laser"
			}
			if err := printFor(context.Background(), goos, printer, "GoPMgr administrator recovery", "code: "+secret); err != nil {
				t.Fatalf("printFor: %v", err)
			}
			last := f.ran[len(f.ran)-1]
			if last.stdin != "code: "+secret {
				t.Fatalf("stdin = %q, want the text", last.stdin)
			}
			for _, s := range append(append([]string{last.path}, last.args...), last.env...) {
				if strings.Contains(s, secret) {
					t.Fatalf("the text appears in %q", s)
				}
			}
			if goos == "windows" {
				if !slices.Contains(last.env, "GOPMGR_PRINTER=Desk Laser") || strings.Contains(strings.Join(last.args, " "), "Desk Laser") {
					t.Fatalf("Windows command = %+v; want the printer only in the environment", last)
				}
			} else if last.path != lpPath || !slices.Equal(last.args, []string{"-d", "Laser_USB", "-t", "GoPMgr administrator recovery"}) {
				t.Fatalf("lp command = %+v", last)
			}
		})
	}
}

func TestPrintRefusesAPrinterNotInTheListing(t *testing.T) {
	f := &fakeService{devices: cupsDevices}
	useFake(t, f)
	for _, printer := range []string{"PDF", "ToFile", "-o", "Nope", ""} {
		before := len(f.ran)
		if err := printFor(context.Background(), "darwin", printer, "t", "x"); !errors.Is(err, ErrNoSuchPrinter) {
			t.Errorf("printFor(%q) = %v, want ErrNoSuchPrinter", printer, err)
		}
		for _, c := range f.ran[before:] {
			if c.path == lpPath {
				t.Errorf("lp ran for %q", printer)
			}
		}
	}
}

func TestPrintReportsTheServiceError(t *testing.T) {
	f := &fakeService{devices: cupsDevices, failPrint: errors.New("printing: lp: printer is offline")}
	useFake(t, f)
	if err := printFor(context.Background(), "linux", "Office", "t", "x"); err == nil || err.Error() != "printing: lp: printer is offline" {
		t.Fatalf("printFor = %v, want the service's error", err)
	}
}

func TestNoPrintServiceMeansUnsupported(t *testing.T) {
	useFake(t, &fakeService{})
	statPath = func(string) (os.FileInfo, error) { return nil, fs.ErrNotExist }
	if _, err := printersFor(context.Background(), "linux"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("printersFor without lpstat = %v, want ErrUnsupported", err)
	}
}

// The real runner reports the service's own message, here lp's refusal of
// a printer that does not exist; it prints nothing.
func TestTheRealRunnerReportsTheServiceMessage(t *testing.T) {
	if _, err := os.Stat(lpPath); err != nil {
		t.Skip("no lp on this computer")
	}
	_, err := runCommand(context.Background(), command{path: lpPath, args: []string{"-d", "GoPMgr_No_Such_Printer", "-t", "test"}, env: []string{"LC_ALL=C"}})
	if err == nil {
		t.Fatal("lp accepted a printer that does not exist")
	}
}
