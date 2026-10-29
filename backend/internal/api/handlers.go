// Package api implements the SheetHappens REST handlers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sheethappens/internal/printer"
)

// API-level error codes (in addition to the printer.Code* values).
const (
	CodeMissingFile    = "missing_file"
	CodeFileTooLarge   = "file_too_large"
	CodeInvalidRequest = "invalid_request"
	CodeBusy           = "server_busy"
)

// sniffLen is the number of bytes inspected by http.DetectContentType.
const sniffLen = 512

// multipartMemory is the portion of a multipart upload kept in RAM; the rest
// is buffered by net/http in temporary files removed by RemoveAll.
const multipartMemory = 8 << 20

// multipartOverhead accounts for boundaries and form fields on top of the
// document itself when enforcing the request body limit.
const multipartOverhead = 1 << 20

// PrinterService is the subset of *printer.Client used by the handlers.
type PrinterService interface {
	Ping(ctx context.Context) error
	ListPrinters(ctx context.Context) (printer.Inventory, error)
	PrintJob(ctx context.Context, req printer.PrintRequest) (printer.PrintResult, error)
}

// Handler serves the REST API.
type Handler struct {
	svc            PrinterService
	maxUploadBytes int64
	log            *slog.Logger
	// slots bounds the number of concurrent print submissions.
	slots chan struct{}
}

// NewHandler builds a Handler. maxConcurrent limits simultaneous print jobs.
func NewHandler(svc PrinterService, maxUploadBytes int64, maxConcurrent int, log *slog.Logger) *Handler {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Handler{
		svc:            svc,
		maxUploadBytes: maxUploadBytes,
		log:            log,
		slots:          make(chan struct{}, maxConcurrent),
	}
}

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: code, Message: msg})
}

// writePrinterError maps a printer.Error to an HTTP status without leaking
// internal command output to the client.
func (h *Handler) writePrinterError(w http.ResponseWriter, err error) {
	pe, ok := printer.AsError(err)
	if !ok {
		h.log.Error("unexpected error", "err", err)
		writeError(w, http.StatusInternalServerError, printer.CodeInternal, "internal server error")
		return
	}
	switch {
	case pe.IsValidation():
		status := http.StatusBadRequest
		if pe.Code == printer.CodePrinterStopped {
			status = http.StatusConflict
		}
		if pe.Code == printer.CodeUnsupportedType {
			status = http.StatusUnsupportedMediaType
		}
		writeError(w, status, pe.Code, pe.Message)
	case pe.Code == printer.CodeCUPSUnavailable || pe.Code == printer.CodePrintFailed:
		h.log.Error("cups failure", "code", pe.Code, "err", pe.Err)
		writeError(w, http.StatusBadGateway, pe.Code, pe.Message)
	default:
		h.log.Error("internal failure", "code", pe.Code, "err", pe.Err)
		writeError(w, http.StatusInternalServerError, pe.Code, "internal server error")
	}
}

// Health handles GET /api/health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	body := map[string]any{
		"status": "ok",
		"cups":   "reachable",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}
	if err := h.svc.Ping(ctx); err != nil {
		h.log.Warn("health check: CUPS unreachable", "err", err)
		body["status"] = "degraded"
		body["cups"] = "unreachable"
		writeJSON(w, http.StatusServiceUnavailable, body)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

// Printers handles GET /api/printers.
func (h *Handler) Printers(w http.ResponseWriter, r *http.Request) {
	inv, err := h.svc.ListPrinters(r.Context())
	if err != nil {
		h.writePrinterError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

// Print handles POST /api/print (multipart/form-data).
//
// Fields: file (required), printer, copies, color_mode, page_range, sides,
// orientation. The client-supplied file name and Content-Type are ignored:
// the type is detected from magic bytes and the spool name is a UUID.
func (h *Handler) Print(w http.ResponseWriter, r *http.Request) {
	ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || ct != "multipart/form-data" {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "expected multipart/form-data")
		return
	}

	// Enforce MAX_UPLOAD_MB on the raw body before parsing anything.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes+multipartOverhead)
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "file exceeds the upload limit")
			return
		}
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "malformed multipart form")
		return
	}
	// Remove any temp files net/http created while buffering the upload.
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingFile, "a file is required")
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		writeError(w, http.StatusBadRequest, printer.CodeEmptyFile, "document is empty")
		return
	}
	if header.Size > h.maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "file exceeds the upload limit")
		return
	}

	// Magic-byte detection on the first 512 bytes.
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, "unable to read upload")
		return
	}
	detected := detectMIME(head[:n])
	if !printer.IsAllowedMIME(detected) {
		writeError(w, http.StatusUnsupportedMediaType, printer.CodeUnsupportedType,
			"only PDF, PNG, JPEG and plain text files are accepted")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, printer.CodeInternal, "internal server error")
		return
	}

	copies := 0
	if raw := strings.TrimSpace(r.FormValue("copies")); raw != "" {
		copies, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, printer.CodeInvalidCopies, "copies must be a number")
			return
		}
	}

	req := printer.PrintRequest{
		PrinterName: r.FormValue("printer"),
		Copies:      copies,
		ColorMode:   r.FormValue("color_mode"),
		PageRange:   r.FormValue("page_range"),
		Sides:       r.FormValue("sides"),
		Orientation: r.FormValue("orientation"),
		MIMEType:    detected,
		Document:    file,
	}

	// Bound concurrent submissions to protect the host and CUPS.
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		writeError(w, http.StatusServiceUnavailable, CodeBusy, "the print server is busy, try again shortly")
		return
	}

	res, err := h.svc.PrintJob(r.Context(), req)
	if err != nil {
		h.writePrinterError(w, err)
		return
	}

	h.log.Info("print job submitted",
		"job_id", res.JobID, "printer", res.Printer, "mime", detected,
		"bytes", header.Size, "copies", req.Copies, "client", clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"job_id":  res.JobID,
		"printer": res.Printer,
		"mime":    detected,
	})
}

// detectMIME runs http.DetectContentType and strips parameters
// ("text/plain; charset=utf-8" -> "text/plain").
func detectMIME(b []byte) string {
	mt := http.DetectContentType(b)
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = mt[:i]
	}
	return strings.TrimSpace(mt)
}

// clientIP returns the visitor IP, preferring the header set by Cloudflare.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
