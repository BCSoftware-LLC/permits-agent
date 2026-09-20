package cli

import (
	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
)

func newAddressCmd() *cobra.Command {
	var o abc.QueryOptions
	c := &cobra.Command{Use: "address <fragment>", Short: "Return records by premises-address fragment", Args: exact(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		rows, err := s.Address(cmd.Context(), args[0], o)
		if err != nil {
			return err
		}
		return printJSON(rows)
	}}
	addQueryFlags(c, &o, 100)
	c.Flags().BoolVar(&o.MatchMail, "mail", false, "Also match mailing addresses")
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}
