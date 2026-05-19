package cli

import (
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

// newOperationsCmd: Bearer bank/account/operations/<accountKey>.
// FIXED 30 most-recent,
// pagination:null (?page/?limit accepted but IGNORED). For full history use
// `export`. JSON output is the exhaustive payload; table is a lossy view.
func newOperationsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "operations",
		Short: "Opérations récentes (Bearer ; 30 plus récentes fixes, sans pagination — utiliser `export` pour l’historique)",
	}
	sel := addAccountFlag(c)
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		body, handled, err := getJSON(ctx, cl, "bank/account/operations/"+a.AccountKey)
		if err != nil {
			return out.Fail(err)
		}
		if handled {
			return nil
		}
		if out.Format != "table" {
			return out.Raw(body)
		}
		rows, err := decodeRows(body, "operations")
		if err != nil {
			return out.Fail(err)
		}
		t := out.Table{Cols: []string{"dates", "labels", "amount", "currency", "category", "status"}}
		for _, o := range rows {
			t.Rows = append(t.Rows, []string{
				rawStr(o["dates"]), rawStr(o["labels"]), rawStr(o["amount"]),
				rawStr(o["currency"]), rawStr(o["category"]), rawStr(o["status"]),
			})
		}
		return out.Data(t)
	}
	return c
}
