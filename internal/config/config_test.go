package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathOverride(t *testing.T) {
	if p, _ := Path("/x/y.json"); p != "/x/y.json" {
		t.Fatalf("override ignored: %s", p)
	}
	if p, err := Path(""); err != nil || !strings.HasSuffix(p, filepath.Join("boursocli", "config.json")) {
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

func TestHasCookie(t *testing.T) {
	cases := []struct {
		hdr, name string
		want      bool
	}{
		{"rememberme=abc; sid=1", "rememberme", true},
		{"sid=1; RememberMe=xyz", "rememberme", true}, // case-insensitive name
		{"sid=1; other=2", "rememberme", false},
		{"", "rememberme", false},
		{"rememberme_x=1", "rememberme", false}, // must not prefix-match
		{"x=rememberme=1", "rememberme", false}, // value containing the name ≠ a cookie named it
		{"  rememberme = v ; a=b", "rememberme", true},
	}
	for _, c := range cases {
		if got := hasCookie(c.hdr, c.name); got != c.want {
			t.Fatalf("hasCookie(%q,%q)=%v want %v", c.hdr, c.name, got, c.want)
		}
	}
}

func TestBearerLikelyExpired(t *testing.T) {
	margin := 2 * time.Minute
	// No bearer → expired
	if !(&Config{}).BearerLikelyExpired(margin) {
		t.Fatal("empty bearer must be expired")
	}
	// Bearer with exp in the future → not expired
	future := time.Now().Add(12 * time.Hour).UTC().Format(time.RFC3339)
	c := &Config{Bearer: "jwt", BearerExp: future}
	if c.BearerLikelyExpired(margin) {
		t.Fatal("bearer with 12h remaining must not be expired")
	}
	// Bearer with exp in the past → expired
	past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	c2 := &Config{Bearer: "jwt", BearerExp: past}
	if !c2.BearerLikelyExpired(margin) {
		t.Fatal("bearer with exp in the past must be expired")
	}
	// Bearer within margin → expired
	soon := time.Now().Add(90 * time.Second).UTC().Format(time.RFC3339)
	c3 := &Config{Bearer: "jwt", BearerExp: soon}
	if !c3.BearerLikelyExpired(margin) {
		t.Fatal("bearer expiring within margin must be expired")
	}
	// No BearerExp but BearerSavedAt recent → not expired (fallback)
	c4 := &Config{Bearer: "jwt", BearerSavedAt: time.Now().UTC().Format(time.RFC3339)}
	if c4.BearerLikelyExpired(margin) {
		t.Fatal("recent BearerSavedAt fallback must not be expired")
	}
	// No BearerExp, BearerSavedAt old → expired (fallback)
	c5 := &Config{Bearer: "jwt", BearerSavedAt: time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)}
	if !c5.BearerLikelyExpired(margin) {
		t.Fatal("old BearerSavedAt fallback must be expired")
	}
}

func TestRedactedSessionAnchorsNoLeak(t *testing.T) {
	c := &Config{
		Bearer:        "supersecretjwt",
		CookiesByHost: map[string]string{"clients.boursobank.com": "rememberme=SECRETVAL; sid=1"},
	}
	blob, err := json.Marshal(c.Redacted())
	if err != nil {
		t.Fatal(err)
	}
	s := string(blob)
	if strings.Contains(s, "SECRETVAL") || strings.Contains(s, "supersecretjwt") {
		t.Fatalf("session_anchors leaked a value: %s", s)
	}
	if !strings.Contains(s, `"rememberme_by_host"`) || !strings.Contains(s, `"bearer_present":true`) {
		t.Fatalf("expected non-secret anchors present: %s", s)
	}
}
