package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"running_cli/pkg/client"
	"running_cli/pkg/config"
	"running_cli/pkg/db"
	"running_cli/pkg/models"
	"running_cli/pkg/storage"

	"github.com/spf13/cobra"
)

var (
	serverPort      int
	DashboardAssets embed.FS
)

var serverCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start a local web server to display the running CLI dashboard",
	Run:   runServer,
}

func init() {
	serverCmd.Flags().IntVar(&serverPort, "port", 8080, "Port to run the dashboard server on")
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	// API Handlers
	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		startEpoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		endEpoch := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

		activities, err := database.GetMergedActivitiesForRange(startEpoch, endEpoch)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		totalDist := 0.0
		totalDur := 0.0
		for _, a := range activities {
			totalDist += a.DistanceMeters
			totalDur += a.DurationSeconds
		}

		// Convert to miles/km
		distVal := totalDist * 0.000621371
		distUnit := "miles"
		if cfg.DistanceUnit == "km" {
			distVal = totalDist * 0.001
			distUnit = "km"
		}

		// Training compliance rates
		plans, _ := database.GetActivePlans()
		completedWorkouts := 0
		totalWorkouts := 0
		complianceRate := 0.0

		if len(plans) > 0 {
			workouts, err := database.GetPlannedWorkoutsForPlan(plans[0].ID)
			if err == nil {
				totalWorkouts = len(workouts)
				scheduled := 0
				for _, w := range workouts {
					if w.Status == "Completed" {
						completedWorkouts++
					} else if w.Status == "Scheduled" && w.PlannedDate.After(time.Now()) {
						scheduled++
					}
				}
				passed := totalWorkouts - scheduled
				if passed > 0 {
					complianceRate = (float64(completedWorkouts) / float64(passed)) * 100.0
				}
			}
		}

		nextRaceDays := -1
		nextRaceTitle := "None Scheduled"
		races, err := database.GetMergedRaces(nil)
		if err == nil {
			for _, r := range races {
				if r.Date.After(time.Now()) || r.Date.Equal(time.Now().Truncate(24*time.Hour)) {
					nextRaceTitle = r.Title
					nextRaceDays = int(time.Until(r.Date).Hours() / 24.0)
					break
				}
			}
		}

		response := map[string]interface{}{
			"activity_count":       len(activities),
			"total_distance":       distVal,
			"distance_unit":        distUnit,
			"total_duration_hours": totalDur / 3600.0,
			"active_plans":         len(plans),
			"completed_workouts":   completedWorkouts,
			"total_workouts":       totalWorkouts,
			"compliance_rate":      complianceRate,
			"next_race_title":      nextRaceTitle,
			"next_race_days":       nextRaceDays,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/api/activities", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		startEpoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		endEpoch := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

		activities, err := database.GetMergedActivitiesForRange(startEpoch, endEpoch)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert distances/paces to display formats
		type viewActivity struct {
			ID                  int64    `json:"id"`
			Date                string   `json:"date"`
			Sport               string   `json:"sport"`
			Title               string   `json:"title"`
			DistanceMeters      float64  `json:"distance_meters"`
			DistanceDisplay     string   `json:"distance_display"`
			DurationSeconds     float64  `json:"duration_seconds"`
			DurationDisplay     string   `json:"duration_display"`
			PaceDisplay         string   `json:"pace_display"`
			ElevationGainMeters float64  `json:"elevation_gain_meters"`
			ElevationDisplay    string   `json:"elevation_display"`
			LocationName        string   `json:"location_name"`
			AvgHR               *float64 `json:"avg_hr"`
			MaxHR               *float64 `json:"max_hr"`
			Sources             []string `json:"sources"`
			IsRace              bool     `json:"is_race"`
		}

		var list []viewActivity
		for i := len(activities) - 1; i >= 0; i-- {
			a := activities[i]
			list = append(list, viewActivity{
				ID:                  a.ID,
				Date:                a.StartTime.Format("2006-01-02 15:04"),
				Sport:               a.Sport,
				Title:               a.Title,
				DistanceMeters:      a.DistanceMeters,
				DistanceDisplay:     storage.FormatDistance(a.Sport, a.DistanceMeters, cfg.DistanceUnit),
				DurationSeconds:     a.DurationSeconds,
				DurationDisplay:     storage.FormatDurationCompact(a.DurationSeconds),
				PaceDisplay:         storage.FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, cfg.DistanceUnit),
				ElevationGainMeters: a.ElevationGainMeters,
				ElevationDisplay:    storage.FormatElevation(a.ElevationGainMeters, cfg.DistanceUnit),
				LocationName:        a.LocationName,
				AvgHR:               a.AvgHR,
				MaxHR:               a.MaxHR,
				Sources:             a.Sources,
				IsRace:              a.IsRace,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	http.HandleFunc("/api/plans", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		plans, err := database.GetActivePlans()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type planWithWorkouts struct {
			Plan     *models.TrainingPlan     `json:"plan"`
			Workouts []*models.PlannedWorkout `json:"workouts"`
		}

		var response []planWithWorkouts
		for _, p := range plans {
			workouts, err := database.GetPlannedWorkoutsForPlan(p.ID)
			if err == nil {
				// Normalize status for missed workouts dynamically
				for _, w := range workouts {
					if w.Status == "Scheduled" && w.PlannedDate.Before(time.Now().Truncate(24*time.Hour)) {
						w.Status = "Missed"
					}
				}
				response = append(response, planWithWorkouts{
					Plan:     p,
					Workouts: workouts,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/api/races", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		races, err := database.GetFutureRaces()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(races)
	})

	http.HandleFunc("/api/weather", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		var lat, lon float64
		if cfg.WeatherLat != nil {
			lat = *cfg.WeatherLat
		}
		if cfg.WeatherLon != nil {
			lon = *cfg.WeatherLon
		}

		if lat == 0.0 && lon == 0.0 {
			res, err := client.GeolocateByIP()
			if err == nil {
				lat = res["lat"].(float64)
				lon = res["lon"].(float64)
			}
		}

		today := time.Now()
		forecast, err := client.FetchWeather(lat, lon, today, today.AddDate(0, 0, 4), cfg.DistanceUnit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(forecast)
	})

	// Static Webapp Frontend File Server (SPA router fallback using embedded files)
	subFS, err := fs.Sub(DashboardAssets, "dashboard/dist")
	if err != nil {
		fmt.Printf("Warning: Failed to load embedded assets: %v\n", err)
	}

	fsServer := http.FileServer(http.FS(subFS))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			return
		}
		// If path doesn't contain an extension (is a route like /activities), serve index.html
		if !strings.Contains(filepath.Base(r.URL.Path), ".") {
			data, err := fs.ReadFile(subFS, "index.html")
			if err != nil {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		fsServer.ServeHTTP(w, r)
	})

	fmt.Printf("\n\033[1;32mStarting dashboard server on http://localhost:%d ...\033[0m\n", serverPort)
	fmt.Println("Press Ctrl+C to terminate.")
	
	// Launch browser automatically (optional default browser launch helper)
	go func() {
		time.Sleep(1 * time.Second)
		launchBrowser(fmt.Sprintf("http://localhost:%d", serverPort))
	}()

	err = http.ListenAndServe(fmt.Sprintf(":%d", serverPort), nil)
	if err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}

func enableCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

