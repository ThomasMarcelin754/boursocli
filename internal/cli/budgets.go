package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

// newBudgetsCmd: Bearer PFM budgets.
//   - list   : pfm/budget/budgets        → [{id,targetAmount:int,currentAmount:int,
//     incomingAmount:int,tag,accounts[],alerts,oldestHistoryDate}]
//   - --id X : pfm/budget/budget/<id>    (SINGULAR = full detail; adds
//     category{…},details{manuallyCreated})
//
// NOTE: pfm/operation/movements is the WRONG
// endpoint (always 406/1204) → for budget movements use `budget-movements`.
func newBudgetsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "budgets",
		Short: "Liste des budgets PFM, ou le détail d’un budget avec --id (Bearer pfm/budget/*)",
	}
	var id string
	c.Flags().StringVar(&id, "id", "", "id de budget → détail complet via pfm/budget/budget/<id> (singulier)")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, _, _, err := session(ctx)
		if err != nil {
			return out.Fail(err)
		}
		resource := "pfm/budget/budgets"
		if id != "" {
			resource = "pfm/budget/budget/" + id // singular = detail
		}
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
		if id != "" {
			var d struct {
				ID             string `json:"id"`
				TargetAmount   int    `json:"targetAmount"`
				CurrentAmount  int    `json:"currentAmount"`
				IncomingAmount int    `json:"incomingAmount"`
				Tag            string `json:"tag"`
				Category       struct {
					Name       string `json:"name"`
					GroupLabel string `json:"groupLabel"`
				} `json:"category"`
			}
			if err := json.Unmarshal(body, &d); err != nil {
				return out.Fail(fmt.Errorf("décodage du détail budget pour le tableau : %w", err))
			}
			t := out.Table{Cols: []string{"id", "tag", "category", "group", "target", "current", "incoming"}}
			t.Rows = append(t.Rows, []string{
				d.ID, d.Tag, d.Category.Name, d.Category.GroupLabel,
				fmt.Sprint(d.TargetAmount), fmt.Sprint(d.CurrentAmount), fmt.Sprint(d.IncomingAmount),
			})
			return out.Data(t)
		}
		var list []struct {
			ID             string `json:"id"`
			TargetAmount   int    `json:"targetAmount"`
			CurrentAmount  int    `json:"currentAmount"`
			IncomingAmount int    `json:"incomingAmount"`
			Tag            string `json:"tag"`
		}
		if err := json.Unmarshal(body, &list); err != nil {
			return out.Fail(fmt.Errorf("décodage des budgets pour le tableau : %w", err))
		}
		t := out.Table{Cols: []string{"id", "tag", "target", "current", "incoming"}}
		for _, b := range list {
			t.Rows = append(t.Rows, []string{
				b.ID, b.Tag, fmt.Sprint(b.TargetAmount),
				fmt.Sprint(b.CurrentAmount), fmt.Sprint(b.IncomingAmount),
			})
		}
		return out.Data(t)
	}
	return c
}
