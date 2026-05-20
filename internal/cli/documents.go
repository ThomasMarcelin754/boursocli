package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/htmlx"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

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
		case "ord", "pea":
			path = "/compte/" + a.urlKind() + "/" + a.AccountKey + "/documents"
		case "cav":
			path = "/compte/cav/" + a.AccountKey + "/releves"
		default:
			return out.Fail(fmt.Errorf("le type de compte %q n'a pas de page documents (utiliser un compte cav, ord ou pea)", a.urlKind()))
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
