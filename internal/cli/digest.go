package cli

import (
	"fmt"
	"github.com/BCSoftware-LLC/permits-agent/internal/abc"
	"github.com/BCSoftware-LLC/permits-agent/internal/reference"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func commaValues(raw string) []string {
	out := []string{}
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func newDigestCmd() *cobra.Command {
	var area abc.AreaFilter
	var zips, counties, watch string
	days := 30
	c := &cobra.Command{Use: "digest", Short: "Source-dated public-data brief for one or multiple areas", Args: exact(0), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()
		page := func(rows []abc.Record) []abc.Record {
			if area.Offset >= len(rows) {
				return []abc.Record{}
			}
			rows = rows[area.Offset:]
			if area.Limit > 0 && len(rows) > area.Limit {
				rows = rows[:area.Limit]
			}
			return rows
		}
		query := func(a abc.AreaFilter) (map[string]any, error) {
			a.Limit = 0
			a.Offset = 0
			p, err := s.Pending(cmd.Context(), a)
			if err != nil {
				return nil, err
			}
			o, err := s.Overdue(cmd.Context(), a, time.Now())
			if err != nil {
				return nil, err
			}
			e, err := s.Expiring(cmd.Context(), a, days, time.Now())
			if err != nil {
				return nil, err
			}
			return map[string]any{"pending_total": len(p), "pending_count": len(p), "overdue_total": len(o), "expiring_total": len(e), "pending": page(p), "overdue": page(o), "expiring": page(e)}, nil
		}
		out := map[string]any{"provenance": s.Provenance(cmd.Context())}
		brief := []string{"ABC PUBLIC-DATA BRIEF — source date " + fmt.Sprint(out["provenance"].(map[string]any)["export_date"])}
		if zips != "" || counties != "" {
			areas := []map[string]any{}
			for _, spec := range []struct{ raw, kind string }{{zips, "zip"}, {counties, "county"}} {
				for _, v := range commaValues(spec.raw) {
					a := abc.AreaFilter{}
					if spec.kind == "zip" {
						a.ZIP = v
					} else {
						a.County = v
					}
					r, err := query(a)
					if err != nil {
						return err
					}
					r["kind"] = spec.kind
					r["value"] = v
					areas = append(areas, r)
					brief = append(brief, fmt.Sprintf("%s %s: %v pending, %v overdue candidates, %v expiring", spec.kind, v, r["pending_total"], r["overdue_total"], r["expiring_total"]))
				}
			}
			out["areas"] = areas
		} else {
			r, err := query(area)
			if err != nil {
				return err
			}
			for k, v := range r {
				out[k] = v
			}
			brief = append(brief, fmt.Sprintf("%v pending; %v overdue candidates; %v expiring", r["pending_total"], r["overdue_total"], r["expiring_total"]))
		}
		watched := []abc.Record{}
		for _, file := range commaValues(watch) {
			rows, err := s.Get(cmd.Context(), file)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("watched file number %s not found", file)
			}
			watched = append(watched, rows...)
		}
		if watch != "" {
			out["watch"] = watched
		}
		news, err := reference.News(cmd.Context(), "all", 0, true)
		if err != nil {
			out["news"] = nil
			out["warnings"] = []string{"News unavailable: " + err.Error()}
			brief = append(brief, "News unavailable; refresh news explicitly before relying on advisories.")
		} else {
			out["news"] = news
		}
		out["brief"] = strings.Join(brief, "\n")
		return printJSON(out)
	}}
	addAreaFlags(c, &area, 25)
	c.Flags().StringVar(&zips, "zips", "", "Comma-separated premises ZIP prefixes")
	c.Flags().StringVar(&counties, "counties", "", "Comma-separated premises counties")
	c.Flags().StringVar(&watch, "watch", "", "Comma-separated exact eight-digit file numbers")
	c.Flags().IntVar(&days, "days", 30, "Expiring horizon")
	c.Flags().Bool("json", false, "Output is formatted JSON")
	return c
}
