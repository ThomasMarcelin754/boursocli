package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

func newTopflopCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "topflop",
		Short: "Top/Flop d'un indice (Bearer _public_/feed/topflop/marketplace)",
	}
	var index string
	c.Flags().StringVar(&index, "index", "1rPCAC", "symbole de l'indice (ex: 1rPCAC pour CAC40)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			index = args[0]
		}
		ctx := cmd.Context()
		cl, _, _, err := session(ctx)
		if err != nil {
			return out.Fail(err)
		}
		body, status, err := cl.PublicAPI(ctx, "_public_/feed/topflop/marketplace/"+index)
		if err != nil {
			return out.Fail(err)
		}
		if status != 200 {
			return out.Fail(fmt.Errorf("topflop %s → HTTP %d : %s", index, status, snippet(body)))
		}
		if out.Format != "table" {
			return out.Raw(body)
		}
		var tf struct {
			Instruments []struct {
				Symbol    string  `json:"symbol"`
				Label     string  `json:"label"`
				ISIN      string  `json:"isin"`
				Last      float64 `json:"last"`
				Variation float64 `json:"variation"`
				Currency  string  `json:"currency"`
			} `json:"instruments"`
		}
		if err := json.Unmarshal(body, &tf); err != nil {
			return out.Fail(fmt.Errorf("décodage topflop : %w", err))
		}
		t := out.Table{Cols: []string{"symbol", "label", "isin", "last", "variation", "ccy"}}
		for _, i := range tf.Instruments {
			t.Rows = append(t.Rows, []string{
				i.Symbol, i.Label, i.ISIN,
				fmt.Sprintf("%.2f", i.Last),
				fmt.Sprintf("%.2f%%", i.Variation*100),
				i.Currency,
			})
		}
		return out.Data(t)
	}
	return c
}
