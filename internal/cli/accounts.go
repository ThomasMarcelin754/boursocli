package cli

import (
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

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
