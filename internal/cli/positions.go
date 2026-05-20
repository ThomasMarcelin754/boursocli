package cli

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/htmlx"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

// num keeps the raw cell text AND its parsed value so nothing is lost and a
// parse miss is visible in the payload (never silently zero).
type num struct {
	Raw    string   `json:"raw"`
	Value  *float64 `json:"value"`
	Parsed bool     `json:"parsed"`
}

func mknum(s string) num {
	n := num{Raw: strings.TrimSpace(s)}
	if v, ok := htmlx.FRNumber(s); ok {
		n.Value, n.Parsed = &v, true
	}
	return n
}

// position mirrors the ORD positions table (10 cols, a "Dernier Mvt"
// column before Notification). Positional, mapped 0..9:
// [0]"" [1]Valeur [2]Quantité [3]Px.Revient [4]Cours [5]Montant
// [6]+/-Latentes [7]+/-% [8]"Dernier Mvt" [9]Notification.
type position struct {
	Name        string `json:"name"`
	ISIN        string `json:"isin"`
	Symbol      string `json:"symbol"` // may be empty (/cours/ with no code)
	Quantity    num    `json:"quantity"`
	PRU         num    `json:"pxRevient"`
	LastPrice   num    `json:"cours"`
	DayVarPct   num    `json:"coursDayVarPct"`
	Amount      num    `json:"montant"`
	UnrealPL    num    `json:"plLatentes"`
	UnrealPLPct num    `json:"plLatentesPct"`
	DernierMvt  string `json:"dernierMvt"`
}

func newPositionsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "positions",
		Short: "Positions du portefeuille ORD (HTML plan cookie, bi-domaine)",
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
		doc, err := getHTML(ctx, cl, "/compte/"+kind+"/"+a.AccountKey+"/positions")
		if err != nil {
			return out.Fail(err)
		}
		tbl, err := doc.ExtractOneTable("table.c-table.c-table--action")
		if err != nil {
			return out.Fail(err)
		}
		// 10 columns, positional. A different width = schema drift → fail
		// loud (never parse a shifted table silently).
		if len(tbl.Headers) != 10 {
			return out.Fail(fmt.Errorf("positions : 10 colonnes attendues (schéma positionnel), %d obtenues : %v", len(tbl.Headers), tbl.Headers))
		}
		var ps []position
		for i, row := range tbl.Rows {
			if len(row) != 10 {
				return out.Fail(fmt.Errorf("positions ligne %d : 10 cellules attendues, %d obtenues (dérive de schéma)", i, len(row)))
			}
			valeur := row[1]
			coursCell := row[4]
			lastPrice := htmlx.Clean(coursCell.Find("span.u-ellipsis").First().Text())
			dayVar := htmlx.Clean(coursCell.Find("span.u-color-big-stone").First().Text())
			p := position{
				Name:        htmlx.Clean(valeur.Find("span.c-link__label").First().Text()),
				ISIN:        htmlx.Clean(valeur.Find("span.c-table__mention").First().Text()),
				Symbol:      symbolFromCours(valeur),
				Quantity:    mknum(row[2].Find("span.u-ellipsis").First().Text()),
				PRU:         mknum(row[3].Text()),
				LastPrice:   mknum(lastPrice),
				DayVarPct:   mknum(dayVar),
				Amount:      mknum(row[5].Text()),
				UnrealPL:    mknum(row[6].Text()),
				UnrealPLPct: mknum(row[7].Text()),
				DernierMvt:  htmlx.Clean(row[8].Text()),
			}
			if p.Name == "" && p.ISIN == "" {
				return out.Fail(fmt.Errorf("positions ligne %d : nom ET ISIN vides (dérive de schéma cellule 1)", i))
			}
			ps = append(ps, p)
		}
		// Aggregate + cash live OUTSIDE the table (separate summary block).
		summary := doc.SummaryByLabel(".c-summary-account-wrapper__item", ".c-databox__name")
		payload := map[string]any{
			"accountKey": a.AccountKey,
			"positions":  ps,
			"summary":    summary, // label→value, verbatim (incl. dated cash line)
		}
		if out.Format != "table" {
			return out.Data(payload)
		}
		t := out.Table{Cols: []string{"name", "isin", "qty", "pru", "cours", "montant", "+/-lat", "+/-%"}}
		for _, p := range ps {
			t.Rows = append(t.Rows, []string{
				p.Name, p.ISIN, p.Quantity.Raw, p.PRU.Raw, p.LastPrice.Raw,
				p.Amount.Raw, p.UnrealPL.Raw, p.UnrealPLPct.Raw,
			})
		}
		return out.Data(t)
	}
	return c
}

// symbolFromCours pulls the Boursorama code from a /cours/<sym>/ href in the
// Valeur cell; empty is valid (some lines have a bare /cours/).
func symbolFromCours(cell *goquery.Selection) string {
	var sym string
	cell.Find("a[href]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		h, _ := a.Attr("href")
		if i := strings.Index(h, "/cours/"); i >= 0 {
			rest := strings.Trim(h[i+len("/cours/"):], "/")
			if rest != "" {
				sym = rest
				return false
			}
		}
		return true
	})
	return sym
}
