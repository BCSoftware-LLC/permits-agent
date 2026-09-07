package cli

import (
	"fmt"
	"runtime"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var jsonOutput bool
	c := &cobra.Command{Use: "doctor", Short: "Check local cache/source/version health", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		out := map[string]any{"version": Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "cache_ok": false, "source_url": abc.ExportURL}
		store, err := abc.OpenDefault()
		if err != nil {
			out["cache_error"] = err.Error()
			if jsonOutput {
				_ = printJSON(out)
			}
			return err
		}
		defer store.Close()
		stats, err := store.Stats(cmd.Context())
		if err != nil {
			out["cache_error"] = err.Error()
			if jsonOutput {
				_ = printJSON(out)
			}
			return err
		}
		out["cache_ok"] = true
		out["records"] = stats.TotalRecords
		out["export_date"] = stats.ExportDate
		out["loaded_at"] = stats.LoadedAt
		out["stale"] = stats.Stale
		if jsonOutput {
			return printJSON(out)
		}
		fmt.Printf("abc-agent %s\ncache: ok (%d records, export %s, stale %v)\nsource: %s\n", Version, stats.TotalRecords, stats.ExportDate, stats.Stale, abc.ExportURL)
		return nil
	}}
	c.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
	return c
}
