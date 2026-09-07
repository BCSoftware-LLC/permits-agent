package abc

import "time"

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
