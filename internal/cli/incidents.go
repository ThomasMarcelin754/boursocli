package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

// newIncidentsCmd: Bearer risk/incident/{list,history}/<accountKey>.
// {incidentCategories:{count,categories[],messages[]},lockedCards:{available,
// message,readOnly},clearancePlan:{status,available,title,description,
// actionLink},help:{title,blocks[]},actions[]}. history = same minus
// lockedCards/clearancePlan. 10100 (not held) handled by getJSON as skip.
func newIncidentsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "incidents",
		Short: "Liste des incidents bancaires (ou --history) (Bearer risk/incident/*)",
	}
	sel := addAccountFlag(c)
	var history bool
	c.Flags().BoolVar(&history, "history", false, "risk/incident/history au lieu de list")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		kind := "list"
		if history {
			kind = "history"
		}
		body, handled, err := getJSON(ctx, cl, "risk/incident/"+kind+"/"+a.AccountKey)
		if err != nil {
			return out.Fail(err)
		}
		if handled {
			return nil
		}
		if out.Format != "table" {
			return out.Raw(body)
		}
		var p struct {
			IncidentCategories struct {
				Count int `json:"count"`
			} `json:"incidentCategories"`
			LockedCards struct {
				Available bool   `json:"available"`
				Message   string `json:"message"`
			} `json:"lockedCards"`
			ClearancePlan struct {
				Status    string `json:"status"`
				Available bool   `json:"available"`
			} `json:"clearancePlan"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return out.Fail(fmt.Errorf("décodage des incidents pour le tableau : %w", err))
		}
		t := out.Table{Cols: []string{"incidentCount", "lockedCards", "clearancePlanStatus"}}
		t.Rows = append(t.Rows, []string{
			fmt.Sprint(p.IncidentCategories.Count),
			fmt.Sprint(p.LockedCards.Available),
			firstNonEmpty(p.ClearancePlan.Status, "-"),
		})
		return out.Data(t)
	}
	return c
}
