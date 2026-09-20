package cli

import (
	"fmt"
	"time"

	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/BCSoftware-LLC/permits-agent/internal/reference"
	"github.com/spf13/cobra"
)

func newAreaCmd() *cobra.Command {
	var a abc.AreaFilter
	var o abc.QueryOptions
	c := &cobra.Command{Use: "area", Aliases: []string{"by"}, Short: "Return records by one area selector", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		rows, err := s.ByArea(cmd.Context(), a, o)
		if err != nil {
			return err
		}
		return printJSON(rows)
	}}
	addAreaFlags(c, &a, 100)
	c.Flags().StringVar(&o.Status, "status", "", "Official ABC status")
	c.Flags().StringVar(&o.Type, "type", "", "Two-digit license type code")
	c.Flags().StringVar(&o.LicOrApp, "lic-or-app", "", "LIC or APP")
	c.Flags().StringVar(&o.ExpireYear, "expires", "", "Expiration four-digit year")
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}

func newExpiringCmd() *cobra.Command {
	var a abc.AreaFilter
	days := 30
	c := &cobra.Command{Use: "expiring", Short: "ACTIVE rows expiring today through N days", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		if days < 0 {
			return abc.ValidationError{Message: "days must be nonnegative"}
		}
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		rows, err := s.Expiring(cmd.Context(), a, days, time.Now())
		if err != nil {
			return err
		}
		return printJSON(rows)
	}}
	addAreaFlags(c, &a, 100)
	c.Flags().IntVar(&days, "days", 30, "Nonnegative days from today")
	c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	return c
}

func newHistoryCmds() []*cobra.Command {
	snapshot := &cobra.Command{Use: "history-snapshot", Short: "Write/read a local mirror history snapshot", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		v, err := s.HistorySnapshot(cmd.Context())
		if err != nil {
			return err
		}
		return printJSON(v)
	}}
	diff := &cobra.Command{Use: "history-diff", Short: "Diff the two latest local mirror snapshots", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		v, err := s.HistoryDiff(cmd.Context())
		if err != nil {
			return err
		}
		return printJSON(v)
	}}
	alias := &cobra.Command{Use: "history", Short: "Alias for history-diff", Args: exact(0), RunE: diff.RunE}
	for _, c := range []*cobra.Command{snapshot, diff, alias} {
		c.Flags().Bool("json", false, "No-op: output is formatted JSON")
	}
	return []*cobra.Command{snapshot, diff, alias}
}

func newReferenceCmds() []*cobra.Command {
	var offline bool
	mk := func(use string, args cobra.PositionalArgs, run func(*cobra.Command, []string) (any, error)) *cobra.Command {
		c := &cobra.Command{Use: use, Short: "Source-backed ABC public reference", Args: args, RunE: func(cmd *cobra.Command, args []string) error {
			v, err := run(cmd, args)
			if err != nil {
				return err
			}
			return printJSON(v)
		}}
		c.Flags().BoolVar(&offline, "offline", false, "Use cached/offline reference mode")
		c.Flags().Bool("json", false, "No-op: output is formatted JSON")
		return c
	}
	types := mk("types [code]", cobra.MaximumNArgs(1), func(cmd *cobra.Command, args []string) (any, error) {
		code := ""
		if len(args) == 1 {
			code = args[0]
		}
		return reference.Types(cmd.Context(), code, offline)
	})
	forms := mk("forms", exact(0), func(cmd *cobra.Command, args []string) (any, error) {
		q, _ := cmd.Flags().GetString("search")
		return reference.Forms(cmd.Context(), q, offline)
	})
	forms.Flags().String("search", "", "Search form number/title")
	fees := mk("fees", exact(0), func(cmd *cobra.Command, args []string) (any, error) { return reference.Fees(cmd.Context(), offline) })
	news := mk("news", exact(0), func(cmd *cobra.Command, args []string) (any, error) {
		feed, _ := cmd.Flags().GetString("feed")
		limit, _ := cmd.Flags().GetInt("limit")
		return reference.News(cmd.Context(), feed, limit, offline)
	})
	news.Flags().String("feed", "news", "news, advisories, or all")
	news.Flags().Int("limit", 10, "Maximum items; 0 means all")
	req := mk("requirements [code] [action]", cobra.RangeArgs(0, 2), func(cmd *cobra.Command, args []string) (any, error) {
		action, _ := cmd.Flags().GetString("action")
		if len(args) == 2 {
			action = args[1]
		}
		code, _ := cmd.Flags().GetString("type")
		if len(args) > 0 {
			if code != "" {
				return nil, usageError{fmt.Errorf("use either positional code or --type, not both")}
			}
			code = args[0]
		}
		if len(args) == 2 && cmd.Flags().Changed("action") {
			return nil, usageError{fmt.Errorf("use either positional action or --action, not both")}
		}
		return reference.Requirements(cmd.Context(), code, action, offline)
	})
	req.Flags().String("action", "new", "new, transfer, or renewal")
	req.Flags().String("type", "", "License type code (alternative to positional code)")
	return []*cobra.Command{types, forms, fees, news, req}
}
