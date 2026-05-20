package cli

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/htmlx"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

func newOrdMouvementsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ord-mouvements",
		Short: "Historique des mouvements comptabilisés ORD/PEA (plan cookie, table legacy)",
	}
	sel := addAccountFlag(c)
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		kind := a.urlKind()
		if kind != "ord" && kind != "pea" {
			return out.Fail(fmt.Errorf("le compte %s est de type %q, pas 'ord' ni 'pea'", a.AccountKey, kind))
		}
		path := fmt.Sprintf("/compte/%s/%s/mouvements", kind, a.AccountKey)
		doc, err := getHTML(ctx, cl, path)
		if err != nil {
			return out.Fail(err)
		}
		tbl, err := doc.ExtractOneTable("table.trading-operations__table")
		if err != nil {
			return out.Fail(err)
		}
		if len(tbl.Headers) != 8 {
			return out.Fail(fmt.Errorf("ord-mouvements : 8 colonnes attendues, %d obtenues : %v", len(tbl.Headers), tbl.Headers))
		}
		type mvt struct {
			DateOp    string `json:"dateOp"`
			DateVal   string `json:"dateVal"`
			Operation string `json:"operation"`
			Name      string `json:"name"`
			ISIN      string `json:"isin"`
			Montant   num    `json:"montant"`
			Quantite  string `json:"quantite"`
			Cours     num    `json:"cours"`
		}
		var mvts []mvt
		for i, row := range tbl.Rows {
			if len(row) != 8 {
				return out.Fail(fmt.Errorf("ord-mouvements ligne %d : 8 cellules attendues, %d obtenues (dérive de schéma)", i, len(row)))
			}
			name, isin := mouvNameISIN(row[3])
			mvts = append(mvts, mvt{
				DateOp:    htmlx.Clean(row[0].Text()),
				DateVal:   htmlx.Clean(row[1].Text()),
				Operation: htmlx.Clean(row[2].Text()),
				Name:      name,
				ISIN:      isin,
				Montant:   mknum(row[5].Text()),
				Quantite:  htmlx.Clean(row[6].Text()),
				Cours:     mknum(row[7].Text()),
			})
		}
		payload := map[string]any{"accountKey": a.AccountKey, "mouvements": mvts}
		if out.Format != "table" {
			return out.Data(payload)
		}
		t := out.Table{Cols: []string{"dateOp", "dateVal", "operation", "name", "isin", "montant", "qty", "cours"}}
		for _, m := range mvts {
			t.Rows = append(t.Rows, []string{m.DateOp, m.DateVal, m.Operation, m.Name, m.ISIN, m.Montant.Raw, m.Quantite, m.Cours.Raw})
		}
		return out.Data(t)
	}
	return c
}

func mouvNameISIN(cell *goquery.Selection) (string, string) {
	raw := htmlx.Clean(cell.Text())
	parts := strings.Fields(raw)
	for i := len(parts) - 1; i >= 0; i-- {
		if isISIN(parts[i]) {
			return strings.TrimSpace(strings.Join(parts[:i], " ")), parts[i]
		}
	}
	return raw, ""
}

func isISIN(s string) bool {
	if len(s) != 12 {
		return false
	}
	for i, c := range s {
		if i < 2 {
			if c < 'A' || c > 'Z' {
				return false
			}
		} else {
			if (c < '0' || c > '9') && (c < 'A' || c > 'Z') {
				return false
			}
		}
	}
	return true
}
