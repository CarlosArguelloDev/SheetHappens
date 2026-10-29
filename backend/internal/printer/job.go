package printer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Allowed values for print options. They map 1:1 to IPP attributes.
const (
	ColorModeColor      = "color"
	ColorModeMonochrome = "monochrome"

	SidesOneSided          = "one-sided"
	SidesTwoSidedLongEdge  = "two-sided-long-edge"
	SidesTwoSidedShortEdge = "two-sided-short-edge"

	OrientationPortrait  = "portrait"
	OrientationLandscape = "landscape"

	MinCopies = 1
	MaxCopies = 99

	maxPageRangeLen = 64
)

// Allowed document MIME types (as returned by http.DetectContentType,
// without parameters) and the extension used for the spool file.
var allowedMIME = map[string]string{
	"application/pdf": ".pdf",
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"text/plain":      ".txt",
}

var (
	printerNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@\-]{0,126}$`)
	pageRangeRe   = regexp.MustCompile(`^[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*$`)
	jobIDRe       = regexp.MustCompile(`request id is (\S+)`)
)

// IsAllowedMIME reports whether a detected MIME type may be printed.
func IsAllowedMIME(mime string) bool {
	_, ok := allowedMIME[mime]
	return ok
}

// PrintRequest describes a print job. Document must already have passed
// magic-byte validation; MIMEType is the detected (not client-declared) type.
type PrintRequest struct {
	PrinterName string
	Copies      int
	ColorMode   string
	PageRange   string
	Sides       string
	Orientation string

	MIMEType string
	Document io.Reader
}

// PrintResult is returned after CUPS accepted the job.
type PrintResult struct {
	JobID   string `json:"job_id"`
	Printer string `json:"printer"`
}

// normalize applies defaults and validates every option. It returns the
// lp option arguments (excluding destination and file).
func (r *PrintRequest) normalize() ([]string, error) {
	r.PrinterName = strings.TrimSpace(r.PrinterName)
	r.ColorMode = strings.ToLower(strings.TrimSpace(r.ColorMode))
	r.PageRange = strings.ReplaceAll(strings.TrimSpace(r.PageRange), " ", "")
	r.Sides = strings.ToLower(strings.TrimSpace(r.Sides))
	r.Orientation = strings.ToLower(strings.TrimSpace(r.Orientation))

	if r.PrinterName != "" && !printerNameRe.MatchString(r.PrinterName) {
		return nil, newError(CodeInvalidPrinter, "printer name contains invalid characters", nil)
	}

	if r.Copies == 0 {
		r.Copies = MinCopies
	}
	if r.Copies < MinCopies || r.Copies > MaxCopies {
		return nil, newError(CodeInvalidCopies, fmt.Sprintf("copies must be between %d and %d", MinCopies, MaxCopies), nil)
	}

	if _, ok := allowedMIME[r.MIMEType]; !ok {
		return nil, newError(CodeUnsupportedType, "document type is not allowed", nil)
	}
	if r.Document == nil {
		return nil, newError(CodeEmptyFile, "document is empty", nil)
	}

	args := []string{"-n", strconv.Itoa(r.Copies)}

	switch r.ColorMode {
	case "":
	case ColorModeColor, ColorModeMonochrome:
		args = append(args, "-o", "print-color-mode="+r.ColorMode)
	default:
		return nil, newError(CodeInvalidColorMode, "color mode must be 'color' or 'monochrome'", nil)
	}

	if r.PageRange != "" {
		if err := validatePageRange(r.PageRange); err != nil {
			return nil, err
		}
		args = append(args, "-P", r.PageRange)
	}

	switch r.Sides {
	case "":
	case SidesOneSided, SidesTwoSidedLongEdge, SidesTwoSidedShortEdge:
		args = append(args, "-o", "sides="+r.Sides)
	default:
		return nil, newError(CodeInvalidSides, "unsupported duplex mode", nil)
	}

	switch r.Orientation {
	case "":
	case OrientationPortrait:
		args = append(args, "-o", "orientation-requested=3")
	case OrientationLandscape:
		args = append(args, "-o", "orientation-requested=4")
	default:
		return nil, newError(CodeInvalidOrientation, "orientation must be 'portrait' or 'landscape'", nil)
	}

	return args, nil
}

// validatePageRange checks syntax ("1-3,5,8-10") and semantic order.
func validatePageRange(pr string) error {
	invalid := newError(CodeInvalidPageRange, "page range must look like 1-3,5,8-10", nil)
	if len(pr) > maxPageRangeLen || !pageRangeRe.MatchString(pr) {
		return invalid
	}
	for _, part := range strings.Split(pr, ",") {
		bounds := strings.SplitN(part, "-", 2)
		start, err := strconv.Atoi(bounds[0])
		if err != nil || start < 1 {
			return invalid
		}
		if len(bounds) == 2 {
			end, err := strconv.Atoi(bounds[1])
			if err != nil || end < start {
				return invalid
			}
		}
	}
	return nil
}

// PrintJob validates the request, spools the document to a UUID-named file
// in the temp directory, submits it with `lp` and always removes the file.
func (c *Client) PrintJob(ctx context.Context, req PrintRequest) (PrintResult, error) {
	opts, err := req.normalize()
	if err != nil {
		return PrintResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Resolve the destination: explicit > configured default > CUPS default.
	dest := req.PrinterName
	if dest == "" {
		dest = c.defaultPrinter
	}
	inv, err := c.ListPrinters(ctx)
	if err != nil {
		return PrintResult{}, err
	}
	if dest == "" {
		if inv.Default == "" {
			return PrintResult{}, newError(CodeNoDefaultPrinter, "no printer selected and no CUPS default destination", nil)
		}
		dest = inv.Default
	}
	p, found := lookup(inv, dest)
	if !found {
		return PrintResult{}, newError(CodePrinterNotFound, fmt.Sprintf("printer %q is not registered in CUPS", dest), nil)
	}
	if !p.Enabled {
		return PrintResult{}, newError(CodePrinterStopped, fmt.Sprintf("printer %q is stopped", dest), nil)
	}

	id, err := newUUID()
	if err != nil {
		return PrintResult{}, newError(CodeInternal, "failed to generate job id", err)
	}

	path, err := c.spool(id, allowedMIME[req.MIMEType], req.Document)
	if err != nil {
		return PrintResult{}, err
	}
	defer os.Remove(path)

	args := []string{"-h", c.host, "-d", dest, "-t", "SheetHappens-" + id}
	args = append(args, opts...)
	args = append(args, "--", path)

	stdout, stderr, err := c.run(ctx, "lp", args...)
	if err != nil {
		return PrintResult{}, newError(CodePrintFailed, "CUPS rejected the print job", cmdErr(err, stderr))
	}

	result := PrintResult{Printer: dest}
	if m := jobIDRe.FindSubmatch(stdout); m != nil {
		result.JobID = string(m[1])
	}
	return result, nil
}

// spool writes the document to <tempDir>/<uuid><ext> with 0600 permissions.
// On any error the partial file is removed.
func (c *Client) spool(id, ext string, doc io.Reader) (string, error) {
	path := filepath.Join(c.tempDir, "sheethappens-"+id+ext)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", newError(CodeInternal, "failed to create spool file", err)
	}
	n, copyErr := io.Copy(f, doc)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(path)
		if copyErr == nil {
			copyErr = closeErr
		}
		return "", newError(CodeInternal, "failed to write spool file", copyErr)
	}
	if n == 0 {
		os.Remove(path)
		return "", newError(CodeEmptyFile, "document is empty", nil)
	}
	return path, nil
}

func lookup(inv Inventory, name string) (Printer, bool) {
	for _, p := range inv.Printers {
		if p.Name == name {
			return p, true
		}
	}
	return Printer{}, false
}

// newUUID returns a random RFC 4122 version 4 UUID.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}
