package cli

import (
	"regexp"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	c := &cobra.Command{Use: "get <file-number>", Short: "Return every record for an exact eight-digit file number", Args: exact(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !regexp.MustCompile(`^\d{8}$`).MatchString(args[0]) {
			return abc.ValidationError{Message: "file number must be exactly eight digits"}
		}
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		rows, err := s.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return printJSON(rows)
	}}
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}
