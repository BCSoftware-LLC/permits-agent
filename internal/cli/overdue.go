package cli

import (
	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
	"time"
)

func newOverdueCmd() *cobra.Command {
	var a abc.AreaFilter
	c := &cobra.Command{Use: "overdue", Short: "Derived ACTIVE rows expired before today; not a legal determination", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		if a.MinDays < 0 {
			return abc.ValidationError{Message: "min-days must be nonnegative"}
		}
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		rows, err := s.Overdue(cmd.Context(), a, time.Now())
		if err != nil {
			return err
		}
		return printJSON(rows)
	}}
	addAreaFlags(c, &a, 100)
	c.Flags().IntVar(&a.MinDays, "min-days", 0, "Minimum days past expiration")
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}
