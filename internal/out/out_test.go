package out

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestDataJSONAndTableAndOK(t *testing.T) {
	Format = "json"
	out := captureStdout(t, func() { _ = Data(map[string]any{"a": 1}) })
	if !strings.Contains(out, `"a": 1`) {
		t.Fatalf("Data json: %q", out)
	}
	Format = "table"
	out = captureStdout(t, func() {
		_ = Data(Table{Cols: []string{"k"}, Rows: [][]string{{"v"}}})
	})
	if !strings.Contains(out, "k") || !strings.Contains(out, "v") {
		t.Fatalf("Data table: %q", out)
	}
	Format = "json"
	out = captureStdout(t, func() { _ = OK("export", map[string]any{"bytes": 7}) })
	var env map[string]any
	if json.Unmarshal([]byte(out), &env) != nil || env["ok"] != true || env["action"] != "export" {
		t.Fatalf("OK envelope wrong: %q", out)
	}
}

func TestTableMarshalJSON(t *testing.T) {
	tb := Table{Cols: []string{"a", "b"}, Rows: [][]string{{"1", "2"}, {"3"}}}
	b, err := json.Marshal(tb)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0]["a"] != "1" || got[0]["b"] != "2" {
		t.Fatalf("row0 wrong: %v", got)
	}
	if _, ok := got[1]["b"]; ok { // short row → no "b" key
		t.Fatalf("short row should omit missing cell: %v", got[1])
	}
}

func TestRawRejectsInvalidJSON(t *testing.T) {
	if err := Raw([]byte("not json")); err == nil {
		t.Fatal("Raw must loudly reject non-JSON, not print it silently")
	}
	if err := Raw([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("Raw valid JSON: %v", err)
	}
}

func TestFailReturnsError(t *testing.T) {
	e := errSentinel("boom")
	if got := Fail(e); got == nil {
		t.Fatal("Fail must return a non-nil error so main exits 1")
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
