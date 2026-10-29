package printer

import (
	"errors"
	"fmt"
)

// Stable, machine-readable error codes. The web UI maps them to localized
// (EN/ES) messages, so they must never change once released.
const (
	CodeInvalidPrinter     = "invalid_printer"
	CodePrinterNotFound    = "printer_not_found"
	CodePrinterStopped     = "printer_stopped"
	CodeNoDefaultPrinter   = "no_default_printer"
	CodeInvalidCopies      = "invalid_copies"
	CodeInvalidColorMode   = "invalid_color_mode"
	CodeInvalidPageRange   = "invalid_page_range"
	CodeInvalidSides       = "invalid_sides"
	CodeInvalidOrientation = "invalid_orientation"
	CodeUnsupportedType    = "unsupported_type"
	CodeEmptyFile          = "empty_file"
	CodeCUPSUnavailable    = "cups_unavailable"
	CodePrintFailed        = "print_failed"
	CodeInternal           = "internal_error"
)

// Error is a typed error carrying a stable Code for API consumers.
type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// IsValidation reports whether the error was caused by invalid client input
// (as opposed to a CUPS/infrastructure failure).
func (e *Error) IsValidation() bool {
	switch e.Code {
	case CodeInvalidPrinter, CodePrinterNotFound, CodePrinterStopped, CodeNoDefaultPrinter,
		CodeInvalidCopies, CodeInvalidColorMode, CodeInvalidPageRange,
		CodeInvalidSides, CodeInvalidOrientation, CodeUnsupportedType, CodeEmptyFile:
		return true
	}
	return false
}

func newError(code, msg string, err error) *Error {
	return &Error{Code: code, Message: msg, Err: err}
}

// AsError extracts a *Error from err, if present.
func AsError(err error) (*Error, bool) {
	var pe *Error
	ok := errors.As(err, &pe)
	return pe, ok
}
