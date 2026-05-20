package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

func newCardCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "card",
		Short: "Détails de la carte bancaire (Bearer bank/creditcard/parameterssummary)",
	}
	sel := addAccountFlag(c)
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		if a.urlKind() != "card" {
			return out.Fail(fmt.Errorf("le compte %s est de type %q, pas 'card'", a.AccountKey, a.urlKind()))
		}
		body, handled, err := getJSON(ctx, cl, "bank/creditcard/parameterssummary/"+a.AccountKey)
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
			CreditCard struct {
				Name           string `json:"name"`
				Label          string `json:"label"`
				Number         string `json:"number"`
				Holder         string `json:"holder"`
				ExpirationDate string `json:"expirationDate"`
				Situation      string `json:"situation"`
				IsActive       bool   `json:"isActive"`
				IsLocked       bool   `json:"isLocked"`
				HasNfc         bool   `json:"hasNfc"`
				IsPrime        bool   `json:"isPrime"`
				IsMetal        bool   `json:"isMetal"`
				Virtual        bool   `json:"virtual"`
			} `json:"creditCard"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return out.Fail(fmt.Errorf("décodage carte : %w", err))
		}
		cc := p.CreditCard
		t := out.Table{Cols: []string{"name", "number", "holder", "expiration", "situation", "active", "locked", "nfc", "prime"}}
		t.Rows = append(t.Rows, []string{
			firstNonEmpty(cc.Label, cc.Name), cc.Number, cc.Holder,
			cc.ExpirationDate, cc.Situation,
			fmt.Sprint(cc.IsActive), fmt.Sprint(cc.IsLocked),
			fmt.Sprint(cc.HasNfc), fmt.Sprint(cc.IsPrime),
		})
		return out.Data(t)
	}
	return c
}
