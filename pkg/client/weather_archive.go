package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ObservedWeather is the weather recorded at a single hour.
type ObservedWeather struct {
	TempC       float64
	HumidityPct float64
	WindKph     float64
	WeatherCode int
}

// HourKey formats a time as the key used by the hourly maps below. Times are
// normalised to UTC so activity timestamps and API responses always agree.
func HourKey(t time.Time) string {
	return t.UTC().Format("2006-01-02T15")
}

// archiveCutoffDays is how far back the archive API is reliably populated.
// Open-Meteo's archive lags real time by several days, so anything newer is
// read from the forecast API's past_days window instead.
const archiveCutoffDays = 7

// forecastPastDaysMax is the largest past_days the forecast API accepts.
const forecastPastDaysMax = 92

const hourlyVars = "temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code"

type openMeteoHourly struct {
	Hourly struct {
		Time        []string  `json:"time"`
		Temperature []float64 `json:"temperature_2m"`
		Humidity    []float64 `json:"relative_humidity_2m"`
		WindSpeed   []float64 `json:"wind_speed_10m"`
		WeatherCode []int     `json:"weather_code"`
	} `json:"hourly"`
}

// FetchHistoricalHourly returns hourly observations for a location between
// start and end (inclusive), keyed by HourKey. It transparently splits the
// request between the archive API and the forecast API's past_days window so
// that recent activities resolve too.
func FetchHistoricalHourly(lat, lon float64, start, end time.Time) (map[string]ObservedWeather, error) {
	out := make(map[string]ObservedWeather)
	cutoff := time.Now().UTC().AddDate(0, 0, -archiveCutoffDays)

	// Older portion: the archive API.
	if start.Before(cutoff) {
		archiveEnd := end
		if archiveEnd.After(cutoff) {
			archiveEnd = cutoff
		}
		url := fmt.Sprintf(
			"https://archive-api.open-meteo.com/v1/archive?latitude=%.4f&longitude=%.4f"+
				"&start_date=%s&end_date=%s&hourly=%s&timezone=UTC",
			lat, lon, start.UTC().Format("2006-01-02"), archiveEnd.UTC().Format("2006-01-02"), hourlyVars,
		)
		if err := collectHourly(url, out); err != nil {
			return nil, err
		}
	}

	// Recent portion: the forecast API, which carries up to 92 past days.
	if end.After(cutoff) {
		pastDays := int(time.Since(start).Hours()/24) + 1
		if pastDays > forecastPastDaysMax {
			pastDays = forecastPastDaysMax
		}
		if pastDays < 1 {
			pastDays = 1
		}
		url := fmt.Sprintf(
			"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
				"&past_days=%d&forecast_days=1&hourly=%s&timezone=UTC",
			lat, lon, pastDays, hourlyVars,
		)
		if err := collectHourly(url, out); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// collectHourly fetches one Open-Meteo hourly response and merges it into dst.
func collectHourly(url string, dst map[string]ObservedWeather) error {
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("open-meteo responded %d", resp.StatusCode)
	}

	var payload openMeteoHourly
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}

	h := payload.Hourly
	for i, ts := range h.Time {
		// Open-Meteo pads all series to the same length, but guard anyway and
		// skip hours where the archive has no reading yet.
		if i >= len(h.Temperature) || i >= len(h.Humidity) || i >= len(h.WindSpeed) || i >= len(h.WeatherCode) {
			break
		}
		t, err := time.Parse("2006-01-02T15:04", ts)
		if err != nil {
			continue
		}
		dst[HourKey(t.UTC())] = ObservedWeather{
			TempC:       h.Temperature[i],
			HumidityPct: h.Humidity[i],
			WindKph:     h.WindSpeed[i],
			WeatherCode: h.WeatherCode[i],
		}
	}

	return nil
}
