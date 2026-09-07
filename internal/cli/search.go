package cli

import (
	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var o abc.QueryOptions
	c := &cobra.Command{Use: "search <query>", Short: "Search licensee name, DBA, and file number", Args: exact(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		rows, err := s.Search(cmd.Context(), args[0], o)
		if err != nil {
			return err
		}
		return printJSON(rows)
	}}
	addQueryFlags(c, &o, 20)
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}
