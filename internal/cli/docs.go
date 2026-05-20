package cli

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/htmlx"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

var docSections = map[string]string{
	"ifu":     "/documents/ifu",
	"bourse":  "/documents/bourse/",
	"releves": "/documents/releves",
	"banque":  "/documents/compte-bancaire",
}

func newDocsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "docs",
		Short: "Documents BoursoBank globaux : IFU, bourse (avis d'opérés), relevés, banque",
	}
	var section, year string
	c.Flags().StringVar(&section, "section", "bourse", "section : ifu | bourse | releves | banque")
	c.Flags().StringVar(&year, "year", "", "année (IFU uniquement, ex: 2025)")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, _, _, err := session(ctx)
		if err != nil {
			return out.Fail(err)
		}
		path, ok := docSections[section]
		if !ok {
			return out.Fail(fmt.Errorf("section %q inconnue (choix : ifu, bourse, releves, banque)", section))
		}
		if year != "" && section == "ifu" {
			path += "?documents_bank_ifu_type%5Bperiod%5D=" + year
		}
		doc, err := getHTML(ctx, cl, path)
		if err != nil {
			return out.Fail(err)
		}
		sel := doc.Sel("table.documents__table")
		if sel.Length() == 0 {
			return out.Data(map[string]any{"section": section, "count": 0, "documents": []any{}})
		}

		type docrow struct {
			Name        string `json:"name"`
			Detail      string `json:"detail,omitempty"`
			Date        string `json:"date,omitempty"`
			DownloadURL string `json:"downloadUrl,omitempty"`
		}
		var rows []docrow
		sel.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
			cells := tr.Find("td")
			if cells.Length() < 2 {
				return
			}

			dlURL := ""
			tr.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
				h, exists := a.Attr("href")
				if !exists {
					return
				}
				if strings.Contains(h, "telecharger") || strings.Contains(h, "download") {
					if h != "" && h[0] == '/' {
						dlURL = cookieBase + h
					} else {
						dlURL = h
					}
				}
			})

			r := docrow{DownloadURL: dlURL}
			switch {
			case cells.Length() >= 5:
				r.Name = htmlx.Clean(cells.Eq(1).Text())
				r.Detail = htmlx.Clean(cells.Eq(2).Text())
				r.Date = htmlx.Clean(cells.Eq(3).Text())
			default:
				r.Name = htmlx.Clean(cells.Eq(0).Text())
				r.Date = htmlx.Clean(cells.Eq(1).Text())
			}
			rows = append(rows, r)
		})

		payload := map[string]any{"section": section, "count": len(rows), "documents": rows}
		if year != "" {
			payload["year"] = year
		}
		if out.Format != "table" {
			return out.Data(payload)
		}
		t := out.Table{Cols: []string{"name", "detail", "date", "download"}}
		for _, d := range rows {
			t.Rows = append(t.Rows, []string{d.Name, d.Detail, d.Date, d.DownloadURL})
		}
		return out.Data(t)
	}
	return c
}
