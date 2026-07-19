package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"running_cli/pkg/client"
	"running_cli/pkg/config"
	"running_cli/pkg/db"
	"running_cli/pkg/merger"
	"running_cli/pkg/models"
	"running_cli/pkg/storage"

	"github.com/spf13/cobra"
)

var (
	syncDays      int
	syncStartDate string
	syncEndDate   string
	syncRacesOnly bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Fetch and sync activities, weather, and schedules",
	Run:   runSync,
}

func init() {
	syncCmd.Flags().IntVar(&syncDays, "days", 7, "Number of days to sync from today")
	syncCmd.Flags().StringVar(&syncStartDate, "start-date", "", "Sync start date YYYY-MM-DD (overrides --days)")
	syncCmd.Flags().StringVar(&syncEndDate, "end-date", "", "Sync end date YYYY-MM-DD (defaults to today)")
	syncCmd.Flags().BoolVar(&syncRacesOnly, "races-only", false, "Only sync races and skip generating weekly summaries")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	vaultPath, err := cfg.GetVaultPath()
	if err != nil {
		fmt.Println("Error: Obsidian vault path is not configured. Run 'running-cli setup' first.")
		return
	}

	if !cfg.IsGarminConfigured() && !cfg.IsStravaConfigured() {
		fmt.Println("Error: Neither Garmin nor Strava is configured. Run 'running-cli setup' first.")
		return
	}

	// 1. Resolve date range
	var start, end time.Time
	today := time.Now()
	todayMidnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)

	if syncEndDate != "" {
		end, err = time.Parse("2006-01-02", syncEndDate)
		if err != nil {
			fmt.Printf("Error: Invalid end date format. Use YYYY-MM-DD.\n")
			return
		}
		// Make sure it includes the full end date (end of day)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, time.Local)
	} else {
		end = time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, time.Local)
	}

	if syncStartDate != "" {
		start, err = time.Parse("2006-01-02", syncStartDate)
		if err != nil {
			fmt.Printf("Error: Invalid start date format. Use YYYY-MM-DD.\n")
			return
		}
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local)
	} else {
		start = todayMidnight.AddDate(0, 0, -syncDays)
	}

	fmt.Printf("Syncing activities from %s to %s...\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

	// 2. Open DB
	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	// 3. Fetch Garmin
	if cfg.IsGarminConfigured() {
		fmt.Println("Fetching Garmin activities...")
		garminActs, err := client.FetchGarminActivities(cfg.Garmin.Email, "", start, end)
		if err != nil {
			fmt.Printf("  \033[0;31mGarmin fetch warning: %v\033[0m\n", err)
		} else {
			fmt.Printf("  Fetched %d Garmin activities.\n", len(garminActs))
			for _, a := range garminActs {
				if err := database.InsertOrUpdateActivity(a); err != nil {
					fmt.Printf("  Error saving Garmin activity %d: %v\n", a.ID, err)
				}
			}
		}
	}

	// 4. Fetch Strava
	if cfg.IsStravaConfigured() {
		fmt.Println("Fetching Strava activities...")
		stravaActs, err := client.FetchStravaActivities(cfg.Strava.ClientID, cfg.Strava.ClientSecret, cfg.Strava.RefreshToken, start, end)
		if err != nil {
			fmt.Printf("  \033[0;31mStrava fetch warning: %v\033[0m\n", err)
		} else {
			fmt.Printf("  Fetched %d Strava activities.\n", len(stravaActs))
			for _, a := range stravaActs {
				if err := database.InsertOrUpdateActivity(a); err != nil {
					fmt.Printf("  Error saving Strava activity %d: %v\n", a.ID, err)
				}
			}
		}
	}

	// 5. Query all raw activities from DB and merge
	gRaw, sRaw, err := database.GetRawActivitiesForSync(start, end)
	if err != nil {
		fmt.Printf("Error reading raw activities from DB: %v\n", err)
		return
	}

	mergedList := merger.MergeActivities(gRaw, sRaw)
	fmt.Printf("Merged into %d unique activities.\n", len(mergedList))

	// 6. Clear and save merged activities in range
	if err := database.ClearMergedActivitiesRange(start, end); err != nil {
		fmt.Printf("Error clearing merged activities range: %v\n", err)
		return
	}

	for _, ma := range mergedList {
		id, err := database.SaveMergedActivity(ma)
		if err != nil {
			fmt.Printf("Error saving merged activity: %v\n", err)
		} else {
			ma.ID = id
		}
	}

	// 7. Auto link planned workouts with synced actual runs
	linkedCount, err := database.AutoLinkWorkouts()
	if err != nil {
		fmt.Printf("Warning: Failed to auto-link workouts: %v\n", err)
	} else if linkedCount > 0 {
		fmt.Printf("Auto-linked %d synced activities to scheduled training plans.\n", linkedCount)
	}

	// 8. Generate notes
	if syncRacesOnly {
		fmt.Println("Syncing races only...")
		allRaces, err := database.GetMergedRaces(nil)
		if err != nil {
			fmt.Printf("Error loading races: %v\n", err)
		} else {
			paths, err := storage.WriteRaceNotes(vaultPath, cfg.ObsidianFolder, allRaces, cfg.DistanceUnit)
			if err != nil {
				fmt.Printf("Error writing race notes: %v\n", err)
			} else {
				fmt.Printf("Updated %d race notes.\n", len(paths))
			}
		}
	} else {
		// Group merged activities by week (Monday date)
		activitiesByWeek := make(map[string][]*models.MergedActivity)
		for _, ma := range mergedList {
			mon := storage.GetMondayOfWeek(ma.StartTime).Format("2006-01-02")
			activitiesByWeek[mon] = append(activitiesByWeek[mon], ma)
		}

		fmt.Println("Writing weekly activity notes...")
		for monStr, acts := range activitiesByWeek {
			mon, _ := time.Parse("2006-01-02", monStr)
			// Get all activities for this week from DB to make sure we don't drop activities
			// that were synced in previous runs
			weekStart := mon
			weekEnd := mon.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			weekActs, err := database.GetMergedActivitiesForRange(weekStart, weekEnd)
			if err == nil && len(weekActs) > 0 {
				acts = weekActs
			}

			path, err := storage.WriteWeeklyNote(vaultPath, cfg.ObsidianFolder, mon, acts, cfg.DistanceUnit)
			if err != nil {
				fmt.Printf("  Error writing week of %s: %v\n", monStr, err)
			} else {
				fmt.Printf("  Created/Updated: %s\n", path)
			}
		}

		// Write master race list
		allRaces, err := database.GetMergedRaces(nil)
		if err == nil {
			paths, err := storage.WriteRaceNotes(vaultPath, cfg.ObsidianFolder, allRaces, cfg.DistanceUnit)
			if err != nil {
				fmt.Printf("  Error writing race notes: %v\n", err)
			} else {
				fmt.Printf("  Updated %d race files.\n", len(paths))
			}
		}

		// Write master trail run list
		allTrails, err := database.GetMergedTrailRuns(nil)
		if err == nil {
			paths, err := storage.WriteTrailRunNotes(vaultPath, cfg.ObsidianFolder, allTrails, cfg.DistanceUnit)
			if err != nil {
				fmt.Printf("  Error writing trail run notes: %v\n", err)
			} else {
				fmt.Printf("  Updated %d trail run files.\n", len(paths))
			}
		}

		// Fetch and write weather
		fmt.Println("Fetching and updating weekly weather...")
		configMap := map[string]interface{}{
			"weather_lat":      cfg.WeatherLat,
			"weather_lon":      cfg.WeatherLon,
			"weather_location": cfg.WeatherLocation,
		}
		var finalLat, finalLon float64
		if cfg.WeatherLat != nil {
			finalLat = *cfg.WeatherLat
		}
		if cfg.WeatherLon != nil {
			finalLon = *cfg.WeatherLon
		}
		if finalLat == 0.0 && finalLon == 0.0 {
			res, err := client.GeolocateByIP()
			if err == nil {
				finalLat = res["lat"].(float64)
				finalLon = res["lon"].(float64)
				configMap["weather_lat"] = finalLat
				configMap["weather_lon"] = finalLon
				configMap["weather_location"] = res["name"].(string)
			}
		}

		path, err := storage.WriteWeeklyWeatherNote(vaultPath, cfg.ObsidianFolder, configMap, cfg.DistanceUnit)
		if err != nil {
			fmt.Printf("  Error updating weather note: %v\n", err)
		} else {
			fmt.Printf("  Updated weather forecast: %s\n", path)
		}

		// Sync future races schedule if zipcode is configured (from geocoding)
		zipcode := "02478" // default Belmont
		locName := cfg.WeatherLocation
		zipRegex := regexpMustCompile(`\b\d{5}\b`)
		zipMatch := zipRegex.FindString(locName)
		if zipMatch != "" {
			zipcode = zipMatch
		}

		fmt.Println("Syncing upcoming races schedule...")
		syncFutureRacesRun(zipcode, 50, 5, vaultPath, cfg.ObsidianFolder, cfg.DistanceUnit)
	}

	fmt.Println("\n\033[1;32mSync complete!\033[0m")
}

// helper wrapper to avoid circular dependency
func regexpMustCompile(str string) *regexpWrapper {
	r := regexp.MustCompile(str)
	return &regexpWrapper{r}
}

type regexpWrapper struct {
	*regexp.Regexp
}

func (rw *regexpWrapper) FindString(s string) string {
	return rw.Regexp.FindString(s)
}

func syncFutureRacesRun(zipcode string, radius, months int, vaultPath, folder, unit string) {
	currentDT := time.Now()
	startDate := time.Date(currentDT.Year(), currentDT.Month(), currentDT.Day(), 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 0, months*30)

	fmt.Printf("  Resolving races within %d miles of ZIP %s...\n", radius, zipcode)
	races, err := client.FetchFutureRaces(zipcode, radius, startDate, endDate)
	if err != nil {
		fmt.Printf("  Error fetching upcoming races: %v\n", err)
		return
	}

	// Insert into DB
	dbConn, err := db.OpenDB()
	if err == nil {
		defer dbConn.Close()
		dbConn.SaveFutureRaces(races)
	}

	obsidianDir := filepath.Join(vaultPath, folder)
	futureRacesPath := filepath.Join(obsidianDir, "FutureRaces.md")

	if err := os.MkdirAll(obsidianDir, 0755); err != nil {
		fmt.Printf("  Error creating future races directory: %v\n", err)
		return
	}

	fileContent := ""
	if _, err := os.Stat(futureRacesPath); err == nil {
		data, _ := os.ReadFile(futureRacesPath)
		fileContent = string(data)
	} else {
		fileContent = `---\ntype: race_schedule\nlast_updated: ""\n---\n\n# 🏃‍♂️ Future Races\n\nCurated list of upcoming races found by the Race Finder agent.\n\n%% START_FUTURE_RACES %%\n%% END_FUTURE_RACES %%\n\n## 📝 Custom Notes & Goals\n*Write your race targets, training schedules, or notes here. This section will be preserved when running the sync again.*\n`
	}

	// Build markdown table
	tableHeaders := []string{"Date", "Sport/Distance", "Race Name", "Location", "Links"}
	var tableRows []string
	tableRows = append(tableRows, "| "+strings.Join(tableHeaders, " | ")+" |")
	tableRows = append(tableRows, "| "+strings.Join([]string{"---", "---", "---", "---", "---"}, " | ")+" |")

	if len(races) == 0 {
		tableRows = append(tableRows, "| - | - | No upcoming races found. | - | - |")
	} else {
		for _, r := range races {
			dateStr := r.RaceDate.Format("Mon, Jan 02, 2006")
			linkStr := fmt.Sprintf("[Website](%s)", r.WebsiteURL)
			if r.WebsiteURL == "" {
				linkStr = "-"
			}
			row := []string{dateStr, r.DistanceDisplay, r.Title, r.Location, linkStr}
			tableRows = append(tableRows, "| "+strings.Join(row, " | ")+" |")
		}
	}

	tableContent := strings.Join(tableRows, "\n")
	frontmatter := map[string]interface{}{
		"type":         "race_schedule",
		"last_updated": currentDT.Format("2006-01-02 15:04:05"),
	}

	updatedContent := storage.SafeUpdateMarkdown(
		fileContent,
		frontmatter,
		tableContent,
		"%% START_FUTURE_RACES %%",
		"%% END_FUTURE_RACES %%",
		"\n# Future Races\n\n{replacement}\n\n{body}",
	)

	err = os.WriteFile(futureRacesPath, []byte(updatedContent), 0644)
	if err != nil {
		fmt.Printf("  Error saving FutureRaces.md: %v\n", err)
	} else {
		fmt.Printf("  Synced %d future races to: %s\n", len(races), futureRacesPath)
	}
}
