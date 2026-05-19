package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

// newOrdOrdersCmd: Bearer trading/orderdetail/orders/<ordAccountKey>.
// trading/orderdetail full schema — REAL pagination (20/page):
// {count,trustedCount,orders:[…],pagination:{page,pageCount,itemsCount,
// itemPerPage,itemOffsetRange[2],paginator[]}}. We fetch page 1 (or --page N)
// and ALWAYS surface the pagination block so missing pages are never silent —
// itemsCount vs returned len is visible. Requires a bourse-elevated session
// (10006 ⇒ handled with an actionable message, not a wall).
func newOrdOrdersCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ord-orders",
		Short: "Historique des ordres titres (Bearer trading/orderdetail/orders, paginé 20/page)",
	}
	sel := addAccountFlag(c)
	var page int
	c.Flags().IntVar(&page, "page", 1, "numéro de page (20 ordres/page ; voir le bloc pagination dans la sortie)")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		if a.urlKind() != "ord" {
			return out.Fail(fmt.Errorf("le compte %s est de type %q, pas 'ord' — ord-orders est réservé à ORD", a.AccountKey, a.urlKind()))
		}
		resource := fmt.Sprintf("trading/orderdetail/orders/%s?page=%d", a.AccountKey, page)
		body, handled, err := getJSON(ctx, cl, resource)
		if err != nil {
			return out.Fail(err)
		}
		if handled {
			return nil
		}
		if out.Format != "table" {
			return out.Raw(body)
		}
		rows, err := decodeRows(body, "orders")
		if err != nil {
			return out.Fail(err)
		}
		t := out.Table{Cols: []string{"date", "name", "isin", "side", "qty", "execPrice", "type", "status"}}
		for _, o := range rows {
			t.Rows = append(t.Rows, []string{
				rawStr(o["creationDate"]), rawStr(o["name"]), rawStr(o["isin"]), rawStr(o["sideLabel"]),
				rawStr(o["quantity"]), rawStr(o["executionPrice"]), rawStr(o["typeLabel"]), rawStr(o["statusLabel"]),
			})
		}
		return out.Data(t)
	}
	return c
}
