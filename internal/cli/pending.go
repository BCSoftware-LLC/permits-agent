package cli

import (
	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
)

func newPendingCmd() *cobra.Command {
	var a abc.AreaFilter
	c := &cobra.Command{Use: "pending", Short: "Return pending applications by area", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		rows, err := s.Pending(cmd.Context(), a)
		if err != nil {
			return err
		}
		return printJSON(rows)
	}}
	addAreaFlags(c, &a, 100)
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}
