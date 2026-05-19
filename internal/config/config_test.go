package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathOverride(t *testing.T) {
	if p, _ := Path("/x/y.json"); p != "/x/y.json" {
		t.Fatalf("override ignored: %s", p)
	}
	if p, err := Path(""); err != nil || !strings.HasSuffix(p, filepath.Join("boursobank", "config.json")) {
		t.Fatalf("default path wrong: %s %v", p, err)
	}
}

func TestSaveAtomicAndLoad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "config.json")
	c := &Config{
		Bearer: "JWT", UserHash: "h", CookiesByHost: map[string]string{"a": "v"},
	}
	if err := c.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// file 0600, dir 0700, no leftover .tmp
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file perm = %o, want 600", fi.Mode().Perm())
	}
	di, _ := os.Stat(filepath.Dir(p))
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm = %o, want 700", di.Mode().Perm())
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatal(".tmp not cleaned (rename failed?)")
	}
	got, err := Load(p)
	if err != nil || got.Bearer != "JWT" || got.Version != Version {
		t.Fatalf("Load roundtrip: %+v %v", got, err)
	}
	// missing file → empty config, no error
	empty, err := Load(filepath.Join(dir, "nope.json"))
	if err != nil || empty.CookiesByHost == nil {
		t.Fatalf("Load(missing): %+v %v", empty, err)
	}
}

func TestRedactedHidesSecrets(t *testing.T) {
	c := &Config{Bearer: "supersecretjwt", UserHash: "abc", CookiesByHost: map[string]string{"h": "cookieval"}}
	r := c.Redacted()
	blob, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(blob)
	for _, leak := range []string{"supersecretjwt", "cookieval", "abc"} {
		if strings.Contains(s, leak) {
			t.Fatalf("Redacted leaked %q: %s", leak, s)
		}
	}
	if !strings.Contains(s, "***") {
		t.Fatalf("expected redaction markers: %s", s)
	}
}
