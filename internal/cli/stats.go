package cli

import "github.com/spf13/cobra"

func newStatsCmd() *cobra.Command {
	c := &cobra.Command{Use: "stats", Short: "Summarize mirror records/status counts", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		st, err := s.Stats(cmd.Context())
		if err != nil {
			return err
		}
		return printJSON(st)
	}}
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}
