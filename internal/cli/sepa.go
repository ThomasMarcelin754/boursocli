package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

func newSepaCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sepa",
		Short: "Mandats de prélèvement SEPA actifs (Bearer bank/directtransfer/sepacreditorsauthorization)",
	}
	sel := addAccountFlag(c)
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		body, handled, err := getJSON(ctx, cl, "bank/directtransfer/sepacreditorsauthorization/filter/"+a.AccountKey)
		if err != nil {
			return out.Fail(err)
		}
		if handled {
			return nil
		}
		if out.Format != "table" {
			return out.Raw(body)
		}
		var mandates []struct {
			ICS          string `json:"ics"`
			ICSLabel     string `json:"icsLabel"`
			State        string `json:"state"`
			StateLabel   string `json:"stateLabel"`
			CreationDate string `json:"creationDate"`
		}
		if err := json.Unmarshal(body, &mandates); err != nil {
			return out.Fail(fmt.Errorf("décodage mandats SEPA : %w", err))
		}
		t := out.Table{Cols: []string{"ics", "créancier", "état", "création"}}
		for _, m := range mandates {
			t.Rows = append(t.Rows, []string{m.ICS, m.ICSLabel, m.StateLabel, m.CreationDate})
		}
		return out.Data(t)
	}
	return c
}
