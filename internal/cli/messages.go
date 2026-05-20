package cli

import (
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
)

func newMessagesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "messages",
		Short: "Messages/notifications (Bearer customer/messages + customer/timeline/events)",
	}
	var timeline bool
	c.Flags().BoolVar(&timeline, "timeline", false, "customer/timeline/events au lieu de messages/userpages")
	c.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		cl, _, _, err := session(ctx)
		if err != nil {
			return out.Fail(err)
		}
		var resource string
		if timeline {
			resource = "customer/timeline/events"
		} else {
			resource = "customer/messages/userpages"
		}
		body, handled, err := getJSON(ctx, cl, resource)
		if err != nil {
			return out.Fail(err)
		}
		if handled {
			return nil
		}
		return out.Raw(body)
	}
	return c
}
