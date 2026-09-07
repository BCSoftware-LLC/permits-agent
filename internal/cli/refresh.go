package cli

import (
	"fmt"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
)

func newRefreshCmd() *cobra.Command {
	var jsonOutput bool
	var force bool
	command := &cobra.Command{
		Use:     "refresh",
		Short:   "Download the official ABC daily export and atomically rebuild the local SQLite mirror",
		Example: "  abc-agent refresh\n  abc-agent refresh --force --json",
		Args:    exact(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, info, err := abc.EnsureData(cmd.Context(), force)
			if err != nil {
				return err
			}
			defer store.Close()
			if jsonOutput {
				return printJSON(info)
			}
			fmt.Printf("Mirror ready: %d records (export date: %s)\n", info.Records, info.ExportDate.Format("2006-01-02"))
			return nil
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
	command.Flags().BoolVar(&force, "force", false, "Bypass refresh TTL and fetch official export")
	return command
}
