package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

func newProfileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "profile",
		Short: "Profil du titulaire (Bearer customer/profile/basic)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl, _, _, err := session(ctx)
			if err != nil {
				return out.Fail(err)
			}
			body, handled, err := getJSON(ctx, cl, "customer/profile/basic")
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
				FirstName    string `json:"firstName"`
				LastName     string `json:"lastName"`
				BirthName    string `json:"birthName"`
				BirthdayDate string `json:"birthdayDate"`
				Email        string `json:"email"`
				BirthCity    string `json:"birthCity"`
				Gender       string `json:"gender"`
			}
			if err := json.Unmarshal(body, &p); err != nil {
				return out.Fail(fmt.Errorf("décodage profil : %w", err))
			}
			t := out.Table{Cols: []string{"prénom", "nom", "naissance", "email", "ville naiss.", "genre"}}
			t.Rows = append(t.Rows, []string{
				p.FirstName, p.LastName, p.BirthdayDate, p.Email, p.BirthCity, p.Gender,
			})
			return out.Data(t)
		},
	}
	return c
}
