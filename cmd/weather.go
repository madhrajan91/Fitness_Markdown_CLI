package cmd

import (
	"fmt"
	"math"
	"strings"
	"time"

	"running_cli/pkg/client"
	"running_cli/pkg/config"

	"github.com/spf13/cobra"
)

var (
	weatherLoc  string
	weatherLat  float64
	weatherLon  float64
	weatherDate string
	weatherDays int
)

var weatherCmd = &cobra.Command{
	Use:   "weather",
	Short: "Fetch and display weather forecast with Running Perceived Exertion (RPE)",
	Run:   runWeather,
}

func init() {
	weatherCmd.Flags().StringVar(&weatherLoc, "location", "", "Location name to geocode")
	weatherCmd.Flags().Float64Var(&weatherLat, "lat", 0.0, "Latitude override")
	weatherCmd.Flags().Float64Var(&weatherLon, "lon", 0.0, "Longitude override")
	weatherCmd.Flags().StringVar(&weatherDate, "date", "", "Target date (YYYY-MM-DD or keywords like today/tomorrow/monday)")
	weatherCmd.Flags().IntVar(&weatherDays, "days", 1, "Number of days to forecast (1-16)")

	rootCmd.AddCommand(weatherCmd)
}

func runWeather(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// 1. Resolve Location Coordinates
	var lat, lon float64
	locName := ""

	if weatherLat != 0.0 && weatherLon != 0.0 {
		lat = weatherLat
		lon = weatherLon
		locName = fmt.Sprintf("Override (%.4f, %.4f)", lat, lon)
	} else if weatherLoc != "" {
		fmt.Printf("Geocoding location '%s'...\n", weatherLoc)
		resolved, err := client.GeocodeLocation(weatherLoc)
		if err != nil {
			fmt.Printf("Error geocoding location: %v\n", err)
			return
		}
		lat = resolved["lat"].(float64)
		lon = resolved["lon"].(float64)
		locName = resolved["name"].(string)
	} else if cfg.WeatherLat != nil && cfg.WeatherLon != nil {
		lat = *cfg.WeatherLat
		lon = *cfg.WeatherLon
		locName = cfg.WeatherLocation
	} else {
		// Fallback to IP geolocation
		res, err := client.GeolocateByIP()
		if err == nil {
			lat = res["lat"].(float64)
			lon = res["lon"].(float64)
			locName = res["name"].(string)
		} else {
			// default fallback
			lat = 42.3709
			lon = -71.1828
			locName = "Watertown, Massachusetts, United States"
		}
	}

	// 2. Resolve Date range
	var targetDate time.Time
	if weatherDate != "" {
		targetDate, err = client.ParseRelativeDate(weatherDate)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	} else {
		now := time.Now()
		targetDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	}

	endDate := targetDate.AddDate(0, 0, weatherDays-1)

	fmt.Printf("Location: %s (%.4f, %.4f)\n", locName, lat, lon)
	fmt.Printf("Forecast: %s to %s (%d days)\n\n",
		targetDate.Format("2006-01-02"), endDate.Format("2006-01-02"), weatherDays)

	// 3. Fetch Forecast API
	weatherData, err := client.FetchWeather(lat, lon, targetDate, endDate, cfg.DistanceUnit)
	if err != nil {
		fmt.Printf("Error fetching weather forecast: %v\n", err)
		return
	}

	tempSuffix := "°F"
	windSuffix := "mph"
	if cfg.DistanceUnit == "km" {
		tempSuffix = "°C"
		windSuffix = "kmh"
	}

	if weatherDays == 1 {
		// Hourly breakdown
		hourly, _ := weatherData["hourly"].(map[string]interface{})
		times, _ := hourly["time"].([]interface{})
		temps, _ := hourly["temperature_2m"].([]interface{})
		apparentTemps, _ := hourly["apparent_temperature"].([]interface{})
		humidities, _ := hourly["relative_humidity_2m"].([]interface{})
		windSpeeds, _ := hourly["wind_speed_10m"].([]interface{})
		windDirs, _ := hourly["wind_direction_10m"].([]interface{})
		precipProbs, _ := hourly["precipitation_probability"].([]interface{})
		weatherCodes, _ := hourly["weather_code"].([]interface{})

		fmt.Println("=== Hourly Forecast (5:00 AM - 7:00 PM) ===")
		fmt.Printf("%-8s | %-15s | %-12s | %-8s | %-12s | %-6s | %-20s\n",
			"Hour", "Condition", "Temp (Feels)", "Humidity", "Wind", "Precip", "Exertion (RPE)")
		fmt.Println(strings.Repeat("-", 100))

		for i := 0; i < len(times); i++ {
			tStr, _ := times[i].(string)
			tVal, err := time.Parse("2006-01-02T15:04", tStr)
			if err != nil {
				tVal, _ = time.Parse(time.RFC3339, tStr)
			}

			// Filter daylight hours
			if tVal.Hour() < 5 || tVal.Hour() > 19 {
				continue
			}

			wCodeFloat, _ := weatherCodes[i].(float64)
			wCode := int(wCodeFloat)
			cond := client.WMOCodes[wCode]
			condStr := fmt.Sprintf("%s %s", cond[1], cond[0])
			if cond[0] == "" {
				condStr = "❓ Unknown"
			}

			temp, _ := temps[i].(float64)
			appTemp, _ := apparentTemps[i].(float64)
			hum, _ := humidities[i].(float64)
			windSp, _ := windSpeeds[i].(float64)
			windD, _ := windDirs[i].(float64)
			precip, _ := precipProbs[i].(float64)

			tF := temp
			wMPH := windSp
			if cfg.DistanceUnit == "km" {
				tF = temp*9.0/5.0 + 32.0
				wMPH = windSp * 0.621371
			}

			rpeScore, rpeLabel, _ := client.CalculatePerceivedExertion(tF, hum, wMPH)
			rpeStr := fmt.Sprintf("%s (Score: %.1f)", rpeLabel, rpeScore)

			hourDisp := strings.TrimPrefix(tVal.Format("03:04 PM"), "0")
			tempDisp := fmt.Sprintf("%d%s (%d%s)", int(math.Round(temp)), tempSuffix, int(math.Round(appTemp)), tempSuffix)
			windDisp := fmt.Sprintf("%d %s %s", int(math.Round(windSp)), windSuffix, client.DegreesToCardinal(windD))

			fmt.Printf("%-8s | %-15s | %-12s | %-8d%% | %-12s | %-5d%% | %-20s\n",
				hourDisp, condStr, tempDisp, int(math.Round(hum)), windDisp, int(math.Round(precip)), rpeStr)
		}
	} else {
		// Daily summaries
		daily, _ := weatherData["daily"].(map[string]interface{})
		times, _ := daily["time"].([]interface{})
		codes, _ := daily["weather_code"].([]interface{})
		tempsMax, _ := daily["temperature_2m_max"].([]interface{})
		tempsMin, _ := daily["temperature_2m_min"].([]interface{})
		sunrises, _ := daily["sunrise"].([]interface{})
		sunsets, _ := daily["sunset"].([]interface{})
		windsMax, _ := daily["wind_speed_10m_max"].([]interface{})

		fmt.Println("=== Multi-Day Weather Outlook ===")
		fmt.Printf("%-12s | %-15s | %-12s | %-8s | %-15s | %-15s\n",
			"Date", "Condition", "Temp Range", "Max Wind", "Sun Times", "Best Run Window")
		fmt.Println(strings.Repeat("-", 90))

		// Cache hourly mapping to compute best window
		hourly, _ := weatherData["hourly"].(map[string]interface{})
		hTimes, _ := hourly["time"].([]interface{})
		hTemps, _ := hourly["temperature_2m"].([]interface{})
		hHumidities, _ := hourly["relative_humidity_2m"].([]interface{})
		hWindSpeeds, _ := hourly["wind_speed_10m"].([]interface{})

		for i := 0; i < len(times); i++ {
			dStr, _ := times[i].(string)
			dVal, _ := time.Parse("2006-01-02", dStr)

			wCodeFloat, _ := codes[i].(float64)
			wCode := int(wCodeFloat)
			cond := client.WMOCodes[wCode]
			condStr := fmt.Sprintf("%s %s", cond[1], cond[0])
			if cond[0] == "" {
				condStr = "❓ Unknown"
			}

			tMax, _ := tempsMax[i].(float64)
			tMin, _ := tempsMin[i].(float64)
			tempStr := fmt.Sprintf("%d - %d %s", int(math.Round(tMin)), int(math.Round(tMax)), tempSuffix)

			wMax, _ := windsMax[i].(float64)
			windStr := fmt.Sprintf("%d %s", int(math.Round(wMax)), windSuffix)

			sunriseStr := "-"
			sunsetStr := "-"
			if i < len(sunrises) && i < len(sunsets) {
				srStr, _ := sunrises[i].(string)
				ssStr, _ := sunsets[i].(string)
				srTime, err1 := time.Parse("2006-01-02T15:04", srStr)
				ssTime, err2 := time.Parse("2006-01-02T15:04", ssStr)
				if err1 == nil && err2 == nil {
					sunriseStr = strings.TrimPrefix(srTime.Format("03:04PM"), "0")
					sunsetStr = strings.TrimPrefix(ssTime.Format("03:04PM"), "0")
				}
			}
			sunTimesStr := fmt.Sprintf("%s - %s", sunriseStr, sunsetStr)

			// Find best window for this date
			bestWindow := "No window"
			bestRPEScore := math.MaxFloat64
			for hIdx := 0; hIdx < len(hTimes); hIdx++ {
				htStr, _ := hTimes[hIdx].(string)
				htVal, err := time.Parse("2006-01-02T15:04", htStr)
				if err != nil {
					htVal, _ = time.Parse(time.RFC3339, htStr)
				}

				if htVal.Format("2006-01-02") != dStr {
					continue
				}

				// Check active hours
				if htVal.Hour() < 5 || htVal.Hour() > 19 {
					continue
				}

				hTemp, _ := hTemps[hIdx].(float64)
				hHum, _ := hHumidities[hIdx].(float64)
				hWind, _ := hWindSpeeds[hIdx].(float64)

				tF := hTemp
				wMPH := hWind
				if cfg.DistanceUnit == "km" {
					tF = hTemp*9.0/5.0 + 32.0
					wMPH = hWind * 0.621371
				}

				rpeScore, rpeLabel, _ := client.CalculatePerceivedExertion(tF, hHum, wMPH)
				if rpeScore < bestRPEScore {
					bestRPEScore = rpeScore
					bestWindow = fmt.Sprintf("%s (%s)", strings.TrimPrefix(htVal.Format("03:04 PM"), "0"), rpeLabel)
				}
			}

			fmt.Printf("%-12s | %-15s | %-12s | %-8s | %-15s | %-15s\n",
				dVal.Format("Mon, Jan 02"), condStr, tempStr, windStr, sunTimesStr, bestWindow)
		}
	}
}
