package cmd

import (
	"encoding/json"
	"testing"
)

// A two-day Open-Meteo forecast trimmed to a few hours, in Fahrenheit/mph.
const forecastFixture = `{
  "hourly": {
    "time": ["2026-10-04T04:00", "2026-10-04T07:00", "2026-10-04T15:00", "2026-10-04T20:00", "2026-10-05T06:00", "2026-10-05T12:00"],
    "temperature_2m":            [40, 50, 82, 60, 72, 88],
    "relative_humidity_2m":      [90, 60, 70, 50, 90, 80],
    "apparent_temperature":      [37, 49, 86, 59, 74, 95],
    "wind_speed_10m":            [3, 5, 20, 4, 4, 12],
    "wind_direction_10m":        [0, 90, 180, 270, 45, 0],
    "precipitation_probability": [0, 10, 40, 0, 80, 20],
    "weather_code":              [0, 1, 61, 0, 999, 3]
  },
  "daily": {
    "time": ["2026-10-04", "2026-10-05", "2026-10-06"],
    "weather_code": [61, 3, 0],
    "temperature_2m_max": [82.4, 88, 70],
    "temperature_2m_min": [40.04, 70, 55],
    "wind_speed_10m_max": [20, 12, 5],
    "sunrise": ["2026-10-04T06:51", "2026-10-05T06:52", "2026-10-06T06:53"],
    "sunset":  ["2026-10-04T18:22", "2026-10-05T18:20", "2026-10-06T18:18"]
  }
}`

func TestBuildRunningWeather(t *testing.T) {
	var forecast map[string]interface{}
	if err := json.Unmarshal([]byte(forecastFixture), &forecast); err != nil {
		t.Fatal(err)
	}
	rw := buildRunningWeather(forecast, "miles")

	if rw.TempUnit != "°F" || rw.WindUnit != "mph" {
		t.Errorf("units = %s %s", rw.TempUnit, rw.WindUnit)
	}

	// 04:00 and 20:00 are outside running hours.
	if len(rw.Hours) != 4 {
		t.Fatalf("got %d hours, want 4: %+v", len(rw.Hours), rw.Hours)
	}
	h := rw.Hours[0]
	if h.Time != "2026-10-04T07:00" || h.WindDir != "E" || h.RPE != 0 || h.RPELabel != "Ideal" || h.Condition == "" {
		t.Errorf("first hour = %+v", h)
	}
	if rw.Hours[2].Condition != "Unknown" || rw.Hours[2].Emoji != "❓" {
		t.Errorf("unknown weather code = %+v", rw.Hours[2])
	}
	if rw.Hours[1].RPE <= rw.Hours[0].RPE {
		t.Errorf("hot, windy, humid afternoon should score higher: %v vs %v", rw.Hours[1].RPE, rw.Hours[0].RPE)
	}

	if len(rw.Days) != 3 {
		t.Fatalf("got %d days, want 3", len(rw.Days))
	}
	d := rw.Days[0]
	if d.Date != "2026-10-04" || d.TempMin != 40 || d.TempMax != 82.4 || d.Sunrise != "2026-10-04T06:51" {
		t.Errorf("day 1 = %+v", d)
	}
	if d.Best == nil || d.Best.Time != "2026-10-04T07:00" {
		t.Errorf("day 1 best = %+v", d.Best)
	}
	if rw.Days[1].Best == nil || rw.Days[1].Best.Time != "2026-10-05T06:00" {
		t.Errorf("day 2 best = %+v", rw.Days[1].Best)
	}
	// No hourly data for the third day: no best window.
	if rw.Days[2].Best != nil {
		t.Errorf("day 3 best = %+v, want nil", rw.Days[2].Best)
	}

	// Metric forecasts are converted before scoring.
	if km := buildRunningWeather(forecast, "km"); km.TempUnit != "°C" || km.WindUnit != "km/h" {
		t.Errorf("km units = %s %s", km.TempUnit, km.WindUnit)
	}

	// An empty forecast gives empty lists, not nulls.
	empty := buildRunningWeather(map[string]interface{}{}, "miles")
	if empty.Days == nil || empty.Hours == nil {
		t.Error("empty forecast should give empty slices")
	}
}
