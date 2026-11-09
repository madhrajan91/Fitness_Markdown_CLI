package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
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
	serverPort      int
	serverHost      string
	serverNoBrowser bool
	DashboardAssets embed.FS
)

var serverCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start a local web server to display the running CLI dashboard",
	Run:   runServer,
}

func init() {
	serverCmd.Flags().IntVar(&serverPort, "port", 8080, "Port to run the dashboard server on")
	serverCmd.Flags().StringVar(&serverHost, "host", "127.0.0.1", "Address to bind the server to")
	serverCmd.Flags().BoolVar(&serverNoBrowser, "no-browser", false, "Don't open the dashboard in a browser on start")
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
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "ok",
			"version":       Version,
			"api_version":   apiVersion,
			"distance_unit": cfg.DistanceUnit,
		})
	})

	// Sync: POST /api/sync?days=N (or start=&end=) starts a background sync;
	// GET /api/sync/status reports its progress.
	syncs := newSyncJob()
	http.HandleFunc("/api/sync", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "use POST to start a sync", http.StatusMethodNotAllowed)
			return
		}

		q := r.URL.Query()
		days := 7
		if s := q.Get("days"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid days %q", s), http.StatusBadRequest)
				return
			}
			days = n
		}
		start, end, err := resolveSyncRange(days, q.Get("start"), q.Get("end"), time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Reload the config so logins redone with `running-cli setup` are used.
		syncCfg, err := config.LoadConfig()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !syncCfg.IsGarminConfigured() && !syncCfg.IsStravaConfigured() {
			http.Error(w, "neither Garmin nor Strava is configured: run 'running-cli setup'", http.StatusUnauthorized)
			return
		}

		status, err := syncs.start(start, end, func(logf func(string, ...interface{})) (*activitySyncResult, error) {
			return syncActivities(syncCfg, database, start, end, logf)
		})
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}
		json.NewEncoder(w).Encode(status)
	})

	http.HandleFunc("/api/sync/status", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(syncs.snapshot())
	})

	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w, r)
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
		enableCors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodPost {
			type manualActivityRequest struct {
				Title           string   `json:"title"`
				Date            string   `json:"date"` // YYYY-MM-DD HH:MM
				Sport           string   `json:"sport"`
				Distance        float64  `json:"distance"` // in display units (miles or km)
				DurationHours   int      `json:"duration_hours"`
				DurationMinutes int      `json:"duration_minutes"`
				DurationSeconds int      `json:"duration_seconds"`
				AvgHR           *float64 `json:"avg_hr"`
				MaxHR           *float64 `json:"max_hr"`
				Elevation       float64  `json:"elevation"` // in display units (feet or meters)
				Location        string   `json:"location"`
				Description     string   `json:"description"`
				IsRace          bool     `json:"is_race"`
				Links           string   `json:"links"`
			}

			var req manualActivityRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if req.Title == "" || req.Date == "" || req.Sport == "" {
				http.Error(w, "Missing required fields: title, date, or sport", http.StatusBadRequest)
				return
			}

			var actTime time.Time
			var err error
			actTime, err = time.ParseInLocation("2006-01-02 15:04", req.Date, time.Local)
			if err != nil {
				actTime, err = time.ParseInLocation("2006-01-02T15:04", req.Date, time.Local)
				if err != nil {
					http.Error(w, "Invalid date format. Use YYYY-MM-DD HH:MM", http.StatusBadRequest)
					return
				}
			}

			distanceMeters := 0.0
			if cfg.DistanceUnit == "km" {
				distanceMeters = req.Distance * 1000.0
			} else {
				distanceMeters = req.Distance / 0.000621371
			}

			durationSeconds := float64(req.DurationHours*3600 + req.DurationMinutes*60 + req.DurationSeconds)

			elevationMeters := 0.0
			if cfg.DistanceUnit == "km" {
				elevationMeters = req.Elevation
			} else {
				elevationMeters = req.Elevation / 3.28084
			}

			// Automatically mark as Trail Run if run and elevation gain >= 1200 ft (365.76m)
			sport := req.Sport
			if sport == "Run" && elevationMeters >= 365.76 {
				sport = "Trail Run"
			}

			// Automatically mark as race if title contains "race"
			isRace := req.IsRace
			if strings.Contains(strings.ToLower(req.Title), "race") {
				isRace = true
			}

			activityID := time.Now().UnixNano()
			act := &models.Activity{
				ID:                  activityID,
				Provider:            "local",
				Sport:               sport,
				Title:               req.Title,
				StartTime:           actTime,
				DistanceMeters:      distanceMeters,
				DurationSeconds:     durationSeconds,
				AvgHR:               req.AvgHR,
				MaxHR:               req.MaxHR,
				ElevationGainMeters: elevationMeters,
				Description:         req.Description,
				LocationName:        req.Location,
				IsRace:              isRace,
				Links:               req.Links,
				RawData:             "{}",
			}

			if err := database.InsertOrUpdateActivity(act); err != nil {
				http.Error(w, fmt.Sprintf("Failed to save activity: %v", err), http.StatusInternalServerError)
				return
			}

			// Re-run the merger for this week and save
			mon := storage.GetMondayOfWeek(actTime)
			weekStart := time.Date(mon.Year(), mon.Month(), mon.Day(), 0, 0, 0, 0, time.Local)
			weekEnd := weekStart.AddDate(0, 0, 7).Add(-1 * time.Second)

			gRaw, sRaw, err := database.GetRawActivitiesForSync(weekStart, weekEnd)
			if err == nil {
				mergedList := merger.MergeActivities(gRaw, sRaw)
				database.ClearMergedActivitiesRange(weekStart, weekEnd)
				for _, ma := range mergedList {
					id, err := database.SaveMergedActivity(ma)
					if err == nil {
						ma.ID = id
					}
				}

				database.AutoLinkWorkouts()

				// Rewrite weekly summary and master lists
				weekActs, err := database.GetMergedActivitiesForRange(weekStart, weekEnd)
				if err == nil && len(weekActs) > 0 {
					vaultPath, err := cfg.GetVaultPath()
					if err == nil {
						storage.WriteWeeklyNote(vaultPath, cfg.ObsidianFolder, weekStart, weekActs, cfg.DistanceUnit)

						allRaces, err := database.GetMergedRaces(nil)
						if err == nil {
							storage.WriteRaceNotes(vaultPath, cfg.ObsidianFolder, allRaces, cfg.DistanceUnit)
						}
						allTrails, err := database.GetMergedTrailRuns(nil)
						if err == nil {
							storage.WriteTrailRunNotes(vaultPath, cfg.ObsidianFolder, allTrails, cfg.DistanceUnit)
						}
					}
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "id": activityID})
			return
		}

		// Handle GET: all activities, one week with ?week=YYYY-MM-DD, or the
		// activity with ?strava_id=N or ?garmin_id=N
		source, sourceID, bySource, err := parseSourceIDParam(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		week := r.URL.Query().Get("week")
		if bySource && week != "" {
			http.Error(w, "give either week or a source id, not both", http.StatusBadRequest)
			return
		}

		var activities []*models.MergedActivity
		if bySource {
			activities, err = database.GetMergedActivitiesBySourceID(source, sourceID)
		} else {
			rangeStart := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
			rangeEnd := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
			if week != "" {
				rangeStart, rangeEnd, err = parseWeekParam(week)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			activities, err = database.GetMergedActivitiesForRange(rangeStart, rangeEnd)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type viewActivity struct {
			ID                  int64               `json:"id"`
			Date                string              `json:"date"`
			Sport               string              `json:"sport"`
			Title               string              `json:"title"`
			DistanceMeters      float64             `json:"distance_meters"`
			DistanceDisplay     string              `json:"distance_display"`
			DurationSeconds     float64             `json:"duration_seconds"`
			DurationDisplay     string              `json:"duration_display"`
			PaceDisplay         string              `json:"pace_display"`
			ElevationGainMeters float64             `json:"elevation_gain_meters"`
			ElevationDisplay    string              `json:"elevation_display"`
			LocationName        string              `json:"location_name"`
			Latitude            *float64            `json:"latitude,omitempty"`
			Longitude           *float64            `json:"longitude,omitempty"`
			AvgHR               *float64            `json:"avg_hr"`
			MaxHR               *float64            `json:"max_hr"`
			GarminID            *int64              `json:"garmin_id,omitempty"`
			StravaID            *int64              `json:"strava_id,omitempty"`
			Sources             []string            `json:"sources"`
			SourceLinks         []models.SourceLink `json:"source_links"`
			IsRace              bool                `json:"is_race"`
			Links               string              `json:"links,omitempty"`
		}

		list := make([]viewActivity, 0)
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
				Latitude:            a.Latitude,
				Longitude:           a.Longitude,
				AvgHR:               a.AvgHR,
				MaxHR:               a.MaxHR,
				GarminID:            a.GarminID,
				StravaID:            a.StravaID,
				Sources:             a.Sources,
				SourceLinks:         a.SourceLinks(),
				IsRace:              a.IsRace,
				Links:               a.Links,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	http.HandleFunc("/api/plans", func(w http.ResponseWriter, r *http.Request) {
		enableCors(w, r)
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

		response := make([]planWithWorkouts, 0)
		for _, p := range plans {
			workouts, err := database.GetPlannedWorkoutsForPlan(p.ID)
			if err == nil {
				// Normalize status for missed workouts dynamically
				for _, w := range workouts {
					if w.Status == "Scheduled" && w.PlannedDate.Before(time.Now().Truncate(24*time.Hour)) {
						w.Status = "Missed"
					}
				}
				if workouts == nil {
					workouts = make([]*models.PlannedWorkout, 0)
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
		enableCors(w, r)
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
		enableCors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		wq, err := parseWeatherQuery(r.URL.Query(), time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		lat, lon, locName, err := resolveWeatherLocation(cfg, wq.Location, wq.Lat, wq.Lon, wq.HasCoords)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		forecast, err := client.FetchWeather(lat, lon, wq.Start, wq.Start.AddDate(0, 0, wq.Days-1), cfg.DistanceUnit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		forecast["location"] = map[string]interface{}{"name": locName, "lat": lat, "lon": lon}
		forecast["running"] = buildRunningWeather(forecast, cfg.DistanceUnit)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(forecast)
	})

	http.Handle("/plugin/", pluginRoutes{
		Unit:       cfg.DistanceUnit,
		Now:        time.Now,
		Activities: database.GetMergedActivitiesForRange,
		Weather: func(today time.Time) (runningWeather, string, error) {
			lat, lon, location, err := resolveWeatherLocation(cfg, "", 0, 0, false)
			if err != nil {
				return runningWeather{}, "", err
			}
			start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
			forecast, err := client.FetchWeather(lat, lon, start, start.AddDate(0, 0, pluginWeatherDays-1), cfg.DistanceUnit)
			if err != nil {
				return runningWeather{}, "", err
			}
			return buildRunningWeather(forecast, cfg.DistanceUnit), location, nil
		},
	}.handler())

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

	addr := net.JoinHostPort(serverHost, strconv.Itoa(serverPort))
	fmt.Printf("\n\033[1;32mStarting dashboard server on http://%s ...\033[0m\n", addr)
	fmt.Println("Press Ctrl+C to terminate.")

	if !serverNoBrowser {
		go func() {
			time.Sleep(1 * time.Second)
			launchBrowser("http://" + addr)
		}()
	}

	err = http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}

// enableCors lets local pages (the Vite dev server) call the API. Other
// origins get no CORS headers, so web pages can't read my data through the browser.
func enableCors(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if !allowedOrigin(origin) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
