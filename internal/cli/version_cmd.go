package cli

import (
	"github.com/spf13/cobra"
	"github.com/thomasmarcelin754/boursobank/internal/out"
	"github.com/thomasmarcelin754/boursobank/internal/version"
)

// newVersionCmd: agent-first build metadata (JSON by default, like every
// other command). `--version` on the root gives the human one-liner.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Affiche version/commit/date du build (JSON)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if out.Format == "table" {
				t := out.Table{Cols: []string{"version", "commit", "date"}}
				i := version.Info()
				t.Rows = append(t.Rows, []string{i["version"], i["commit"], i["date"]})
				return out.Data(t)
			}
			return out.Data(version.Info())
		},
	}
}
