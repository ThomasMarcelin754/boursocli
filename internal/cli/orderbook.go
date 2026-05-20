package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursocli/internal/out"
)

func newOrderbookCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "orderbook",
		Short: "Carnet d'ordres d'un instrument (Bearer _public_/feed/instrument/orderbook)",
	}
	var symbol string
	c.Flags().StringVar(&symbol, "symbol", "", "symbole Boursorama (ex: 1rPENGI)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if symbol == "" && len(args) > 0 {
			symbol = args[0]
		}
		if symbol == "" {
			return out.Fail(fmt.Errorf("--symbol requis (ex: 1rPENGI)"))
		}
		ctx := cmd.Context()
		cl, _, _, err := session(ctx)
		if err != nil {
			return out.Fail(err)
		}
		body, status, err := cl.PublicAPI(ctx, "_public_/feed/instrument/orderbook/"+symbol)
		if err != nil {
			return out.Fail(err)
		}
		if status != 200 {
			return out.Fail(fmt.Errorf("orderbook %s → HTTP %d : %s", symbol, status, snippet(body)))
		}
		if out.Format != "table" {
			return out.Raw(body)
		}
		var ob struct {
			Lines []struct {
				BidNb   int     `json:"bidNb"`
				BidSize int     `json:"bidSize"`
				Bid     float64 `json:"bid"`
				AskNb   int     `json:"askNb"`
				AskSize int     `json:"askSize"`
				Ask     float64 `json:"ask"`
			} `json:"lines"`
		}
		if err := json.Unmarshal(body, &ob); err != nil {
			return out.Fail(fmt.Errorf("décodage orderbook : %w", err))
		}
		t := out.Table{Cols: []string{"bidNb", "bidSize", "bid", "ask", "askSize", "askNb"}}
		for _, l := range ob.Lines {
			t.Rows = append(t.Rows, []string{
				fmt.Sprint(l.BidNb), fmt.Sprint(l.BidSize), fmt.Sprintf("%.2f", l.Bid),
				fmt.Sprintf("%.2f", l.Ask), fmt.Sprint(l.AskSize), fmt.Sprint(l.AskNb),
			})
		}
		return out.Data(t)
	}
	return c
}
