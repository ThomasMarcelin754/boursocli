package cli

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/htmlx"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

func fiscaliteNameISIN(cell *goquery.Selection) (string, string) {
	return htmlx.Clean(cell.Find("a").First().Text()),
		htmlx.Clean(cell.Find("span").First().Text())
}

func newOrdFiscaliteCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ord-fiscalite",
		Short: "Plus/moins-values réalisées/latentes & historique fiscal ORD (plan cookie, par année fiscale)",
	}
	sel := addAccountFlag(c)
	var year string
	c.Flags().StringVar(&year, "year", "2026", "année fiscale (années possibles 2021–2026)")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		if a.urlKind() != "ord" {
			return out.Fail(fmt.Errorf("le compte %s est de type %q, pas 'ord'", a.AccountKey, a.urlKind()))
		}
		path := fmt.Sprintf("/compte/ord/%s/fiscalite?FiscalityFiltersType%%5BfiscalYear%%5D=%s", a.AccountKey, year)
		doc, err := getHTML(ctx, cl, path)
		if err != nil {
			return out.Fail(err)
		}
		tbl, err := doc.ExtractOneTable("table.table--trading-operations")
		if err != nil {
			return out.Fail(err)
		}
		if len(tbl.Headers) != 5 {
			return out.Fail(fmt.Errorf("ord-fiscalite : 5 colonnes attendues, %d obtenues : %v", len(tbl.Headers), tbl.Headers))
		}
		type line struct {
			Name         string `json:"name"`
			ISIN         string `json:"isin"`
			Realized     num    `json:"realized"`
			Unrealized   num    `json:"unrealized"`
			Cessions     num    `json:"cessions"`
			LastMovement string `json:"lastMovement"`
		}
		var lines []line
		for i, row := range tbl.Rows {
			if len(row) != 5 {
				return out.Fail(fmt.Errorf("ord-fiscalite ligne %d : 5 cellules attendues, %d obtenues (dérive de schéma)", i, len(row)))
			}
			n, isin := fiscaliteNameISIN(row[0])
			if n == "" && isin == "" {
				return out.Fail(fmt.Errorf("ord-fiscalite ligne %d : nom ET ISIN vides (dérive de schéma)", i))
			}
			lines = append(lines, line{
				Name: n, ISIN: isin,
				Realized:     mknum(row[1].Text()),
				Unrealized:   mknum(row[2].Text()),
				Cessions:     mknum(row[3].Text()),
				LastMovement: htmlx.Clean(row[4].Text()),
			})
		}
		payload := map[string]any{
			"accountKey": a.AccountKey, "fiscalYear": year,
			"lines":   lines,
			"summary": doc.SummaryByLabel(".c-summary-account-wrapper__item", ".c-databox__name"),
		}
		if out.Format != "table" {
			return out.Data(payload)
		}
		t := out.Table{Cols: []string{"name", "isin", "realized", "unrealized", "cessions", "lastMove"}}
		for _, l := range lines {
			t.Rows = append(t.Rows, []string{l.Name, l.ISIN, l.Realized.Raw, l.Unrealized.Raw, l.Cessions.Raw, l.LastMovement})
		}
		return out.Data(t)
	}
	return c
}
