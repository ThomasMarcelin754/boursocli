package cli

import (
	"fmt"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/htmlx"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

// newBudgetMovementsCmd: cookie-plane PFM/budget movements.
// GET /budget/compte/<webid>/mouvements?movementSearch[fromDate|toDate]=
// dd/mm/YYYY&movementSearch[selectedAccounts][]=<webid>  (webid =
// pfmAccountKey). This page is NOT a table — it is
// <ul.list__movement> of <li.list-operation-item data-id> grouped by
// <li.list-operation-date-line>. Missing container = loud schema drift; an
// item with empty label AND amount = loud (never a silent half-row).
func newBudgetMovementsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "budget-movements",
		Short: "Mouvements PFM/budget (plan cookie <ul.list__movement> ; schéma div, mappé empiriquement)",
	}
	sel := addAccountFlag(c)
	var from, to string
	c.Flags().StringVar(&from, "from", "", "date de début jj/mm/AAAA (défaut : il y a 3 ans)")
	c.Flags().StringVar(&to, "to", "", "date de fin jj/mm/AAAA (défaut : aujourd’hui+40j)")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		webid := a.PfmAccountKey
		if webid == "" {
			return out.Fail(fmt.Errorf("le compte %s n’a pas de pfmAccountKey (webid budget) — non activé PFM", a.AccountKey))
		}
		now := time.Now()
		if from == "" {
			from = now.AddDate(-3, 0, 0).Format("02/01/2006")
		}
		if to == "" {
			to = now.AddDate(0, 0, 40).Format("02/01/2006")
		}
		q := url.Values{}
		q.Set("movementSearch[fromDate]", from)
		q.Set("movementSearch[toDate]", to)
		q.Add("movementSearch[selectedAccounts][]", webid)
		doc, err := getHTML(ctx, cl, "/budget/compte/"+webid+"/mouvements?"+q.Encode())
		if err != nil {
			return out.Fail(err)
		}

		ul := doc.Sel("ul.list__movement")
		if ul.Length() == 0 {
			return out.Fail(fmt.Errorf("budget-movements : pas de conteneur <ul.list__movement> (dérive de schéma — structure de page modifiée)"))
		}

		type mvt struct {
			Date      string `json:"date"`
			DataID    string `json:"dataId"`
			Label     string `json:"label"`
			LabelSub  string `json:"labelSub"`
			Category  string `json:"category"`
			Amount    string `json:"amount"`
			AmountNum num    `json:"amountValue"`
		}
		var rows []mvt
		curDate := ""
		var perr error
		ul.First().Children().Each(func(i int, li *goquery.Selection) {
			if perr != nil {
				return
			}
			switch {
			case li.HasClass("list-operation-date-line"):
				curDate = htmlx.Clean(li.Text())
			case li.HasClass("list-operation-item"):
				id, _ := li.Attr("data-id")
				label := htmlx.Clean(li.Find(".list-operation-item__label-name").First().Text())
				if label == "" {
					label = htmlx.Clean(li.Find(".list-operation-item__label").First().Text())
				}
				amount := htmlx.Clean(li.Find(".list-operation-item__amount").First().Text())
				if label == "" && amount == "" {
					perr = fmt.Errorf("budget-movements élément %d (date %q, id %q) : libellé ET montant vides — dérive de schéma", i, curDate, id)
					return
				}
				rows = append(rows, mvt{
					Date:      curDate,
					DataID:    id,
					Label:     label,
					LabelSub:  htmlx.Clean(li.Find(".list-operation-item__label-sub").First().Text()),
					Category:  htmlx.Clean(li.Find(".list-operation-item__category").First().Text()),
					Amount:    amount,
					AmountNum: mknum(amount),
				})
			}
		})
		if perr != nil {
			return out.Fail(perr)
		}
		payload := map[string]any{
			"accountKey": a.AccountKey, "webid": webid,
			"from": from, "to": to, "count": len(rows), "movements": rows,
		}
		if out.Format != "table" {
			return out.Data(payload)
		}
		t := out.Table{Cols: []string{"date", "label", "category", "amount"}}
		for _, m := range rows {
			t.Rows = append(t.Rows, []string{m.Date, m.Label, m.Category, m.Amount})
		}
		return out.Data(t)
	}
	return c
}
