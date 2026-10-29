package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"sheethappens/internal/printer"
)

type fakeService struct {
	pingErr error
	inv     printer.Inventory
	got     *printer.PrintRequest
	body    []byte
	err     error
}

func (f *fakeService) Ping(context.Context) error { return f.pingErr }
func (f *fakeService) ListPrinters(context.Context) (printer.Inventory, error) {
	return f.inv, nil
}
func (f *fakeService) PrintJob(_ context.Context, req printer.PrintRequest) (printer.PrintResult, error) {
	f.got = &req
	f.body, _ = io.ReadAll(req.Document)
	if f.err != nil {
		return printer.PrintResult{}, f.err
	}
	return printer.PrintResult{JobID: "HP-7", Printer: req.PrinterName}, nil
}

func newTestHandler(svc PrinterService, max int64) *Handler {
	return NewHandler(svc, max, 2, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func multipartBody(t *testing.T, fields map[string]string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if content != nil {
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(content)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var e errorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
		t.Fatalf("invalid JSON body %q: %v", rr.Body.String(), err)
	}
	return e.Error
}

var pdfBytes = []byte("%PDF-1.7\n1 0 obj << /Type /Catalog >> endobj\n%%EOF")

func TestPrintAcceptsPDF(t *testing.T) {
	svc := &fakeService{}
	h := newTestHandler(svc, 1<<20)
	body, ct := multipartBody(t, map[string]string{
		"printer": "HP_DeskJet_2300", "copies": "3", "color_mode": "monochrome",
		"page_range": "1-2", "sides": "two-sided-long-edge", "orientation": "portrait",
	}, "../../etc/passwd; rm -rf.pdf", pdfBytes)

	req := httptest.NewRequest(http.MethodPost, "/api/print", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Print(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	g := svc.got
	if g.MIMEType != "application/pdf" || g.Copies != 3 || g.ColorMode != "monochrome" ||
		g.PageRange != "1-2" || g.Sides != "two-sided-long-edge" || g.Orientation != "portrait" {
		t.Fatalf("unexpected request: %+v", g)
	}
	if !bytes.Equal(svc.body, pdfBytes) {
		t.Fatalf("document was not rewound after sniffing")
	}
}

func TestPrintRejectsSpoofedType(t *testing.T) {
	svc := &fakeService{}
	h := newTestHandler(svc, 1<<20)
	// ZIP magic bytes disguised with a .pdf name.
	body, ct := multipartBody(t, nil, "invoice.pdf", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"))
	req := httptest.NewRequest(http.MethodPost, "/api/print", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Print(rr, req)

	if rr.Code != http.StatusUnsupportedMediaType || decodeError(t, rr) != printer.CodeUnsupportedType {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if svc.got != nil {
		t.Fatal("PrintJob must not be called")
	}
}

func TestPrintRejectsOversize(t *testing.T) {
	h := newTestHandler(&fakeService{}, 1024)
	big := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("A"), 4096)...)
	body, ct := multipartBody(t, nil, "big.pdf", big)
	req := httptest.NewRequest(http.MethodPost, "/api/print", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Print(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge || decodeError(t, rr) != CodeFileTooLarge {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPrintRequiresFile(t *testing.T) {
	h := newTestHandler(&fakeService{}, 1<<20)
	body, ct := multipartBody(t, map[string]string{"copies": "1"}, "", nil)
	req := httptest.NewRequest(http.MethodPost, "/api/print", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Print(rr, req)
	if rr.Code != http.StatusBadRequest || decodeError(t, rr) != CodeMissingFile {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPrintRejectsNonMultipart(t *testing.T) {
	h := newTestHandler(&fakeService{}, 1<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/print", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Print(rr, req)
	if rr.Code != http.StatusBadRequest || decodeError(t, rr) != CodeInvalidRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPrintMapsValidationErrors(t *testing.T) {
	svc := &fakeService{err: &printer.Error{Code: printer.CodePrinterNotFound, Message: "nope"}}
	h := newTestHandler(svc, 1<<20)
	body, ct := multipartBody(t, map[string]string{"printer": "Ghost"}, "a.txt", []byte("hello"))
	req := httptest.NewRequest(http.MethodPost, "/api/print", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	h.Print(rr, req)
	if rr.Code != http.StatusBadRequest || decodeError(t, rr) != printer.CodePrinterNotFound {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHealth(t *testing.T) {
	rr := httptest.NewRecorder()
	newTestHandler(&fakeService{}, 1).Health(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	svc := &fakeService{pingErr: &printer.Error{Code: printer.CodeCUPSUnavailable}}
	newTestHandler(svc, 1).Health(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestPrinters(t *testing.T) {
	svc := &fakeService{inv: printer.Inventory{
		Default:  "HP",
		Printers: []printer.Printer{{Name: "HP", State: printer.StateIdle, Enabled: true, IsDefault: true}},
	}}
	rr := httptest.NewRecorder()
	newTestHandler(svc, 1).Printers(rr, httptest.NewRequest(http.MethodGet, "/api/printers", nil))
	var inv printer.Inventory
	if err := json.Unmarshal(rr.Body.Bytes(), &inv); err != nil || inv.Default != "HP" || len(inv.Printers) != 1 {
		t.Fatalf("status=%d body=%s err=%v", rr.Code, rr.Body.String(), err)
	}
}
