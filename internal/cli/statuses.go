package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newStatusesCmd() *cobra.Command {
	c := &cobra.Command{Use: "statuses", Short: "Show status vocabulary and observed counts", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		v, err := s.Statuses(cmd.Context(), time.Now())
		if err != nil {
			return err
		}
		return printJSON(v)
	}}
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}
