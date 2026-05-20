package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

func newQuoteCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "quote",
		Short: "Cours temps réel + résumé + consensus analystes (Bearer _public_/feed/instrument/*)",
	}
	var symbol string
	c.Flags().StringVar(&symbol, "symbol", "", "symbole Boursorama (ex: 1rPENGI, 1rPSTMPA, 1rPAIR)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if symbol == "" && len(args) > 0 {
			symbol = args[0]
		}
		if symbol == "" {
			return out.Fail(fmt.Errorf("--symbol requis (ex: 1rPENGI pour ENGIE, 1rPSTMPA pour STMicro)"))
		}
		ctx := cmd.Context()
		cl, _, _, err := session(ctx)
		if err != nil {
			return out.Fail(err)
		}
		body, status, err := cl.PublicAPI(ctx, "_public_/feed/instrument/quote/"+symbol)
		if err != nil {
			return out.Fail(err)
		}
		if status != 200 {
			return out.Fail(fmt.Errorf("quote %s → HTTP %d : %s", symbol, status, snippet(body)))
		}

		if out.Format != "table" {
			var merged map[string]any
			_ = json.Unmarshal(body, &merged)
			if sb, st, e := cl.PublicAPI(ctx, "_public_/feed/instrument/quotesummary/"+symbol); e == nil && st == 200 {
				var qs map[string]any
				if json.Unmarshal(sb, &qs) == nil {
					merged["summary"] = qs
				}
			}
			if ab, st, e := cl.PublicAPI(ctx, "_public_/feed/analysis/analystsrecommendation/"+symbol); e == nil && st == 200 {
				var an map[string]any
				if json.Unmarshal(ab, &an) == nil {
					merged["analysis"] = an
				}
			}
			j, err := json.Marshal(merged)
			if err != nil {
				return err
			}
			return out.Raw(j)
		}

		var q struct {
			Symbol    string  `json:"symbol"`
			Label     string  `json:"label"`
			ISIN      string  `json:"isin"`
			Last      float64 `json:"last"`
			Variation float64 `json:"variation"`
			Currency  string  `json:"currency"`
		}
		if err := json.Unmarshal(body, &q); err != nil {
			return out.Fail(fmt.Errorf("décodage quote : %w", err))
		}
		t := out.Table{Cols: []string{"symbol", "label", "isin", "last", "variation", "currency"}}
		t.Rows = append(t.Rows, []string{
			q.Symbol, q.Label, q.ISIN,
			fmt.Sprintf("%.2f", q.Last),
			fmt.Sprintf("%.2f%%", q.Variation*100),
			q.Currency,
		})
		return out.Data(t)
	}
	return c
}
