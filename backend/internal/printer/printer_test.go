package printer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleLpstat = `printer HP_DeskJet_2300 is idle.  enabled since Tue 28 Oct 2026 09:00:00 PM CST
printer Brother_HL now printing Brother_HL-12.  enabled since Tue 28 Oct 2026 09:05:00 PM CST
printer Epson_L3150 disabled since Tue 28 Oct 2026 09:10:00 PM CST -
	Paused
system default destination: HP_DeskJet_2300
`

func TestParseLpstat(t *testing.T) {
	inv := parseLpstat([]byte(sampleLpstat))
	if inv.Default != "HP_DeskJet_2300" {
		t.Fatalf("default = %q", inv.Default)
	}
	want := []Printer{
		{Name: "HP_DeskJet_2300", State: StateIdle, Enabled: true, IsDefault: true},
		{Name: "Brother_HL", State: StateProcessing, Enabled: true},
		{Name: "Epson_L3150", State: StateStopped, Enabled: false, Reason: "Paused"},
	}
	if len(inv.Printers) != len(want) {
		t.Fatalf("got %d printers, want %d: %+v", len(inv.Printers), len(want), inv.Printers)
	}
	for i, w := range want {
		if inv.Printers[i] != w {
			t.Errorf("printer %d = %+v, want %+v", i, inv.Printers[i], w)
		}
	}
}

func TestParseLpstatNoDefault(t *testing.T) {
	inv := parseLpstat([]byte("printer X is idle.  enabled since now\nno system default destination\n"))
	if inv.Default != "" || len(inv.Printers) != 1 || inv.Printers[0].IsDefault {
		t.Fatalf("unexpected inventory: %+v", inv)
	}
}

// fakeRunner records lp invocations and serves canned lpstat output.
type fakeRunner struct {
	lpstatOut string
	lpstatErr error
	lpOut     string
	lpErr     error
	lpArgs    []string
	spooled   string // content of the spool file at lp time
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) ([]byte, []byte, error) {
	switch name {
	case "lpstat":
		return []byte(f.lpstatOut), nil, f.lpstatErr
	case "lp":
		f.lpArgs = args
		if b, err := os.ReadFile(args[len(args)-1]); err == nil {
			f.spooled = string(b)
		}
		return []byte(f.lpOut), nil, f.lpErr
	}
	return nil, nil, errors.New("unexpected command " + name)
}

func newTestClient(t *testing.T, f *fakeRunner, opts ...Option) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	opts = append([]Option{WithRunner(f.run), WithTempDir(dir)}, opts...)
	c, err := NewClient("cups:631", opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c, dir
}

func assertDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("spool directory not cleaned up: %v", entries)
	}
}

func TestPrintJobSuccess(t *testing.T) {
	f := &fakeRunner{lpstatOut: sampleLpstat, lpOut: "request id is HP_DeskJet_2300-42 (1 file(s))\n"}
	c, dir := newTestClient(t, f)

	res, err := c.PrintJob(context.Background(), PrintRequest{
		PrinterName: "HP_DeskJet_2300",
		Copies:      2,
		ColorMode:   "monochrome",
		PageRange:   "1-3, 5",
		Sides:       "two-sided-long-edge",
		Orientation: "landscape",
		MIMEType:    "text/plain",
		Document:    strings.NewReader("hello printer"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.JobID != "HP_DeskJet_2300-42" || res.Printer != "HP_DeskJet_2300" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if f.spooled != "hello printer" {
		t.Fatalf("spooled content = %q", f.spooled)
	}

	got := strings.Join(f.lpArgs, " ")
	for _, want := range []string{
		"-h cups:631", "-d HP_DeskJet_2300", "-n 2", "-P 1-3,5",
		"-o print-color-mode=monochrome", "-o sides=two-sided-long-edge",
		"-o orientation-requested=4", " -- ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lp args %q missing %q", got, want)
		}
	}

	spool := f.lpArgs[len(f.lpArgs)-1]
	if filepath.Dir(spool) != dir || !strings.HasPrefix(filepath.Base(spool), "sheethappens-") || filepath.Ext(spool) != ".txt" {
		t.Fatalf("unexpected spool path %q", spool)
	}
	assertDirEmpty(t, dir)
}

func TestPrintJobUsesDefaults(t *testing.T) {
	f := &fakeRunner{lpstatOut: sampleLpstat, lpOut: "request id is Brother_HL-1 (1 file(s))\n"}

	// Configured default printer wins over the CUPS default.
	c, _ := newTestClient(t, f, WithDefaultPrinter("Brother_HL"))
	res, err := c.PrintJob(context.Background(), PrintRequest{MIMEType: "application/pdf", Document: strings.NewReader("%PDF-1.7")})
	if err != nil || res.Printer != "Brother_HL" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(strings.Join(f.lpArgs, " "), "-n 1") {
		t.Fatalf("copies default not applied: %v", f.lpArgs)
	}

	// Without a configured default, the CUPS system default is used.
	c2, _ := newTestClient(t, f)
	res, err = c2.PrintJob(context.Background(), PrintRequest{MIMEType: "image/png", Document: strings.NewReader("png")})
	if err != nil || res.Printer != "HP_DeskJet_2300" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestPrintJobCleansUpOnLpFailure(t *testing.T) {
	f := &fakeRunner{lpstatOut: sampleLpstat, lpErr: errors.New("exit status 1")}
	c, dir := newTestClient(t, f)

	_, err := c.PrintJob(context.Background(), PrintRequest{MIMEType: "text/plain", Document: strings.NewReader("x")})
	pe, ok := AsError(err)
	if !ok || pe.Code != CodePrintFailed {
		t.Fatalf("expected print_failed, got %v", err)
	}
	assertDirEmpty(t, dir)
}

func TestPrintJobValidation(t *testing.T) {
	base := func() PrintRequest {
		return PrintRequest{PrinterName: "HP_DeskJet_2300", MIMEType: "text/plain", Document: strings.NewReader("x")}
	}
	cases := map[string]struct {
		mutate func(*PrintRequest)
		code   string
	}{
		"flag as printer":  {func(r *PrintRequest) { r.PrinterName = "-oevil" }, CodeInvalidPrinter},
		"path as printer":  {func(r *PrintRequest) { r.PrinterName = "../../etc/passwd" }, CodeInvalidPrinter},
		"unknown printer":  {func(r *PrintRequest) { r.PrinterName = "Ghost" }, CodePrinterNotFound},
		"stopped printer":  {func(r *PrintRequest) { r.PrinterName = "Epson_L3150" }, CodePrinterStopped},
		"too many copies":  {func(r *PrintRequest) { r.Copies = 100 }, CodeInvalidCopies},
		"negative copies":  {func(r *PrintRequest) { r.Copies = -1 }, CodeInvalidCopies},
		"bad color":        {func(r *PrintRequest) { r.ColorMode = "sepia" }, CodeInvalidColorMode},
		"bad range syntax": {func(r *PrintRequest) { r.PageRange = "1-;rm -rf" }, CodeInvalidPageRange},
		"reversed range":   {func(r *PrintRequest) { r.PageRange = "5-2" }, CodeInvalidPageRange},
		"page zero":        {func(r *PrintRequest) { r.PageRange = "0" }, CodeInvalidPageRange},
		"bad sides":        {func(r *PrintRequest) { r.Sides = "three-sided" }, CodeInvalidSides},
		"bad orientation":  {func(r *PrintRequest) { r.Orientation = "diagonal" }, CodeInvalidOrientation},
		"disallowed mime":  {func(r *PrintRequest) { r.MIMEType = "application/zip" }, CodeUnsupportedType},
		"empty document":   {func(r *PrintRequest) { r.Document = strings.NewReader("") }, CodeEmptyFile},
		"nil document":     {func(r *PrintRequest) { r.Document = nil }, CodeEmptyFile},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakeRunner{lpstatOut: sampleLpstat, lpOut: "request id is X-1 (1 file(s))"}
			c, dir := newTestClient(t, f)
			req := base()
			tc.mutate(&req)
			_, err := c.PrintJob(context.Background(), req)
			pe, ok := AsError(err)
			if !ok || pe.Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
			if !pe.IsValidation() {
				t.Fatalf("%s should be a validation error", tc.code)
			}
			if f.lpArgs != nil {
				t.Fatalf("lp must not run on validation failure")
			}
			assertDirEmpty(t, dir)
		})
	}
}

func TestListPrintersEmpty(t *testing.T) {
	f := &fakeRunner{lpstatOut: "lpstat: No destinations added.\n", lpstatErr: errors.New("exit status 1")}
	c, _ := newTestClient(t, f)
	inv, err := c.ListPrinters(context.Background())
	if err != nil || len(inv.Printers) != 0 {
		t.Fatalf("inv=%+v err=%v", inv, err)
	}
}

func TestListPrintersUnavailable(t *testing.T) {
	f := &fakeRunner{lpstatErr: errors.New("connection refused")}
	c, _ := newTestClient(t, f)
	_, err := c.ListPrinters(context.Background())
	if pe, ok := AsError(err); !ok || pe.Code != CodeCUPSUnavailable {
		t.Fatalf("expected cups_unavailable, got %v", err)
	}
}

func TestNewUUID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := newUUID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 36 || id[14] != '4' || seen[id] {
			t.Fatalf("bad uuid %q", id)
		}
		seen[id] = true
	}
}

func TestNewClientRejectsBadHost(t *testing.T) {
	if _, err := NewClient("-h evil"); err == nil {
		t.Fatal("expected error")
	}
}
