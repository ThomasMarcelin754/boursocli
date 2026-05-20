// Package cli wires the cobra command tree. Auth = chromecookies dual-domain
// (auto) → scrape bearer; data = Bearer JSON first, cookie-plane fallback.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/auth"
	"github.com/thomasmarcelin754/boursocli/internal/client"
	"github.com/thomasmarcelin754/boursocli/internal/config"
	"github.com/thomasmarcelin754/boursocli/internal/out"
	"github.com/thomasmarcelin754/boursocli/internal/version"
)

var (
	flagConfig  string
	flagProfile string
	flagFormat  string
	flagQuiet   bool
	flagDebug   bool
	flagRefresh bool // force re-extract cookies + re-scrape bearer
)

func ExecuteContext(ctx context.Context) error {
	root := buildRoot()
	return root.ExecuteContext(ctx)
}

func buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "boursocli",
		Short:         "CLI agent-first pour un compte BoursoBank personnel (lecture + virement assisté).",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			out.Format, out.Quiet, out.Debug = flagFormat, flagQuiet, flagDebug
			return nil
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	pf := root.PersistentFlags()
	pf.StringVar(&flagConfig, "config", "", "chemin de config (défaut : dossier config de l’OS)")
	pf.StringVar(&flagProfile, "chrome-profile", "", "nom/chemin du profil Chrome (défaut : Default, ou config)")
	pf.StringVar(&flagFormat, "format", "json", "sortie : json (agent-first) | table")
	pf.BoolVar(&flagQuiet, "quiet", false, "supprime les diagnostics stderr")
	pf.BoolVar(&flagDebug, "debug", false, "diagnostics stderr verbeux")
	pf.BoolVar(&flagRefresh, "refresh", false, "force la ré-extraction des cookies + re-scrape du bearer")

	root.AddCommand(
		newConfigCmd(), newAccountsCmd(),
		newOperationsCmd(), newTransfersCmd(), newBudgetsCmd(), newIncidentsCmd(),
		newPositionsCmd(), newOrdOrdersCmd(), newOrdFiscaliteCmd(), newOrdMouvementsCmd(),
		newDocumentsCmd(), newOrdOstCmd(), newBudgetMovementsCmd(), newExportCmd(),
		newCardCmd(), newSepaCmd(), newQuoteCmd(),
		newOrderbookCmd(), newTopflopCmd(), newMessagesCmd(),
		newProfileCmd(), newRecipientsCmd(),
		newVersionCmd(),
	)
	return root
}

// session loads config, ensures dual-domain cookies + a fresh-enough bearer,
// and returns a ready client. Re-extracts from Chrome on demand / on staleness.
func session(ctx context.Context) (*client.Client, *config.Config, string, error) {
	cfgPath, err := config.Path(flagConfig)
	if err != nil {
		return nil, nil, "", err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, "", err
	}
	if flagProfile != "" {
		cfg.ChromeProfile = flagProfile
	}
	cacheDir := filepath.Join(filepath.Dir(cfgPath), "ck-cache")

	need := flagRefresh || cfg.CookiesByHost["clients.boursobank.com"] == ""
	if need {
		out.Logf("extraction des cookies BoursoBank depuis Chrome (profil %q, bi-domaine)…", orDefault(cfg.ChromeProfile))
		ck, err := auth.ExtractCookies(ctx, cfg.ChromeProfile, cacheDir, os.Stderr)
		if err != nil {
			return nil, nil, "", err
		}
		cfg.CookiesByHost = ck
	}
	c := client.New(auth.MergedHeader(cfg.CookiesByHost), cfg.HTTPUserAgent)

	needBootstrap := flagRefresh || cfg.Bearer == ""

	// steipete TokenLikelyExpired pattern: if the bearer JWT is near expiry
	// (or expired), the cookie session may still be alive (empirically proven:
	// cookie outlives the 24h JWT). Refresh the cookie session first (keep it
	// warm), then re-bootstrap a fresh bearer from the dashboard.
	if !needBootstrap && cfg.BearerLikelyExpired(2*time.Minute) {
		out.Debugf("bearer expiré ou imminent (exp %s) — refresh cookie + re-bootstrap", cfg.BearerExp)
		_ = c.Refresh(ctx) // best-effort: keep cookie session alive
		needBootstrap = true
	}

	// Proactive keep-warm: past ~half the bearer life, refresh the cookie
	// session so it doesn't lapse between CLI invocations. Does NOT re-bootstrap
	// the bearer (it's still valid); just extends the cookie session server-side.
	if !needBootstrap {
		c.Bearer, c.UserHash = cfg.Bearer, cfg.UserHash
		if t, e := time.Parse(time.RFC3339, cfg.BearerSavedAt); e == nil && time.Since(t) > 12*time.Hour {
			out.Debugf("bearer âgé de ~%s — refresh proactif (keep-warm cookie)", time.Since(t).Round(time.Hour))
			if err := c.Refresh(ctx); err == nil {
				cfg.BearerSavedAt = time.Now().UTC().Format(time.RFC3339)
				_ = cfg.Save(cfgPath)
			}
		}
		// A re-login in Chrome kills the old server session even though the
		// JWT exp is still in the future. Probe cheaply; if the bearer is
		// dead, re-extract cookies and re-bootstrap transparently.
		if err := c.Probe(ctx); err != nil {
			out.Debugf("bearer en config rejeté (%v) — re-bootstrap", err)
			needBootstrap = true
			ck, e2 := auth.ExtractCookies(ctx, cfg.ChromeProfile, cacheDir, os.Stderr)
			if e2 == nil {
				cfg.CookiesByHost = ck
				c = client.New(auth.MergedHeader(ck), cfg.HTTPUserAgent)
			}
		}
	}

	if needBootstrap {
		if err := c.Bootstrap(ctx); err != nil {
			out.Debugf("bootstrap failed (%v) — re-extracting cookies", err)
			ck, e2 := auth.ExtractCookies(ctx, cfg.ChromeProfile, cacheDir, os.Stderr)
			if e2 != nil {
				return nil, nil, "", err
			}
			cfg.CookiesByHost = ck
			c = client.New(auth.MergedHeader(ck), cfg.HTTPUserAgent)
			if err := c.Bootstrap(ctx); err != nil {
				return nil, nil, "", err
			}
		}
		cfg.Bearer, cfg.UserHash = c.Bearer, c.UserHash
		cfg.BearerSavedAt = time.Now().UTC().Format(time.RFC3339)
		if exp := c.BearerExp(); !exp.IsZero() {
			cfg.BearerExp = exp.Format(time.RFC3339)
		}
		_ = cfg.Save(cfgPath)
	}
	return c, cfg, cfgPath, nil
}

func orDefault(s string) string {
	if s == "" {
		return "auto (multi-profil : profil bourso le plus frais)"
	}
	return s
}
