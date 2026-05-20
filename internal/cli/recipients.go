package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

func newRecipientsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "recipients",
		Short: "Bénéficiaires de virement (Bearer bank/cashtransfer/recipients)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl, _, _, err := session(ctx)
			if err != nil {
				return out.Fail(err)
			}
			body, handled, err := getJSON(ctx, cl, "bank/cashtransfer/recipients")
			if err != nil {
				return out.Fail(err)
			}
			if handled {
				return nil
			}
			if out.Format != "table" {
				return out.Raw(body)
			}
			var r struct {
				CustomerAccounts []struct {
					AccountKey    string `json:"accountKey"`
					AccountNumber string `json:"accountNumber"`
					HolderID      string `json:"holderIdentity"`
					Category      string `json:"categoryLabel"`
				} `json:"customerAccounts"`
				OtherAccounts []struct {
					AccountKey    string `json:"accountKey"`
					AccountNumber string `json:"accountNumber"`
					HolderID      string `json:"holderIdentity"`
					BenefID       string `json:"beneficiaryIdentity"`
					BIC           string `json:"bic"`
					Category      string `json:"categoryLabel"`
				} `json:"otherAccounts"`
			}
			if err := json.Unmarshal(body, &r); err != nil {
				return out.Fail(fmt.Errorf("décodage recipients : %w", err))
			}
			t := out.Table{Cols: []string{"type", "holder", "accountNumber", "bic", "category"}}
			for _, a := range r.CustomerAccounts {
				t.Rows = append(t.Rows, []string{"interne", firstNonEmpty(a.HolderID, "-"), a.AccountNumber, "", a.Category})
			}
			for _, a := range r.OtherAccounts {
				t.Rows = append(t.Rows, []string{"externe", firstNonEmpty(a.BenefID, a.HolderID), a.AccountNumber, a.BIC, a.Category})
			}
			return out.Data(t)
		},
	}
	return c
}
