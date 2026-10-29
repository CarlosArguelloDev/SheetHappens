package printer

import (
	"bufio"
	"bytes"
	"context"
	"strings"
)

// State is the operational state of a CUPS queue.
type State string

const (
	StateIdle       State = "idle"
	StateProcessing State = "processing"
	StateStopped    State = "stopped"
	StateUnknown    State = "unknown"
)

// Printer describes a CUPS destination.
type Printer struct {
	Name      string `json:"name"`
	State     State  `json:"state"`
	Enabled   bool   `json:"enabled"`
	IsDefault bool   `json:"is_default"`
	// Reason holds the optional status message reported by CUPS
	// (e.g. "Paused", "Waiting for printer to become available.").
	Reason string `json:"reason,omitempty"`
}

// Inventory is the result of a printer discovery.
type Inventory struct {
	Printers []Printer `json:"printers"`
	// Default is the CUPS system default destination ("" if none).
	Default string `json:"default"`
}

// ListPrinters queries `lpstat -h HOST -p -d` and returns every registered
// queue with its state and the system default destination.
func (c *Client) ListPrinters(ctx context.Context) (Inventory, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	stdout, stderr, err := c.run(ctx, "lpstat", "-h", c.host, "-p", "-d")
	if err != nil {
		// lpstat exits non-zero when no queue exists yet; that is not a
		// connectivity failure, just an empty inventory.
		if isNoDestinations(stdout) || isNoDestinations(stderr) {
			return Inventory{Printers: []Printer{}}, nil
		}
		return Inventory{}, newError(CodeCUPSUnavailable, "failed to list CUPS printers", cmdErr(err, stderr))
	}
	return parseLpstat(stdout), nil
}

func isNoDestinations(b []byte) bool {
	return bytes.Contains(b, []byte("No destinations added"))
}

// parseLpstat parses the C-locale output of `lpstat -p -d`, e.g.:
//
//	printer HP_DeskJet_2300 is idle.  enabled since Tue 28 Oct 2026 09:00:00 PM CST
//	printer Brother now printing Brother-12.  enabled since ...
//	printer Epson disabled since Tue 28 Oct 2026 ... -
//		Paused
//	system default destination: HP_DeskJet_2300
func parseLpstat(out []byte) Inventory {
	inv := Inventory{Printers: []Printer{}}
	var current *Printer

	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		raw := sc.Text()
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(raw, "printer "):
			fields := strings.Fields(line)
			if len(fields) < 3 {
				current = nil
				continue
			}
			p := Printer{Name: fields[1], State: StateUnknown}
			rest := strings.Join(fields[2:], " ")
			switch {
			case strings.HasPrefix(rest, "disabled"):
				p.State, p.Enabled = StateStopped, false
			case strings.HasPrefix(rest, "is idle"):
				p.State, p.Enabled = StateIdle, true
			case strings.HasPrefix(rest, "now printing"):
				p.State, p.Enabled = StateProcessing, true
			}
			// A paused-but-listed queue reports "disabled" elsewhere in the line.
			if strings.Contains(rest, " disabled since") {
				p.State, p.Enabled = StateStopped, false
			}
			inv.Printers = append(inv.Printers, p)
			current = &inv.Printers[len(inv.Printers)-1]

		case strings.HasPrefix(line, "system default destination:"):
			inv.Default = strings.TrimSpace(strings.TrimPrefix(line, "system default destination:"))
			current = nil

		case strings.HasPrefix(line, "no system default destination"):
			current = nil

		case (strings.HasPrefix(raw, "\t") || strings.HasPrefix(raw, " ")) && current != nil:
			// Indented continuation line: status reason of the last printer.
			if current.Reason == "" {
				current.Reason = line
			} else {
				current.Reason += " " + line
			}
		}
	}

	for i := range inv.Printers {
		inv.Printers[i].IsDefault = inv.Printers[i].Name == inv.Default
	}
	return inv
}
