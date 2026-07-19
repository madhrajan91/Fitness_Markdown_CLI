package client

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WMOCodes maps Open-Meteo weather codes to a name and emoji
var WMOCodes = map[int][2]string{
	0:  {"Clear", "☀️"},
	1:  {"Mainly Clear", "🌤️"},
	2:  {"Partly Cloudy", "⛅"},
	3:  {"Overcast", "☁️"},
	45: {"Fog", "🌫️"},
	48: {"Depositing Rime Fog", "🌫️"},
	51: {"Light Drizzle", "🌧️"},
	53: {"Moderate Drizzle", "🌧️"},
	55: {"Dense Drizzle", "🌧️"},
	56: {"Light Freezing Drizzle", "🌧️"},
	57: {"Dense Freezing Drizzle", "🌧️"},
	61: {"Slight Rain", "🌧️"},
	63: {"Moderate Rain", "🌧️"},
	65: {"Heavy Rain", "🌧️"},
	66: {"Light Freezing Rain", "🌧️"},
	67: {"Heavy Freezing Rain", "🌧️"},
	71: {"Slight Snow", "❄️"},
	73: {"Moderate Snow", "❄️"},
	75: {"Heavy Snow", "❄️"},
	77: {"Snow Grains", "❄️"},
	80: {"Slight Rain Showers", "🌦️"},
	81: {"Moderate Rain Showers", "🌦️"},
	82: {"Violent Rain Showers", "🌦️"},
	85: {"Slight Snow Showers", "❄️"},
	86: {"Heavy Snow Showers", "❄️"},
	95: {"Thunderstorm", "⛈️"},
	96: {"Thunderstorm with Hail", "⛈️"},
	99: {"Severe Thunderstorm", "⛈️"},
}

// StateMap maps state abbreviations to full names
var StateMap = map[string]string{
	"AL": "Alabama", "AK": "Alaska", "AZ": "Arizona", "AR": "Arkansas", "CA": "California",
	"CO": "Colorado", "CT": "Connecticut", "DE": "Delaware", "FL": "Florida", "GA": "Georgia",
	"HI": "Hawaii", "ID": "Idaho", "IL": "Illinois", "IN": "Indiana", "IA": "Iowa",
	"KS": "Kansas", "KY": "Kentucky", "LA": "Louisiana", "ME": "Maine", "MD": "Maryland",
	"MA": "Massachusetts", "MI": "Michigan", "MN": "Minnesota", "MS": "Mississippi", "MO": "Missouri",
	"MT": "Montana", "NE": "Nebraska", "NV": "Nevada", "NH": "New Hampshire", "NJ": "New Jersey",
	"NM": "New Mexico", "NY": "New York", "NC": "North Carolina", "ND": "North Dakota", "OH": "Ohio",
	"OK": "Oklahoma", "OR": "Oregon", "PA": "Pennsylvania", "RI": "Rhode Island", "SC": "South Carolina",
	"SD": "South Dakota", "TN": "Tennessee", "TX": "Texas", "UT": "Utah", "VT": "Vermont",
	"VA": "Virginia", "WA": "Washington", "WV": "West Virginia", "WI": "Wisconsin", "WY": "Wyoming",
}

// GeocodeLocation geocodes a location name using the Open-Meteo Geocoding API
func GeocodeLocation(locationName string) (map[string]interface{}, error) {
	if strings.TrimSpace(locationName) == "" {
		return nil, fmt.Errorf("empty location name")
	}

	searchName := strings.TrimSpace(locationName)
	filterState := ""
	if strings.Contains(locationName, ",") {
		parts := strings.Split(locationName, ",")
		searchName = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			filterState = strings.TrimSpace(parts[1])
		}
	}

	apiURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=50", url.QueryEscape(searchName))
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
	}

	var data struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Results) == 0 {
		return nil, fmt.Errorf("location not found")
	}

	var selected map[string]interface{}
	if filterState != "" {
		filterStateLower := strings.ToLower(filterState)
		fullStateName := strings.ToLower(StateMap[strings.ToUpper(filterState)])

		for _, res := range data.Results {
			admin1, _ := res["admin1"].(string)
			if admin1 != "" {
				admin1Lower := strings.ToLower(admin1)
				if admin1Lower == filterStateLower || (fullStateName != "" && admin1Lower == fullStateName) {
					selected = res
					break
				}
			}
		}
	}

	if selected == nil {
		selected = data.Results[0]
	}

	// Format a pretty name
	var parts []string
	if name, ok := selected["name"].(string); ok && name != "" {
		parts = append(parts, name)
	}
	if admin1, ok := selected["admin1"].(string); ok && admin1 != "" {
		parts = append(parts, admin1)
	}
	if country, ok := selected["country"].(string); ok && country != "" {
		parts = append(parts, country)
	}
	formattedName := strings.Join(parts, ", ")

	lat, _ := selected["latitude"].(float64)
	lon, _ := selected["longitude"].(float64)

	return map[string]interface{}{
		"lat":  lat,
		"lon":  lon,
		"name": formattedName,
	}, nil
}

// GeolocateByIP locates user by IP address
func GeolocateByIP() (map[string]interface{}, error) {
	// Try primary
	resp, err := http.Get("http://ip-api.com/json/")
	if err == nil {
		defer resp.Body.Close()
		var data struct {
			Status      string  `json:"status"`
			City        string  `json:"city"`
			RegionName  string  `json:"regionName"`
			CountryCode string  `json:"countryCode"`
			Lat         float64 `json:"lat"`
			Lon         float64 `json:"lon"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && data.Status == "success" {
			var parts []string
			if data.City != "" {
				parts = append(parts, data.City)
			}
			if data.RegionName != "" {
				parts = append(parts, data.RegionName)
			}
			if data.CountryCode != "" {
				parts = append(parts, data.CountryCode)
			}
			return map[string]interface{}{
				"lat":  data.Lat,
				"lon":  data.Lon,
				"name": strings.Join(parts, ", "),
			}, nil
		}
	}

	// Try fallback
	resp2, err2 := http.Get("https://ipapi.co/json/")
	if err2 == nil {
		defer resp2.Body.Close()
		var data struct {
			Error       bool    `json:"error"`
			City        string  `json:"city"`
			Region      string  `json:"region"`
			CountryName string  `json:"country_name"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
		}
		if err := json.NewDecoder(resp2.Body).Decode(&data); err == nil && !data.Error {
			var parts []string
			if data.City != "" {
				parts = append(parts, data.City)
			}
			if data.Region != "" {
				parts = append(parts, data.Region)
			}
			if data.CountryName != "" {
				parts = append(parts, data.CountryName)
			}
			return map[string]interface{}{
				"lat":  data.Latitude,
				"lon":  data.Longitude,
				"name": strings.Join(parts, ", "),
			}, nil
		}
	}

	return nil, fmt.Errorf("IP geolocation failed")
}

// DegreesToCardinal converts wind degrees to cardinal direction
func DegreesToCardinal(degrees float64) string {
	cardinals := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	idx := int((degrees+11.25)/22.5) % 16
	return cardinals[idx]
}

// CalculatePerceivedExertion estimates running perceived exertion score (RPE) from 0 to 10+
func CalculatePerceivedExertion(tempF, humidity, windMPH float64) (float64, string, string) {
	// 1. Dew point impact
	tempC := (tempF - 32.0) * 5.0 / 9.0
	dewPointC := tempC - ((100.0 - humidity) / 5.0)
	dewPointF := (dewPointC * 9.0 / 5.0) + 32.0

	var humidityPoints float64
	switch {
	case dewPointF < 50:
		humidityPoints = 0.0
	case dewPointF >= 50 && dewPointF < 60:
		humidityPoints = 0.5
	case dewPointF >= 60 && dewPointF < 65:
		humidityPoints = 1.5
	case dewPointF >= 65 && dewPointF < 70:
		humidityPoints = 3.0
	case dewPointF >= 70 && dewPointF < 75:
		humidityPoints = 4.5
	default: // >= 75
		humidityPoints = 6.0
	}

	// 2. Temperature impact
	var tempPoints float64
	switch {
	case tempF < 20:
		tempPoints = 3.0
	case tempF >= 20 && tempF < 32:
		tempPoints = 1.5
	case tempF >= 32 && tempF <= 60:
		tempPoints = 0.0 // Ideal
	case tempF > 60 && tempF <= 70:
		tempPoints = 0.5
	case tempF > 70 && tempF <= 80:
		tempPoints = 1.5
	case tempF > 80 && tempF <= 90:
		tempPoints = 3.0
	default: // > 90
		tempPoints = 5.0
	}

	// 3. Wind impact
	var windPoints float64
	switch {
	case windMPH < 10:
		windPoints = 0.0
	case windMPH >= 10 && windMPH <= 18:
		windPoints = 1.0
	case windMPH > 18 && windMPH <= 25:
		windPoints = 2.0
	default: // > 25
		windPoints = 3.5
	}

	score := humidityPoints + tempPoints + windPoints
	score = math.Round(score*10) / 10.0

	label := ""
	desc := ""
	switch {
	case score <= 1.5:
		label = "Ideal"
		desc = "Perfect running conditions. RPE is baseline."
	case score > 1.5 && score <= 3.0:
		label = "Moderate"
		desc = "Slightly increased effort. Keep steady pace."
	case score > 3.0 && score <= 5.0:
		label = "Hard"
		desc = "Noticeable RPE increase. Slow pace slightly."
	case score > 5.0 && score <= 7.0:
		label = "Very Hard"
		desc = "High cardiovascular strain. Hydrate and run by feel."
	default:
		label = "Extreme"
		desc = "Severe strain. Avoid peak hours or run indoors."
	}

	return score, label, desc
}

// FetchWeather fetches forecast from Open-Meteo API
func FetchWeather(lat, lon float64, startDate, endDate time.Time, unit string) (map[string]interface{}, error) {
	tempUnit := "fahrenheit"
	windUnit := "mph"
	if unit == "km" {
		tempUnit = "celsius"
		windUnit = "kmh"
	}

	apiURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.6f&longitude=%.6f&"+
			"hourly=temperature_2m,relative_humidity_2m,apparent_temperature,wind_speed_10m,wind_direction_10m,precipitation_probability,weather_code&"+
			"daily=sunrise,sunset,temperature_2m_max,temperature_2m_min,wind_speed_10m_max,weather_code&"+
			"timezone=auto&start_date=%s&end_date=%s&temperature_unit=%s&wind_speed_unit=%s",
		lat, lon, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), tempUnit, windUnit,
	)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// ParseRelativeDate parses text dates like "today", "+3", "tomorrow", or "monday"
func ParseRelativeDate(input string) (time.Time, error) {
	clean := strings.ToLower(strings.TrimSpace(input))
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	if clean == "today" || clean == "0" {
		return today, nil
	}
	if clean == "tomorrow" || clean == "1" || clean == "+1" {
		return today.AddDate(0, 0, 1), nil
	}
	if clean == "day-after" || clean == "day_after" || clean == "2" || clean == "+2" {
		return today.AddDate(0, 0, 2), nil
	}

	// Offset number check (e.g. +4, 5)
	offsetStr := clean
	if strings.HasPrefix(clean, "+") {
		offsetStr = clean[1:]
	}

	var days int
	if _, err := fmt.Sscanf(offsetStr, "%d", &days); err == nil {
		if days >= 0 && days <= 7 {
			return today.AddDate(0, 0, days), nil
		}
	}

	// Weekdays
	weekdays := map[string]time.Weekday{
		"monday":    time.Monday,
		"mon":       time.Monday,
		"tuesday":   time.Tuesday,
		"tue":       time.Tuesday,
		"wednesday": time.Wednesday,
		"wed":       time.Wednesday,
		"thursday":  time.Thursday,
		"thu":       time.Thursday,
		"friday":    time.Friday,
		"fri":       time.Friday,
		"saturday":  time.Saturday,
		"sat":       time.Saturday,
		"sunday":    time.Sunday,
		"sun":       time.Sunday,
	}

	if targetWd, ok := weekdays[clean]; ok {
		currentWd := today.Weekday()
		daysAhead := int(targetWd - currentWd)
		if daysAhead <= 0 {
			daysAhead += 7
		}
		return today.AddDate(0, 0, daysAhead), nil
	}

	// Try standard date parsing
	fmts := []string{"2006-01-02", "01/02/2006", "01-02-2006"}
	for _, f := range fmts {
		if t, err := time.ParseInLocation(f, input, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse relative date: %s", input)
}
