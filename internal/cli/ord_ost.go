package cli

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/htmlx"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

func ostNameISIN(cell *goquery.Selection) (string, string) {
	return htmlx.Clean(cell.Find("span.u-ellipsis").First().Text()),
		htmlx.Clean(cell.Find("span.u-color-big-stone").First().Text())
}

func newOrdOstCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ord-ost",
		Short: "Opérations sur titres (OST) ORD (plan cookie)",
	}
	sel := addAccountFlag(c)
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		if a.urlKind() != "ord" {
			return out.Fail(fmt.Errorf("le compte %s est de type %q, pas 'ord'", a.AccountKey, a.urlKind()))
		}
		doc, err := getHTML(ctx, cl, "/compte/ord/"+a.AccountKey+"/ost")
		if err != nil {
			return out.Fail(err)
		}
		tbl, err := doc.ExtractOneTable("table.c-table.c-table--action")
		if err != nil {
			return out.Fail(err)
		}
		if len(tbl.Headers) != 4 {
			return out.Fail(fmt.Errorf("ord-ost : 4 colonnes attendues, %d obtenues : %v", len(tbl.Headers), tbl.Headers))
		}
		type ost struct {
			Name     string `json:"name"`
			ISIN     string `json:"isin"`
			Nature   string `json:"nature"`
			Deadline string `json:"deadline"`
			State    string `json:"state"`
		}
		var list []ost
		for i, r := range tbl.Rows {
			if len(r) != 4 {
				return out.Fail(fmt.Errorf("ord-ost ligne %d : 4 cellules attendues, %d obtenues (dérive de schéma)", i, len(r)))
			}
			n, isin := ostNameISIN(r[0])
			nature := htmlx.Clean(r[1].Text())
			if n == "" && isin == "" && nature == "" {
				return out.Fail(fmt.Errorf("ord-ost ligne %d : nom, ISIN ET nature vides (dérive de schéma cellule 0/1)", i))
			}
			list = append(list, ost{
				Name: n, ISIN: isin,
				Nature: nature, Deadline: htmlx.Clean(r[2].Text()), State: htmlx.Clean(r[3].Text()),
			})
		}
		payload := map[string]any{"accountKey": a.AccountKey, "ost": list}
		if out.Format != "table" {
			return out.Data(payload)
		}
		t := out.Table{Cols: []string{"name", "isin", "nature", "deadline", "state"}}
		for _, o := range list {
			t.Rows = append(t.Rows, []string{o.Name, o.ISIN, o.Nature, o.Deadline, o.State})
		}
		return out.Data(t)
	}
	return c
}
