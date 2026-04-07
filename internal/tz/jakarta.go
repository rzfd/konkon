package tz

import "time"

// Jakarta is Asia/Jakarta (WIB, UTC+7). Falls back to a fixed +7 zone if tz data is missing.
var Jakarta *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		Jakarta = time.FixedZone("WIB", 7*3600)
		return
	}
	Jakarta = loc
}

// FormatRFC3339 formats t in Jakarta with numeric offset (e.g. 2026-04-02T18:53:00+07:00).
func FormatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(Jakarta).Format(time.RFC3339)
}

// FormatWIB formats t as local Jakarta wall time with WIB suffix (PDF, human-readable exports).
func FormatWIB(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.In(Jakarta).Format("2006-01-02 15:04 WIB")
}

// FormatExportedAt is a single timestamp line for footers ("Diekspor …").
func FormatExportedAt(now time.Time) string {
	return now.In(Jakarta).Format("2006-01-02 15:04 WIB")
}
