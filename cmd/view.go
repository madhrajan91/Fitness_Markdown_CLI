package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"running_cli/pkg/config"
	"running_cli/pkg/db"
	"running_cli/pkg/models"
	"running_cli/pkg/storage"

	"github.com/spf13/cobra"
)

var (
	viewWeek      string
	racesStart    string
	trailRunsStart string
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View merged activities for a specific week from local cache",
	Run:   runView,
}

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Select a merged activity and open it in the web browser",
	Run:   runOpen,
}

var racesCmd = &cobra.Command{
	Use:   "races",
	Short: "View all cached races in a terminal table",
	Run:   runRaces,
}

var trailRunsCmd = &cobra.Command{
	Use:   "trail-runs",
	Short: "View all cached trail runs in a terminal table",
	Run:   runTrailRuns,
}

func init() {
	viewCmd.Flags().StringVar(&viewWeek, "week", "", "Date in the target week (YYYY-MM-DD), defaults to today")
	racesCmd.Flags().StringVar(&racesStart, "start-date", "", "Filter races starting from this date (YYYY-MM-DD)")
	trailRunsCmd.Flags().StringVar(&trailRunsStart, "start-date", "", "Filter trail runs starting from this date (YYYY-MM-DD)")
	openCmd.Flags().StringVar(&viewWeek, "week", "", "Date in the target week (YYYY-MM-DD), defaults to today")

	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(racesCmd)
	rootCmd.AddCommand(trailRunsCmd)
}

func runView(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	var targetDate time.Time
	if viewWeek != "" {
		targetDate, err = time.Parse("2006-01-02", viewWeek)
		if err != nil {
			fmt.Println("Error: Invalid week date format. Use YYYY-MM-DD.")
			return
		}
	} else {
		targetDate = time.Now()
	}

	monday := storage.GetMondayOfWeek(targetDate)
	sunday := monday.AddDate(0, 0, 6)

	fmt.Printf("Retrieving cached activities for Week of %s (%s to %s)...\n\n",
		monday.Format("2006-01-02"), monday.Format("2006-01-02"), sunday.Format("2006-01-02"))

	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	// Query merged activities in range
	start := monday
	end := monday.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	activities, err := database.GetMergedActivitiesForRange(start, end)
	if err != nil {
		fmt.Printf("Error querying activities: %v\n", err)
		return
	}

	if len(activities) == 0 {
		fmt.Println("No activities recorded for this week.")
		return
	}

	// 1. Weekly totals
	type stats struct {
		distance float64
		duration float64
		count    int
	}
	totals := make(map[string]*stats)
	for _, a := range activities {
		if _, ok := totals[a.Sport]; !ok {
			totals[a.Sport] = &stats{}
		}
		totals[a.Sport].distance += a.DistanceMeters
		totals[a.Sport].duration += a.DurationSeconds
		totals[a.Sport].count++
	}

	fmt.Println("=== Weekly Breakdown ===")
	var sports []string
	for s := range totals {
		sports = append(sports, s)
	}
	sort.Strings(sports)

	for _, sport := range sports {
		st := totals[sport]
		emoji := storage.SportEmojis[sport]
		if emoji == "" {
			emoji = storage.SportEmojis["Other"]
		}
		distStr := storage.FormatDistance(sport, st.distance, cfg.DistanceUnit)
		durStr := storage.FormatDuration(st.duration)
		actLabel := "activity"
		if st.count != 1 {
			actLabel = "activities"
		}
		fmt.Printf("- %s %s: %s | %s | %d %s\n", emoji, sport, distStr, durStr, st.count, actLabel)
	}
	fmt.Println()

	// 2. Display Table
	fmt.Println("=== Activity Log ===")
	formatActivityTable(activities, cfg.DistanceUnit)
}

func runRaces(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	var filterDate *time.Time
	if racesStart != "" {
		t, err := time.Parse("2006-01-02", racesStart)
		if err != nil {
			fmt.Println("Error: Invalid start date format. Use YYYY-MM-DD.")
			return
		}
		filterDate = &t
	}

	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	races, err := database.GetMergedRaces(filterDate)
	if err != nil {
		fmt.Printf("Error loading races: %v\n", err)
		return
	}

	if len(races) == 0 {
		fmt.Println("No races cached.")
		return
	}

	fmt.Printf("Cached Races (%d total):\n\n", len(races))
	formatActivityTable(races, cfg.DistanceUnit)
}

func runTrailRuns(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	var filterDate *time.Time
	if trailRunsStart != "" {
		t, err := time.Parse("2006-01-02", trailRunsStart)
		if err != nil {
			fmt.Println("Error: Invalid start date format. Use YYYY-MM-DD.")
			return
		}
		filterDate = &t
	}

	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	trails, err := database.GetMergedTrailRuns(filterDate)
	if err != nil {
		fmt.Printf("Error loading trail runs: %v\n", err)
		return
	}

	if len(trails) == 0 {
		fmt.Println("No trail runs cached.")
		return
	}

	fmt.Printf("Cached Trail Runs (%d total):\n\n", len(trails))
	formatActivityTable(trails, cfg.DistanceUnit)
}

func runOpen(cmd *cobra.Command, args []string) {
	var targetDate time.Time
	var err error
	if viewWeek != "" {
		targetDate, err = time.Parse("2006-01-02", viewWeek)
		if err != nil {
			fmt.Println("Error: Invalid week date format. Use YYYY-MM-DD.")
			return
		}
	} else {
		targetDate = time.Now()
	}

	monday := storage.GetMondayOfWeek(targetDate)

	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	start := monday
	end := monday.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	activities, err := database.GetMergedActivitiesForRange(start, end)
	if err != nil {
		fmt.Printf("Error querying activities: %v\n", err)
		return
	}

	if len(activities) == 0 {
		fmt.Println("No activities cached for this week.")
		return
	}

	fmt.Println("Select an activity to open in browser:")
	for idx, a := range activities {
		dateStr := a.StartTime.Format("Mon, Jan 02")
		emoji := storage.SportEmojis[a.Sport]
		if emoji == "" {
			emoji = storage.SportEmojis["Other"]
		}
		sources := strings.Join(a.Sources, "/")
		fmt.Printf("[%d] %s - %s %s (%s)\n", idx+1, dateStr, emoji, a.Title, sources)
	}

	fmt.Print("\nEnter number: ")
	var choice int
	_, scanErr := fmt.Scanln(&choice)
	if scanErr != nil || choice < 1 || choice > len(activities) {
		fmt.Println("Invalid selection.")
		return
	}

	selected := activities[choice-1]
	urlStr := ""

	// Prefer opening Strava activity link
	if selected.StravaID != nil {
		urlStr = fmt.Sprintf("https://www.strava.com/activities/%d", *selected.StravaID)
	} else if selected.GarminID != nil {
		urlStr = fmt.Sprintf("https://connect.garmin.com/modern/activity/%d", *selected.GarminID)
	}

	if urlStr == "" {
		fmt.Println("No URL found for selected activity.")
		return
	}

	fmt.Printf("Launching browser: %s\n", urlStr)
	launchBrowser(urlStr)
}

func formatActivityTable(activities []*models.MergedActivity, unit string) {
	// Simple CLI table formatting
	fmt.Printf("%-12s | %-10s | %-30s | %-10s | %-8s | %-10s | %-8s | %-12s\n",
		"Date", "Sport", "Title", "Distance", "Duration", "Pace/Speed", "Elev", "Location")
	fmt.Println(strings.Repeat("-", 110))

	for _, a := range activities {
		dateStr := a.StartTime.Format("Jan 02, 15:04")
		sportStr := a.Sport
		titleStr := a.Title
		if len(titleStr) > 30 {
			titleStr = titleStr[:27] + "..."
		}
		distStr := storage.FormatDistance(a.Sport, a.DistanceMeters, unit)
		timeStr := storage.FormatDurationCompact(a.DurationSeconds)
		paceStr := storage.FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, unit)
		elevStr := storage.FormatElevation(a.ElevationGainMeters, unit)
		locStr := a.LocationName
		if len(locStr) > 12 {
			locStr = locStr[:9] + "..."
		}
		if locStr == "" {
			locStr = "-"
		}

		fmt.Printf("%-12s | %-10s | %-30s | %-10s | %-8s | %-10s | %-8s | %-12s\n",
			dateStr, sportStr, titleStr, distStr, timeStr, paceStr, elevStr, locStr)
	}
}

func launchBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // Linux
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Run()
}
