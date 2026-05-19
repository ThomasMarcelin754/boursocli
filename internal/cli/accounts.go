package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/config"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{Use: "config", Short: "Affiche/gère la config du CLI"}
	c.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Affiche la config (secrets masqués)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.Path(flagConfig)
			if err != nil {
				return out.Fail(err)
			}
			cfg, err := config.Load(p)
			if err != nil {
				return out.Fail(err)
			}
			return out.Data(cfg.Redacted())
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "set chrome_profile <nom-ou-chemin>",
		Short: "Épingle un profil Chrome dédié (stable) ; sinon auto-pick du plus frais",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "chrome_profile" {
				return out.Fail(fmt.Errorf("clé inconnue %q (supportée : chrome_profile)", args[0]))
			}
			p, err := config.Path(flagConfig)
			if err != nil {
				return out.Fail(err)
			}
			cfg, err := config.Load(p)
			if err != nil {
				return out.Fail(err)
			}
			cfg.ChromeProfile = args[1]
			if err := cfg.Save(p); err != nil {
				return out.Fail(err)
			}
			return out.Data(map[string]string{"chrome_profile": args[1]})
		},
	})
	return c
}

func newAccountsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "accounts",
		Short: "Liste les comptes + soldes (Bearer JSON : bank/account/accounts)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			_, accs, body, err := resolveAccounts(ctx)
			if err != nil {
				return out.Fail(err)
			}
			if out.Format == "table" {
				return out.Data(acctsTable(accs))
			}
			return out.Raw(body) // lossless: full 47-field payload, nothing dropped
		},
	}
}

func snippet(b []byte) string {
	if len(b) > 200 {
		return string(b[:200])
	}
	return string(b)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
