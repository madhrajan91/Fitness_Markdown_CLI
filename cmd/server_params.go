package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"running_cli/pkg/client"
	"running_cli/pkg/config"
	"running_cli/pkg/storage"
)

// apiVersion is bumped whenever the HTTP API changes incompatibly, so clients
// (the dashboard, running-cli.el) can check they talk to a server they understand.
const apiVersion = 1

// parseWeekParam returns the Monday 00:00 to Sunday 23:59:59 range of the week
// containing the given YYYY-MM-DD date.
func parseWeekParam(s string) (time.Time, time.Time, error) {
	d, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid week %q: use YYYY-MM-DD", s)
	}
	mon := storage.GetMondayOfWeek(d)
	start := time.Date(mon.Year(), mon.Month(), mon.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 7).Add(-1 * time.Second)
	return start, end, nil
}

// parseSourceIDParam reads ?strava_id= or ?garmin_id= from q. ok is false
// when neither is given. Only one may be given.
func parseSourceIDParam(q url.Values) (source string, id int64, ok bool, err error) {
	for _, src := range []string{"strava", "garmin"} {
		v := q.Get(src + "_id")
		if v == "" {
			continue
		}
		if ok {
			return "", 0, false, fmt.Errorf("give only one of strava_id and garmin_id")
		}
		id, err = strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return "", 0, false, fmt.Errorf("invalid %s_id %q", src, v)
		}
		source, ok = src, true
	}
	return source, id, ok, nil
}

// weatherQuery holds the parsed query parameters of GET /api/weather.
type weatherQuery struct {
	Location  string
	Lat, Lon  float64
	HasCoords bool
	Start     time.Time
	Days      int
}

// defaultWeatherDays is what the dashboard has always shown (today plus four days).
const defaultWeatherDays = 5

// parseWeatherQuery reads location, lat, lon, date and days, mirroring the
// flags of the weather command. today is the start date when none is given.
func parseWeatherQuery(q url.Values, today time.Time) (weatherQuery, error) {
	wq := weatherQuery{
		Location: strings.TrimSpace(q.Get("location")),
		Start:    time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local),
		Days:     defaultWeatherDays,
	}

	latStr, lonStr := q.Get("lat"), q.Get("lon")
	if latStr != "" || lonStr != "" {
		if latStr == "" || lonStr == "" {
			return wq, fmt.Errorf("lat and lon must be given together")
		}
		lat, err := strconv.ParseFloat(latStr, 64)
		if err != nil || lat < -90 || lat > 90 {
			return wq, fmt.Errorf("invalid lat %q", latStr)
		}
		lon, err := strconv.ParseFloat(lonStr, 64)
		if err != nil || lon < -180 || lon > 180 {
			return wq, fmt.Errorf("invalid lon %q", lonStr)
		}
		wq.Lat, wq.Lon, wq.HasCoords = lat, lon, true
	}

	if s := q.Get("days"); s != "" {
		days, err := strconv.Atoi(s)
		if err != nil || days < 1 || days > 16 {
			return wq, fmt.Errorf("invalid days %q: use 1-16", s)
		}
		wq.Days = days
	}

	if s := q.Get("date"); s != "" {
		d, err := client.ParseRelativeDate(s)
		if err != nil {
			return wq, err
		}
		wq.Start = d
	}

	return wq, nil
}

// resolveWeatherLocation picks the forecast coordinates: explicit coordinates,
// then a geocoded location name, then the configured location, then IP
// geolocation, then a fixed fallback.
func resolveWeatherLocation(cfg *config.Config, name string, lat, lon float64, hasCoords bool) (float64, float64, string, error) {
	if hasCoords {
		return lat, lon, fmt.Sprintf("Override (%.4f, %.4f)", lat, lon), nil
	}
	if name != "" {
		resolved, err := client.GeocodeLocation(name)
		if err != nil {
			return 0, 0, "", fmt.Errorf("geocoding location %q: %w", name, err)
		}
		return resolved["lat"].(float64), resolved["lon"].(float64), resolved["name"].(string), nil
	}
	if cfg.WeatherLat != nil && cfg.WeatherLon != nil {
		return *cfg.WeatherLat, *cfg.WeatherLon, cfg.WeatherLocation, nil
	}
	if res, err := client.GeolocateByIP(); err == nil {
		return res["lat"].(float64), res["lon"].(float64), res["name"].(string), nil
	}
	return 42.3709, -71.1828, "Watertown, Massachusetts, United States", nil
}

// allowedOrigin reports whether a browser origin may call the API: only pages
// served from this machine (the embedded dashboard or the Vite dev server).
func allowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
