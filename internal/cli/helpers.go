package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/BCSoftware-LLC/permits-agent/internal/reference"
	"github.com/spf13/cobra"
)

type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if abc.IsValidationError(err) || reference.IsValidationError(err) {
		return 2
	}
	msg := err.Error()
	for _, s := range []string{"unknown command", "unknown flag", "accepts ", "expected ", "requires at least", "invalid argument"} {
		if strings.Contains(msg, s) {
			return 2
		}
	}
	var u usageError
	if errors.As(err, &u) {
		return 2
	}
	return 1
}
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func openStore() (*abc.Store, error) {
	s, err := abc.OpenDefault()
	if err != nil {
		return nil, err
	}
	info, err := s.Stats(context.Background())
	if err != nil {
		s.Close()
		return nil, err
	}
	if info.Stale {
		fmt.Fprintln(os.Stderr, "warning: local ABC mirror is stale; export date "+info.ExportDate+"; run abc-agent refresh --force")
	}
	return s, nil
}
func addQueryFlags(c *cobra.Command, o *abc.QueryOptions, def int) {
	c.Flags().IntVar(&o.Limit, "limit", def, "Maximum rows; 0 means all")
	c.Flags().IntVar(&o.Offset, "offset", 0, "Rows to skip")
	c.Flags().StringVar(&o.Status, "status", "", "Official ABC status")
	c.Flags().StringVar(&o.County, "county", "", "Premises county")
	c.Flags().StringVar(&o.City, "city", "", "Premises city")
	c.Flags().StringVar(&o.Type, "type", "", "Two-digit license type code")
	c.Flags().StringVar(&o.LicOrApp, "lic-or-app", "", "LIC or APP")
	c.Flags().StringVar(&o.ExpireYear, "expires", "", "Expiration four-digit year")
}
func addAreaFlags(c *cobra.Command, a *abc.AreaFilter, def int) {
	c.Flags().StringVar(&a.ZIP, "zip", "", "Premises ZIP prefix")
	c.Flags().StringVar(&a.City, "city", "", "Premises city prefix")
	c.Flags().StringVar(&a.County, "county", "", "Premises county prefix")
	c.Flags().StringVar(&a.District, "district", "", "ABC district prefix")
	c.Flags().IntVar(&a.Limit, "limit", def, "Maximum rows; 0 means all")
	c.Flags().IntVar(&a.Offset, "offset", 0, "Rows to skip")
}
func exact(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return usageError{fmt.Errorf("expected %d args, got %d", n, len(args))}
		}
		return nil
	}
}
