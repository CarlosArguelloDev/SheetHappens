// Package printer wraps the CUPS command-line clients (lpstat, lp) to discover
// printers and submit print jobs to a (possibly remote) CUPS scheduler.
//
// Security model:
//   - Commands are executed with exec.CommandContext and explicit argument
//     slices; no shell is ever involved.
//   - User input never reaches argv verbatim: every option is validated
//     against a strict allow-list or regular expression first.
//   - User-supplied file names are never used; documents are spooled to a
//     random UUID path and removed after submission.
package printer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"
)

// Runner executes an external command and returns its stdout and stderr.
// It exists so the CUPS interaction can be faked in tests.
type Runner func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)

// ExecRunner is the production Runner backed by os/exec.
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Force the C locale so lpstat/lp output is stable and parseable.
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// Client talks to a CUPS server through lpstat/lp.
type Client struct {
	host           string
	defaultPrinter string
	tempDir        string
	run            Runner
	timeout        time.Duration
}

// Option customizes a Client.
type Option func(*Client)

// WithRunner overrides the command runner (used in tests).
func WithRunner(r Runner) Option { return func(c *Client) { c.run = r } }

// WithTempDir sets the spool directory for uploaded documents.
func WithTempDir(dir string) Option { return func(c *Client) { c.tempDir = dir } }

// WithDefaultPrinter sets the queue used when a request names no printer.
func WithDefaultPrinter(name string) Option { return func(c *Client) { c.defaultPrinter = name } }

// WithTimeout sets the per-command execution timeout.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

var hostRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.\-]{0,252}(:[0-9]{1,5})?$`)

// NewClient creates a Client for the given CUPS host (host[:port]).
func NewClient(host string, opts ...Option) (*Client, error) {
	if !hostRe.MatchString(host) {
		return nil, fmt.Errorf("printer: invalid CUPS host %q", host)
	}
	c := &Client{
		host:    host,
		tempDir: os.TempDir(),
		run:     ExecRunner,
		timeout: 60 * time.Second,
	}
	for _, o := range opts {
		o(c)
	}
	if c.defaultPrinter != "" && !printerNameRe.MatchString(c.defaultPrinter) {
		return nil, fmt.Errorf("printer: invalid default printer %q", c.defaultPrinter)
	}
	return c, nil
}

// Host returns the configured CUPS server.
func (c *Client) Host() string { return c.host }

// Ping checks that the CUPS scheduler is reachable (`lpstat -h HOST -r`).
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	stdout, stderr, err := c.run(ctx, "lpstat", "-h", c.host, "-r")
	if err != nil {
		return newError(CodeCUPSUnavailable, "CUPS scheduler is unreachable", cmdErr(err, stderr))
	}
	if !bytes.Contains(stdout, []byte("scheduler is running")) {
		return newError(CodeCUPSUnavailable, "CUPS scheduler is not running", errors.New(string(bytes.TrimSpace(stdout))))
	}
	return nil
}

// cmdErr enriches an exec error with the command's stderr output.
func cmdErr(err error, stderr []byte) error {
	if msg := bytes.TrimSpace(stderr); len(msg) > 0 {
		return fmt.Errorf("%w: %s", err, msg)
	}
	return err
}
