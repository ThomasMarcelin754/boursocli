// Package config persists CLI state (versioned JSON, OS config dir, --config override).
// Secrets (cookies/bearer) live here but are redacted by `config show`. Never logged.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const Version = 1

// Config is the on-disk state. CookiesByHost natively models the dual-domain
// requirement: keys are "clients.boursobank.com" and "clients.boursorama.com".
type Config struct {
	Version       int               `json:"version"`
	ChromeProfile string            `json:"chrome_profile,omitempty"` // "Default", "Profile 1", or a path
	CookiesByHost map[string]string `json:"cookies_by_host,omitempty"`
	Bearer        string            `json:"bearer,omitempty"`    // scraped DEFAULT_API_BEARER (24h)
	UserHash      string            `json:"user_hash,omitempty"` // scraped USER_HASH
	HTTPUserAgent string            `json:"http_user_agent,omitempty"`
}

// Path returns the config file path: --config override, else $XDG/OS config dir.
func Path(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "boursobank", "config.json"), nil
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is our own config file (OS config dir or the user's explicit --config)
	if os.IsNotExist(err) {
		return &Config{Version: Version, CookiesByHost: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.CookiesByHost == nil {
		c.CookiesByHost = map[string]string{}
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	// 0700: this directory holds the session-secret config.json — owner-only.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	c.Version = Version
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	// Atomic write: a crash or a concurrent run
	// mid-write must never corrupt/truncate the secrets file. Write a 0600
	// temp then rename (rename is atomic on the same filesystem).
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil { // 0600: session secrets
		return err
	}
	return os.Rename(tmp, path)
}

// Redacted returns a copy safe to print (`config show`).
func (c *Config) Redacted() map[string]any {
	red := func(s string) string {
		if s == "" {
			return ""
		}
		return "*** (" + itoa(len(s)) + " chars)"
	}
	hosts := map[string]string{}
	for h, v := range c.CookiesByHost {
		hosts[h] = red(v)
	}
	return map[string]any{
		"version":         c.Version,
		"chrome_profile":  c.ChromeProfile,
		"cookies_by_host": hosts,
		"bearer":          red(c.Bearer),
		"user_hash":       red(c.UserHash), // account-linkable → redact (show nothing identifying)
		"http_user_agent": c.HTTPUserAgent,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
