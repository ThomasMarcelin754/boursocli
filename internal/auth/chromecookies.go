// Package auth: dual-domain cookie extraction from the local Chrome profile
// (no manual paste) + dashboard bearer bootstrap. Secrets never logged.
package auth

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

//go:embed load.mjs
var loadScript []byte

// The two registrable domains BoursoBank spans. Banking works with the
// boursobank jar alone; securities/ORD/bourse REQUIRE the boursorama jar too
// (x-domain-authentification SSO sets it). Always extract+merge both.
var cookieTargets = []struct{ host, url string }{
	{"clients.boursobank.com", "https://clients.boursobank.com/"},
	{"clients.boursorama.com", "https://clients.boursorama.com/"},
}

type scriptIn struct {
	TargetURL     string `json:"target_url"`
	ChromeProfile string `json:"chrome_profile"`
	TimeoutMillis int    `json:"timeout_millis"`
}
type scriptOut struct {
	CookieHeader string `json:"cookie_header"`
	CookieCount  int    `json:"cookie_count"`
	Error        string `json:"error"`
}

// ExtractCookies runs the embedded Node helper once per domain and returns the
// per-host cookie map (to store in config.CookiesByHost). cacheDir holds the
// one-off npm install of chrome-cookies-secure.
func ExtractCookies(ctx context.Context, chromeProfile, cacheDir string, log io.Writer) (map[string]string, error) {
	if _, err := exec.LookPath("node"); err != nil {
		return nil, fmt.Errorf("node introuvable (requis pour chromecookies ; installer Node ou utiliser un override cookie via --config)")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return nil, fmt.Errorf("npm introuvable (requis pour l’amorçage chrome-cookies-secure)")
	}
	if err := ensureNpm(ctx, cacheDir, log); err != nil {
		return nil, err
	}
	scriptPath := filepath.Join(cacheDir, "load.mjs")
	if err := os.WriteFile(scriptPath, loadScript, 0o600); err != nil {
		return nil, err
	}
	res := make(map[string]string, len(cookieTargets))
	for _, t := range cookieTargets {
		hdr, err := runOne(ctx, cacheDir, scriptPath, chromeProfile, t.url, log)
		if err != nil {
			// boursorama jar may be absent if the user never visited bourse;
			// boursobank is mandatory.
			if t.host == "clients.boursobank.com" {
				return nil, fmt.Errorf("échec de l’extraction des cookies pour %s : %w (Chrome est-il connecté à BoursoBank ?)", t.host, err)
			}
			_, _ = fmt.Fprintf(orDiscard(log), "avert : aucun cookie pour %s (%v) — titres/ORD indisponibles tant que vous n.aurez pas visité l.espace bourse dans Chrome\n", t.host, err)
			continue
		}
		if hdr != "" {
			res[t.host] = hdr
		}
	}
	if res["clients.boursobank.com"] == "" {
		return nil, fmt.Errorf("aucun cookie de session BoursoBank dans le profil Chrome %q — se connecter d’abord à clients.boursobank.com dans Chrome", chromeProfile)
	}
	return res, nil
}

// MergedHeader concatenates both jars for a request to any *.boursobank.com /
// *.boursorama.com host (cookies are domain-scoped; sending both is harmless
// and required for the bourse plane).
func MergedHeader(byHost map[string]string) string {
	var buf bytes.Buffer
	for _, t := range cookieTargets {
		if v := byHost[t.host]; v != "" {
			if buf.Len() > 0 {
				buf.WriteString("; ")
			}
			buf.WriteString(v)
		}
	}
	return buf.String()
}

func ensureNpm(ctx context.Context, dir string, log io.Writer) error {
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "chrome-cookies-secure", "package.json")); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	pkg := `{"private":true,"type":"module","dependencies":{"chrome-cookies-secure":"3.0.0"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o600); err != nil {
		return err
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(c, "npm", "install", "--silent", "--no-progress", "--no-fund", "--no-audit")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "npm_config_loglevel=error")
	cmd.Stdout = io.Discard
	cmd.Stderr = orDiscard(log)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install chrome-cookies-secure : %w", err)
	}
	return nil
}

func runOne(ctx context.Context, cacheDir, scriptPath, profile, targetURL string, log io.Writer) (string, error) {
	tmp, err := os.MkdirTemp("", "bb-ckout-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	outPath := filepath.Join(tmp, "out.json")
	in, err := json.Marshal(scriptIn{TargetURL: targetURL, ChromeProfile: profile, TimeoutMillis: 8000})
	if err != nil {
		return "", err
	}
	c, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	//nolint:gosec // scriptPath is our own go:embed'd load.mjs written to a private temp dir — not user input
	cmd := exec.CommandContext(c, "node", scriptPath)
	cmd.Dir = cacheDir
	cmd.Env = append(os.Environ(), "BOURSOBANK_OUTPUT_PATH="+outPath, "npm_config_loglevel=error")
	cmd.Stdin = bytes.NewReader(in)
	cmd.Stdout = io.Discard
	cmd.Stderr = orDiscard(log)
	runErr := cmd.Run()
	b, readErr := os.ReadFile(outPath) //nolint:gosec // outPath is our own file in a private os.MkdirTemp dir
	if readErr != nil {
		if runErr != nil {
			return "", runErr
		}
		return "", readErr
	}
	var o scriptOut
	if err := json.Unmarshal(b, &o); err != nil {
		return "", err
	}
	if o.Error != "" {
		return "", fmt.Errorf("%s", o.Error)
	}
	return o.CookieHeader, nil
}

func orDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
