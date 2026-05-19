package cli

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/htmlx"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

// The ORD tables use THREE different cell markups (verified — they are
// NOT all the same):
//   - positions : span.c-link__label    + span.c-table__mention      (own file)
//   - ost        : span.u-ellipsis       + span.u-color-big-stone
//   - fiscalite  : td.table__name strong a (name) + td span (ISIN), legacy

func fiscaliteNameISIN(cell *goquery.Selection) (string, string) {
	return htmlx.Clean(cell.Find("a").First().Text()),
		htmlx.Clean(cell.Find("span").First().Text())
}

func ostNameISIN(cell *goquery.Selection) (string, string) {
	return htmlx.Clean(cell.Find("span.u-ellipsis").First().Text()),
		htmlx.Clean(cell.Find("span.u-color-big-stone").First().Text())
}

// newOrdFiscaliteCmd: cookie-plane /compte/ord/<key>/fiscalite
// ?FiscalityFiltersType[fiscalYear]=YYYY. Schema:
// table.table.table--trading-operations, 5 td positional:
// [0] name+ISIN, [1] +/- réalisées €, [2] +/- latentes €, [3] Cessions €,
// [4] dernier mouvement DD/MM/YYYY. Totals in c-summary-account-wrapper.
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
		// exact param name FiscalityFiltersType[fiscalYear] (bracket-encoded);
		// ?millesime/?year/?selectedYear are IGNORED by the server.
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

// newDocumentsCmd: cookie-plane documents list — ORD => /compte/ord/<key>/
// documents, CAV => /compte/cav/<key>/releves. SAME schema for both:
// table.documents__table,
// 6 td: [0] type, [1] doc name, [2] account, [3] add date, [4] preview,
// [5] download (href in the cell, text empty).
func newDocumentsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "documents",
		Short: "Liste relevés/IFU/avis — documents ORD ou relevés CAV (même schéma)",
	}
	sel := addAccountFlag(c)
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		var path string
		switch a.urlKind() {
		case "ord":
			path = "/compte/ord/" + a.AccountKey + "/documents"
		case "cav":
			path = "/compte/cav/" + a.AccountKey + "/releves"
		default:
			return out.Fail(fmt.Errorf("le type de compte %q n’a pas de page documents (utiliser un compte cav ou ord)", a.urlKind()))
		}
		doc, err := getHTML(ctx, cl, path)
		if err != nil {
			return out.Fail(err)
		}
		tbl, err := doc.ExtractOneTable("table.documents__table")
		if err != nil {
			return out.Fail(err)
		}
		if len(tbl.Headers) != 6 {
			return out.Fail(fmt.Errorf("documents : 6 colonnes attendues, %d obtenues : %v", len(tbl.Headers), tbl.Headers))
		}
		type docrow struct {
			DocType     string `json:"docType"`
			Name        string `json:"name"`
			Account     string `json:"account"`
			AddedDate   string `json:"addedDate"`
			DownloadURL string `json:"downloadUrl"`
		}
		var rows []docrow
		for i, r := range tbl.Rows {
			if len(r) != 6 {
				return out.Fail(fmt.Errorf("documents ligne %d : 6 cellules attendues, %d obtenues (dérive de schéma)", i, len(r)))
			}
			href, _ := r[5].Find("a[href]").First().Attr("href")
			name := htmlx.Clean(r[1].Text())
			if name == "" {
				return out.Fail(fmt.Errorf("documents ligne %d : nom de document vide (dérive de schéma cellule 1)", i))
			}
			if href != "" && len(href) > 0 && href[0] == '/' {
				href = cookieBase + href
			}
			rows = append(rows, docrow{
				DocType: htmlx.Clean(r[0].Text()), Name: name,
				Account: htmlx.Clean(r[2].Text()), AddedDate: htmlx.Clean(r[3].Text()),
				DownloadURL: href,
			})
		}
		payload := map[string]any{"accountKey": a.AccountKey, "kind": a.urlKind(), "documents": rows}
		if out.Format != "table" {
			return out.Data(payload)
		}
		t := out.Table{Cols: []string{"type", "name", "account", "added", "download"}}
		for _, d := range rows {
			t.Rows = append(t.Rows, []string{d.DocType, d.Name, d.Account, d.AddedDate, d.DownloadURL})
		}
		return out.Data(t)
	}
	return c
}

// newOrdOstCmd: cookie-plane /compte/ord/<key>/ost → 302 →
// /compte/ord/<key>/operations-sur-titre/liste (Go follows same-domain).
// Schema: table.c-table.c-table--action, 4 cols:
// Valeur(name+ISIN inline) | Nature de l'opération | Date limite de réponse |
// État de l'instruction.
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
