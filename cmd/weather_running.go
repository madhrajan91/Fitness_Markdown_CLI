package cmd

import (
	"math"
	"strings"
	"time"

	"running_cli/pkg/client"
)

// Running hours considered for the hourly forecast and the best run window.
const (
	firstRunHour = 5
	lastRunHour  = 19
)

// runningHour is one daytime hour of the forecast with its RPE.
type runningHour struct {
	Time       string  `json:"time"` // YYYY-MM-DDTHH:MM, local to the location
	Condition  string  `json:"condition"`
	Emoji      string  `json:"emoji"`
	Temp       float64 `json:"temp"`
	FeelsLike  float64 `json:"feels_like"`
	Humidity   float64 `json:"humidity"`
	Wind       float64 `json:"wind"`
	WindDir    string  `json:"wind_dir"`
	PrecipProb float64 `json:"precip_prob"`
	RPE        float64 `json:"rpe"`
	RPELabel   string  `json:"rpe_label"`
	RPEDesc    string  `json:"rpe_desc"`
}

// runningDay summarizes one day, with the daytime hour of lowest RPE.
type runningDay struct {
	Date      string       `json:"date"`
	Condition string       `json:"condition"`
	Emoji     string       `json:"emoji"`
	TempMin   float64      `json:"temp_min"`
	TempMax   float64      `json:"temp_max"`
	WindMax   float64      `json:"wind_max"`
	Sunrise   string       `json:"sunrise"`
	Sunset    string       `json:"sunset"`
	Best      *runningHour `json:"best,omitempty"`
}

// runningWeather is the client-friendly view of a forecast added to
// /api/weather replies under "running".
type runningWeather struct {
	TempUnit string        `json:"temp_unit"`
	WindUnit string        `json:"wind_unit"`
	Days     []runningDay  `json:"days"`
	Hours    []runningHour `json:"hours"`
}

func seriesFloat(m map[string]interface{}, key string, i int) float64 {
	arr, _ := m[key].([]interface{})
	if i >= len(arr) {
		return 0
	}
	f, _ := arr[i].(float64)
	return f
}

func seriesString(m map[string]interface{}, key string, i int) string {
	arr, _ := m[key].([]interface{})
	if i >= len(arr) {
		return ""
	}
	s, _ := arr[i].(string)
	return s
}

func wmoCondition(code float64) (string, string) {
	cond := client.WMOCodes[int(code)]
	if cond[0] == "" {
		return "Unknown", "❓"
	}
	return cond[0], cond[1]
}

// buildRunningWeather turns an Open-Meteo forecast (in the units FetchWeather
// requested for unit) into daily summaries and daytime hours with RPE.
func buildRunningWeather(forecast map[string]interface{}, unit string) runningWeather {
	rw := runningWeather{TempUnit: "°F", WindUnit: "mph", Days: []runningDay{}, Hours: []runningHour{}}
	if unit == "km" {
		rw.TempUnit, rw.WindUnit = "°C", "km/h"
	}

	hourly, _ := forecast["hourly"].(map[string]interface{})
	hourTimes, _ := hourly["time"].([]interface{})
	for i := range hourTimes {
		ts := seriesString(hourly, "time", i)
		t, err := time.Parse("2006-01-02T15:04", ts)
		if err != nil || t.Hour() < firstRunHour || t.Hour() > lastRunHour {
			continue
		}

		temp := seriesFloat(hourly, "temperature_2m", i)
		hum := seriesFloat(hourly, "relative_humidity_2m", i)
		wind := seriesFloat(hourly, "wind_speed_10m", i)
		tempF, windMPH := temp, wind
		if unit == "km" {
			tempF = temp*9.0/5.0 + 32.0
			windMPH = wind * 0.621371
		}
		score, label, desc := client.CalculatePerceivedExertion(tempF, hum, windMPH)
		cond, emoji := wmoCondition(seriesFloat(hourly, "weather_code", i))

		rw.Hours = append(rw.Hours, runningHour{
			Time:       ts,
			Condition:  cond,
			Emoji:      emoji,
			Temp:       math.Round(temp*10) / 10,
			FeelsLike:  math.Round(seriesFloat(hourly, "apparent_temperature", i)*10) / 10,
			Humidity:   hum,
			Wind:       math.Round(wind*10) / 10,
			WindDir:    client.DegreesToCardinal(seriesFloat(hourly, "wind_direction_10m", i)),
			PrecipProb: seriesFloat(hourly, "precipitation_probability", i),
			RPE:        score,
			RPELabel:   label,
			RPEDesc:    desc,
		})
	}

	daily, _ := forecast["daily"].(map[string]interface{})
	dayTimes, _ := daily["time"].([]interface{})
	for i := range dayTimes {
		date := seriesString(daily, "time", i)
		cond, emoji := wmoCondition(seriesFloat(daily, "weather_code", i))
		day := runningDay{
			Date:      date,
			Condition: cond,
			Emoji:     emoji,
			TempMin:   math.Round(seriesFloat(daily, "temperature_2m_min", i)*10) / 10,
			TempMax:   math.Round(seriesFloat(daily, "temperature_2m_max", i)*10) / 10,
			WindMax:   math.Round(seriesFloat(daily, "wind_speed_10m_max", i)*10) / 10,
			Sunrise:   seriesString(daily, "sunrise", i),
			Sunset:    seriesString(daily, "sunset", i),
		}
		day.Best = bestHour(rw.Hours, date, day.Sunrise, day.Sunset)
		rw.Days = append(rw.Days, day)
	}

	return rw
}

// bestHour is the date's hour of lowest RPE between sunrise and sunset, or from
// all its hours if none falls in daylight. The coolest hour is usually before
// dawn, which isn't when anyone means to run. Timestamps are all
// YYYY-MM-DDTHH:MM, so they compare as strings.
func bestHour(hours []runningHour, date, sunrise, sunset string) *runningHour {
	var best, bestDaylight *runningHour
	for i := range hours {
		h := &hours[i]
		if !strings.HasPrefix(h.Time, date) {
			continue
		}
		if best == nil || h.RPE < best.RPE {
			best = h
		}
		if h.Time >= sunrise && (sunset == "" || h.Time <= sunset) && (bestDaylight == nil || h.RPE < bestDaylight.RPE) {
			bestDaylight = h
		}
	}
	if bestDaylight != nil {
		best = bestDaylight
	}
	if best == nil {
		return nil
	}
	found := *best
	return &found
}
