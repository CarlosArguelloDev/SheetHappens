// Package config loads and validates the SheetHappens runtime configuration
// from environment variables (see .env.example at the repository root).
package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Defaults applied when a variable is unset or empty.
const (
	DefaultPort        = "8080"
	DefaultCUPSHost    = "localhost:631"
	DefaultMaxUploadMB = 20
	DefaultWebDir      = "./web"

	// MaxAllowedUploadMB is a hard ceiling to protect the host from
	// misconfiguration (MAX_UPLOAD_MB=99999).
	MaxAllowedUploadMB = 200
)

var (
	// hostRe accepts a hostname or IPv4 address with an optional port.
	// It deliberately rejects a leading '-' so the value can never be
	// interpreted as a command-line flag by lp/lpstat.
	hostRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.\-]{0,252}(:[0-9]{1,5})?$`)

	// printerNameRe mirrors the CUPS queue-name rules: printable characters
	// except space, '/', '#', and not starting with '-'.
	printerNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@\-]{0,126}$`)
)

// Config holds the validated server configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port string
	// CUPSHost is the CUPS server (host[:port]) passed to lp/lpstat via -h.
	CUPSHost string
	// DefaultPrinter is used when a print request does not name a printer.
	// Empty means "use the CUPS system default destination".
	DefaultPrinter string
	// MaxUploadMB is the maximum accepted document size in megabytes.
	MaxUploadMB int64
	// WebDir is the directory holding the static frontend assets.
	WebDir string
	// TempDir is where uploaded documents are spooled before printing.
	TempDir string
}

// MaxUploadBytes returns the upload limit in bytes.
func (c Config) MaxUploadBytes() int64 {
	return c.MaxUploadMB << 20
}

// Addr returns the listen address for http.Server.
func (c Config) Addr() string {
	return ":" + c.Port
}

// Load reads the configuration from the process environment.
func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

// LoadFrom reads the configuration using the provided lookup function,
// which makes the loader testable without touching the real environment.
func LoadFrom(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Port:           get("PORT", DefaultPort),
		CUPSHost:       get("CUPS_HOST", DefaultCUPSHost),
		DefaultPrinter: get("DEFAULT_PRINTER", ""),
		WebDir:         get("WEB_DIR", DefaultWebDir),
		TempDir:        get("TMP_DIR", os.TempDir()),
	}

	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("config: PORT must be an integer between 1 and 65535, got %q", cfg.Port)
	}

	if !hostRe.MatchString(cfg.CUPSHost) {
		return Config{}, fmt.Errorf("config: CUPS_HOST %q is not a valid host[:port]", cfg.CUPSHost)
	}

	if cfg.DefaultPrinter != "" && !printerNameRe.MatchString(cfg.DefaultPrinter) {
		return Config{}, fmt.Errorf("config: DEFAULT_PRINTER %q is not a valid CUPS queue name", cfg.DefaultPrinter)
	}

	maxMB := int64(DefaultMaxUploadMB)
	if raw := getenv("MAX_UPLOAD_MB"); strings.TrimSpace(raw) != "" {
		maxMB, err = strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || maxMB < 1 || maxMB > MaxAllowedUploadMB {
			return Config{}, fmt.Errorf("config: MAX_UPLOAD_MB must be an integer between 1 and %d, got %q", MaxAllowedUploadMB, raw)
		}
	}
	cfg.MaxUploadMB = maxMB

	return cfg, nil
}
