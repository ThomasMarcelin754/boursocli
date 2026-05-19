package cli

import (
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
