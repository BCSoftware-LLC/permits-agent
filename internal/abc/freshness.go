package abc

import (
	"context"
	"time"
)

func snapshotStale(md map[string]string, now time.Time) bool {
	loaded, err := time.Parse(time.RFC3339, md["loaded_at"])
	if err != nil || now.Sub(loaded) > cacheTTL || loaded.After(now.Add(5*time.Minute)) {
		return true
	}
	export, err := time.ParseInLocation("2006-01-02", md["export_date"], laLocation)
	if err != nil {
		return true
	}
	local := now.In(laLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, laLocation)
	return export.Before(today) || export.After(today)
}

// Provenance returns snapshot identity without computing aggregate record counts.
func (s *Store) Provenance(ctx context.Context) map[string]any {
	md := s.metadata(ctx)
	return map[string]any{"source_url": md["source_url"], "export_date": md["export_date"], "loaded_at": md["loaded_at"], "stale": snapshotStale(md, time.Now()), "snapshot_id": md["export_date"] + "@" + md["loaded_at"]}
}
