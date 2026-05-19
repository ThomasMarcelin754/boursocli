package cli

import (
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

// newTransfersCmd: Bearer bank/cashtransfer/history?accountKey=<key>
// (accountKey is a QUERY param, NOT a path segment — history/<key> path-form
// → 500/10008). bank/cashtransfer/history
// full schema": {transfers:[{id,key,hash,amount,channel,label,originAccount,
// destinationAccount,repetition,repetitionType,state,stateLabel}],actions,pagination}.
func newTransfersCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "transfers",
		Short: "Historique des virements (Bearer bank/cashtransfer/history?accountKey=)",
	}
	sel := addAccountFlag(c)
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, a, err := resolvePicked(ctx, *sel)
		if err != nil {
			return out.Fail(err)
		}
		body, handled, err := getJSON(ctx, cl, "bank/cashtransfer/history?accountKey="+a.AccountKey)
		if err != nil {
			return out.Fail(err)
		}
		if handled {
			return nil
		}
		if out.Format != "table" {
			return out.Raw(body)
		}
		rows, err := decodeRows(body, "transfers")
		if err != nil {
			return out.Fail(err)
		}
		t := out.Table{Cols: []string{"amount", "label", "state", "channel", "repetition"}}
		for _, x := range rows {
			t.Rows = append(t.Rows, []string{
				rawStr(x["amount"]), rawStr(x["label"]),
				firstNonEmpty(rawStr(x["stateLabel"]), rawStr(x["state"])),
				rawStr(x["channel"]), rawStr(x["repetition"]),
			})
		}
		return out.Data(t)
	}
	return c
}
