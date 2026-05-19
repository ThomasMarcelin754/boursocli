// Package out: agent-first output. JSON to stdout by default (parse-friendly),
// logs/diagnostics to stderr, machine "ok envelope" for writes. `--format table`
// for humans. Never prints secrets.
package out

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

var (
	Format = "json" // "json" (default, agent-first) | "table"
	Quiet  bool
	Debug  bool
)

// Logf writes diagnostics to STDERR only (keeps stdout pure JSON for agents).
func Logf(format string, a ...any) {
	if Quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

func Debugf(format string, a ...any) {
	if Debug {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", a...)
	}
}

// Data prints a successful payload. JSON mode: raw JSON to stdout. Table mode:
// rows (if [][]string via headers) else pretty JSON.
func Data(v any) error {
	if Format == "table" {
		if t, ok := v.(Table); ok {
			return t.write(os.Stdout)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Raw passes an upstream JSON body straight to stdout (lossless, agent-first —
// no field is dropped by our structs). It validates the bytes are JSON first:
// an invalid body is a loud error, never silently printed.
func Raw(body []byte) error {
	if !json.Valid(body) {
		return fmt.Errorf("la réponse amont n’est pas du JSON valide (%d octets)", len(body))
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err := buf.WriteTo(os.Stdout)
	return err
}

// OK is the machine envelope for write/action ops.
func OK(action string, extra map[string]any) error {
	m := map[string]any{"ok": true, "action": action}
	for k, v := range extra {
		m[k] = v
	}
	return Data(m)
}

// Fail prints a structured error to stdout (so agents still get JSON) and
// returns a non-nil error so main exits 1.
func Fail(err error) error {
	//nolint:errchkjson // best-effort error print; we are already on the failure path and return err anyway
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "error": err.Error()})
	return err
}

// Table is an optional human view; JSON mode ignores it and dumps Rows.
type Table struct {
	Cols []string
	Rows [][]string
}

func (t Table) MarshalJSON() ([]byte, error) {
	out := make([]map[string]string, 0, len(t.Rows))
	for _, r := range t.Rows {
		m := map[string]string{}
		for i, c := range t.Cols {
			if i < len(r) {
				m[c] = r[i]
			}
		}
		out = append(out, m)
	}
	return json.Marshal(out)
}

func (t Table) write(w *os.File) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, strings.Join(t.Cols, "\t"))
	for _, r := range t.Rows {
		_, _ = fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
}
