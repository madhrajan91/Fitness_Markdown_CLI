package cmd

import (
	"net/url"
	"testing"
	"time"
)

func TestParseWeekParam(t *testing.T) {
	cases := []struct {
		in, start, end string
	}{
		{"2026-09-28", "2026-09-28", "2026-10-04"}, // Monday
		{"2026-10-01", "2026-09-28", "2026-10-04"}, // Thursday
		{"2026-10-04", "2026-09-28", "2026-10-04"}, // Sunday stays in the same week
		{"2026-01-01", "2025-12-29", "2026-01-04"}, // across a year boundary
	}
	for _, c := range cases {
		start, end, err := parseWeekParam(c.in)
		if err != nil {
			t.Fatalf("parseWeekParam(%q): %v", c.in, err)
		}
		if got := start.Format("2006-01-02"); got != c.start {
			t.Errorf("parseWeekParam(%q) start = %s, want %s", c.in, got, c.start)
		}
		if got := end.Format("2006-01-02"); got != c.end {
			t.Errorf("parseWeekParam(%q) end = %s, want %s", c.in, got, c.end)
		}
		if start.Hour() != 0 || start.Minute() != 0 {
			t.Errorf("parseWeekParam(%q) start not at midnight: %v", c.in, start)
		}
	}

	for _, bad := range []string{"", "2026-13-01", "28-09-2026", "last week"} {
		if _, _, err := parseWeekParam(bad); err == nil {
			t.Errorf("parseWeekParam(%q) succeeded, want error", bad)
		}
	}
}

func TestParseWeatherQueryDefaults(t *testing.T) {
	today := time.Date(2026, 10, 4, 15, 30, 0, 0, time.Local)
	wq, err := parseWeatherQuery(url.Values{}, today)
	if err != nil {
		t.Fatal(err)
	}
	if wq.Days != defaultWeatherDays {
		t.Errorf("Days = %d, want %d", wq.Days, defaultWeatherDays)
	}
	if want := time.Date(2026, 10, 4, 0, 0, 0, 0, time.Local); !wq.Start.Equal(want) {
		t.Errorf("Start = %v, want %v", wq.Start, want)
	}
	if wq.HasCoords || wq.Location != "" {
		t.Errorf("unexpected location in defaults: %+v", wq)
	}
}

func TestParseWeatherQueryValues(t *testing.T) {
	q := url.Values{
		"location": {"  Boston, MA "},
		"lat":      {"42.36"},
		"lon":      {"-71.06"},
		"days":     {"3"},
		"date":     {"2026-10-10"},
	}
	wq, err := parseWeatherQuery(q, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if wq.Location != "Boston, MA" {
		t.Errorf("Location = %q", wq.Location)
	}
	if !wq.HasCoords || wq.Lat != 42.36 || wq.Lon != -71.06 {
		t.Errorf("coords = %v,%v (has=%v)", wq.Lat, wq.Lon, wq.HasCoords)
	}
	if wq.Days != 3 {
		t.Errorf("Days = %d, want 3", wq.Days)
	}
	if got := wq.Start.Format("2006-01-02"); got != "2026-10-10" {
		t.Errorf("Start = %s, want 2026-10-10", got)
	}
}

func TestParseWeatherQueryErrors(t *testing.T) {
	bad := []url.Values{
		{"lat": {"42"}},                    // lon missing
		{"lon": {"-71"}},                   // lat missing
		{"lat": {"north"}, "lon": {"-71"}}, // not a number
		{"lat": {"91"}, "lon": {"0"}},      // out of range
		{"lat": {"0"}, "lon": {"181"}},
		{"days": {"0"}},
		{"days": {"17"}},
		{"days": {"three"}},
		{"date": {"someday"}},
	}
	for _, q := range bad {
		if _, err := parseWeatherQuery(q, time.Now()); err == nil {
			t.Errorf("parseWeatherQuery(%v) succeeded, want error", q)
		}
	}
}

func TestAllowedOrigin(t *testing.T) {
	allowed := []string{
		"http://localhost:5173",
		"http://127.0.0.1:8080",
		"http://localhost",
		"http://[::1]:5173",
	}
	for _, o := range allowed {
		if !allowedOrigin(o) {
			t.Errorf("allowedOrigin(%q) = false, want true", o)
		}
	}
	denied := []string{
		"",
		"null",
		"https://evil.example",
		"http://localhost.evil.example",
		"http://192.168.1.20:8080",
		"file://",
	}
	for _, o := range denied {
		if allowedOrigin(o) {
			t.Errorf("allowedOrigin(%q) = true, want false", o)
		}
	}
}

func TestParseSourceIDParam(t *testing.T) {
	src, id, ok, err := parseSourceIDParam(url.Values{"strava_id": {"20062347587"}})
	if err != nil || !ok || src != "strava" || id != 20062347587 {
		t.Errorf("strava_id: got %q %d %v %v", src, id, ok, err)
	}
	src, id, ok, err = parseSourceIDParam(url.Values{"garmin_id": {"123"}})
	if err != nil || !ok || src != "garmin" || id != 123 {
		t.Errorf("garmin_id: got %q %d %v %v", src, id, ok, err)
	}
	if _, _, ok, err := parseSourceIDParam(url.Values{"week": {"2026-09-28"}}); ok || err != nil {
		t.Errorf("no id: got ok=%v err=%v", ok, err)
	}
	for _, q := range []url.Values{
		{"strava_id": {"1"}, "garmin_id": {"2"}},
		{"strava_id": {"abc"}},
		{"garmin_id": {"0"}},
		{"strava_id": {"-5"}},
	} {
		if _, _, _, err := parseSourceIDParam(q); err == nil {
			t.Errorf("parseSourceIDParam(%v) succeeded, want error", q)
		}
	}
}
