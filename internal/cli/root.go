// Package cli wires the cobra command tree. Auth = chromecookies dual-domain
// (auto) → scrape bearer; data = Bearer JSON first, cookie-plane fallback.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/auth"
	"github.com/thomasmarcelin754/boursobank/internal/client"
	"github.com/thomasmarcelin754/boursobank/internal/config"
	"github.com/thomasmarcelin754/boursobank/internal/out"
	"github.com/thomasmarcelin754/boursobank/internal/version"
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
		Use:           "boursobank",
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
		newPositionsCmd(), newOrdOrdersCmd(), newOrdFiscaliteCmd(),
		newDocumentsCmd(), newOrdOstCmd(), newBudgetMovementsCmd(), newExportCmd(),
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

	if flagRefresh || cfg.Bearer == "" {
		if err := c.Bootstrap(ctx); err != nil {
			// Stale Chrome session: one auto re-extract attempt.
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
		_ = cfg.Save(cfgPath)
	} else {
		c.Bearer, c.UserHash = cfg.Bearer, cfg.UserHash
		// Opportunistic keep-warm: past ~half the bearer life, proactively
		// renew the session server-side so an idle-but-alive session doesn't
		// lapse between uses. Best-effort — a hard-dead session is still
		// caught reactively with the proper reconnect message (cannot beat a
		// hard server expiry; this only extends a still-alive session).
		if t, e := time.Parse(time.RFC3339, cfg.BearerSavedAt); e == nil && time.Since(t) > 12*time.Hour {
			out.Debugf("bearer âgé de ~%s — refresh proactif (keep-warm)", time.Since(t).Round(time.Hour))
			if err := c.Refresh(ctx); err == nil {
				cfg.BearerSavedAt = time.Now().UTC().Format(time.RFC3339)
				_ = cfg.Save(cfgPath)
			}
		}
	}
	return c, cfg, cfgPath, nil
}

func orDefault(s string) string {
	if s == "" {
		return "auto (multi-profil : profil bourso le plus frais)"
	}
	return s
}
