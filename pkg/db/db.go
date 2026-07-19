package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"running_cli/pkg/config"
	"running_cli/pkg/models"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

// OpenDB opens the SQLite database and runs migrations
func OpenDB() (*DB, error) {
	dbPath, err := config.GetDBPath()
	if err != nil {
		return nil, err
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate creates tables if they do not exist
func (db *DB) migrate() error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS activities (
			id INTEGER NOT NULL,
			provider TEXT NOT NULL,
			sport TEXT NOT NULL,
			title TEXT NOT NULL,
			start_time DATETIME NOT NULL,
			distance_meters REAL NOT NULL,
			duration_seconds REAL NOT NULL,
			avg_hr REAL,
			max_hr REAL,
			elevation_gain_meters REAL NOT NULL,
			description TEXT,
			location_name TEXT,
			latitude REAL,
			longitude REAL,
			is_race INTEGER NOT NULL,
			raw_data TEXT,
			PRIMARY KEY (id, provider)
		);`,
		`CREATE TABLE IF NOT EXISTS merged_activities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE NOT NULL,
			start_time DATETIME NOT NULL,
			sport TEXT NOT NULL,
			title TEXT NOT NULL,
			distance_meters REAL NOT NULL,
			duration_seconds REAL NOT NULL,
			avg_hr REAL,
			max_hr REAL,
			elevation_gain_meters REAL NOT NULL,
			garmin_id INTEGER,
			strava_id INTEGER,
			description TEXT,
			location_name TEXT,
			latitude REAL,
			longitude REAL,
			is_race INTEGER NOT NULL,
			sources TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS training_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			start_date DATE NOT NULL,
			end_date DATE NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS planned_workouts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_id INTEGER NOT NULL,
			planned_date DATE NOT NULL,
			sport TEXT NOT NULL,
			target_distance_miles REAL,
			target_duration_minutes REAL,
			target_intensity TEXT,
			target_notes TEXT,
			associated_activity_id INTEGER,
			status TEXT NOT NULL,
			FOREIGN KEY (plan_id) REFERENCES training_plans(id) ON DELETE CASCADE,
			FOREIGN KEY (associated_activity_id) REFERENCES merged_activities(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS future_races (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			race_date DATE NOT NULL,
			distance_display TEXT NOT NULL,
			location TEXT,
			website_url TEXT,
			latitude REAL,
			longitude REAL
		);`,
	}

	for _, query := range schema {
		_, err := db.conn.Exec(query)
		if err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}

// parseDate matches multiple date formats returned by SQLite
func parseDate(str string) time.Time {
	str = strings.TrimSpace(str)
	if len(str) >= 10 {
		// try simple date format
		t, err := time.Parse("2006-01-02", str[:10])
		if err == nil {
			return t
		}
	}
	// try ISO formats
	for _, fmtStr := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		t, err := time.Parse(fmtStr, str)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// InsertOrUpdateActivity inserts a raw provider activity
func (db *DB) InsertOrUpdateActivity(a *models.Activity) error {
	query := `INSERT OR REPLACE INTO activities (
		id, provider, sport, title, start_time, distance_meters, duration_seconds,
		avg_hr, max_hr, elevation_gain_meters, description, location_name,
		latitude, longitude, is_race, raw_data
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	startTimeStr := a.StartTime.Format(time.RFC3339)
	isRaceVal := 0
	if a.IsRace {
		isRaceVal = 1
	}

	_, err := db.conn.Exec(query,
		a.ID, a.Provider, a.Sport, a.Title, startTimeStr, a.DistanceMeters, a.DurationSeconds,
		a.AvgHR, a.MaxHR, a.ElevationGainMeters, a.Description, a.LocationName,
		a.Latitude, a.Longitude, isRaceVal, a.RawData,
	)
	return err
}

// GetActivitiesForRange retrieves raw activities for a provider in a date range
func (db *DB) GetActivitiesForRange(provider string, start, end time.Time) ([]*models.Activity, error) {
	query := `SELECT id, provider, sport, title, start_time, distance_meters, duration_seconds,
		avg_hr, max_hr, elevation_gain_meters, description, location_name,
		latitude, longitude, is_race, raw_data
		FROM activities
		WHERE provider = ? AND start_time >= ? AND start_time <= ?
		ORDER BY start_time ASC`

	rows, err := db.conn.Query(query, provider, start.Format("2006-01-02T00:00:00Z"), end.Format("2006-01-02T23:59:59Z"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []*models.Activity
	for rows.Next() {
		var a models.Activity
		var startTimeStr string
		var isRaceInt int

		err := rows.Scan(
			&a.ID, &a.Provider, &a.Sport, &a.Title, &startTimeStr, &a.DistanceMeters, &a.DurationSeconds,
			&a.AvgHR, &a.MaxHR, &a.ElevationGainMeters, &a.Description, &a.LocationName,
			&a.Latitude, &a.Longitude, &isRaceInt, &a.RawData,
		)
		if err != nil {
			return nil, err
		}

		t, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			// fallback parse
			t, _ = time.Parse("2006-01-02 15:04:05", startTimeStr)
		}
		a.StartTime = t
		a.IsRace = isRaceInt == 1

		activities = append(activities, &a)
	}

	return activities, nil
}

// ClearMergedActivitiesRange removes merged activities in the date range
func (db *DB) ClearMergedActivitiesRange(start, end time.Time) error {
	query := `DELETE FROM merged_activities WHERE date >= ? AND date <= ?`
	_, err := db.conn.Exec(query, start.Format("2006-01-02"), end.Format("2006-01-02"))
	return err
}

// SaveMergedActivity inserts a merged activity
func (db *DB) SaveMergedActivity(ma *models.MergedActivity) (int64, error) {
	query := `INSERT INTO merged_activities (
		date, start_time, sport, title, distance_meters, duration_seconds,
		avg_hr, max_hr, elevation_gain_meters, garmin_id, strava_id,
		description, location_name, latitude, longitude, is_race, sources
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	sourcesStr := strings.Join(ma.Sources, ",")
	isRaceVal := 0
	if ma.IsRace {
		isRaceVal = 1
	}

	res, err := db.conn.Exec(query,
		ma.Date.Format("2006-01-02"),
		ma.StartTime.Format(time.RFC3339),
		ma.Sport,
		ma.Title,
		ma.DistanceMeters,
		ma.DurationSeconds,
		ma.AvgHR,
		ma.MaxHR,
		ma.ElevationGainMeters,
		ma.GarminID,
		ma.StravaID,
		ma.Description,
		ma.LocationName,
		ma.Latitude,
		ma.Longitude,
		isRaceVal,
		sourcesStr,
	)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// GetMergedActivitiesForRange retrieves merged activities in range
func (db *DB) GetMergedActivitiesForRange(start, end time.Time) ([]*models.MergedActivity, error) {
	query := `SELECT id, date, start_time, sport, title, distance_meters, duration_seconds,
		avg_hr, max_hr, elevation_gain_meters, garmin_id, strava_id,
		description, location_name, latitude, longitude, is_race, sources
		FROM merged_activities
		WHERE date >= ? AND date <= ?
		ORDER BY date ASC, start_time ASC`

	rows, err := db.conn.Query(query, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.MergedActivity
	for rows.Next() {
		var ma models.MergedActivity
		var dateStr, startTimeStr, sourcesStr string
		var isRaceInt int

		err := rows.Scan(
			&ma.ID, &dateStr, &startTimeStr, &ma.Sport, &ma.Title, &ma.DistanceMeters, &ma.DurationSeconds,
			&ma.AvgHR, &ma.MaxHR, &ma.ElevationGainMeters, &ma.GarminID, &ma.StravaID,
			&ma.Description, &ma.LocationName, &ma.Latitude, &ma.Longitude, &isRaceInt, &sourcesStr,
		)
		if err != nil {
			return nil, err
		}

		ma.Date = parseDate(dateStr)
		ma.StartTime, _ = time.Parse(time.RFC3339, startTimeStr)
		ma.IsRace = isRaceInt == 1
		ma.Sources = strings.Split(sourcesStr, ",")

		list = append(list, &ma)
	}

	return list, nil
}

// SaveTrainingPlan inserts a training plan and returns its ID
func (db *DB) SaveTrainingPlan(plan *models.TrainingPlan) (int64, error) {
	query := `INSERT INTO training_plans (name, description, start_date, end_date) VALUES (?, ?, ?, ?)`
	res, err := db.conn.Exec(query, plan.Name, plan.Description, plan.StartDate.Format("2006-01-02"), plan.EndDate.Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SavePlannedWorkout inserts a planned workout
func (db *DB) SavePlannedWorkout(pw *models.PlannedWorkout) error {
	query := `INSERT INTO planned_workouts (
		plan_id, planned_date, sport, target_distance_miles, target_duration_minutes,
		target_intensity, target_notes, associated_activity_id, status
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.conn.Exec(query,
		pw.PlanID,
		pw.PlannedDate.Format("2006-01-02"),
		pw.Sport,
		pw.TargetDistanceMiles,
		pw.TargetDurationMinutes,
		pw.TargetIntensity,
		pw.TargetNotes,
		pw.AssociatedActivityID,
		pw.Status,
	)
	return err
}

// GetPlannedWorkoutsForPlan gets workouts for a plan
func (db *DB) GetPlannedWorkoutsForPlan(planID int64) ([]*models.PlannedWorkout, error) {
	query := `SELECT id, plan_id, planned_date, sport, target_distance_miles, target_duration_minutes,
		target_intensity, target_notes, associated_activity_id, status
		FROM planned_workouts
		WHERE plan_id = ?
		ORDER BY planned_date ASC`

	rows, err := db.conn.Query(query, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.PlannedWorkout
	for rows.Next() {
		var pw models.PlannedWorkout
		var dateStr string

		err := rows.Scan(
			&pw.ID, &pw.PlanID, &dateStr, &pw.Sport, &pw.TargetDistanceMiles, &pw.TargetDurationMinutes,
			&pw.TargetIntensity, &pw.TargetNotes, &pw.AssociatedActivityID, &pw.Status,
		)
		if err != nil {
			return nil, err
		}

		pw.PlannedDate = parseDate(dateStr)
		list = append(list, &pw)
	}

	return list, nil
}

// GetActivePlans gets all plans
func (db *DB) GetActivePlans() ([]*models.TrainingPlan, error) {
	query := `SELECT id, name, description, start_date, end_date FROM training_plans ORDER BY start_date ASC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.TrainingPlan
	for rows.Next() {
		var p models.TrainingPlan
		var startStr, endStr string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &startStr, &endStr); err != nil {
			return nil, err
		}
		p.StartDate = parseDate(startStr)
		p.EndDate = parseDate(endStr)
		list = append(list, &p)
	}
	return list, nil
}

// AutoLinkWorkouts attempts to link unlinked planned workouts with actual merged activities
func (db *DB) AutoLinkWorkouts() (int, error) {
	// Find all planned workouts without an associated activity
	query := `SELECT id, planned_date, sport FROM planned_workouts WHERE associated_activity_id IS NULL`
	rows, err := db.conn.Query(query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type unlinked struct {
		id    int64
		date  string
		sport string
	}

	var ulList []unlinked
	for rows.Next() {
		var ul unlinked
		if err := rows.Scan(&ul.id, &ul.date, &ul.sport); err != nil {
			return 0, err
		}
		ulList = append(ulList, ul)
	}

	linkedCount := 0
	for _, ul := range ulList {
		// Look for a merged activity on the same date (or ±1 day) with the same sport type
		// If multiple matched, we pick the closest one
		matchQuery := `SELECT id FROM merged_activities
			WHERE sport = ? AND date >= date(?, '-1 day') AND date <= date(?, '+1 day')
			AND id NOT IN (SELECT associated_activity_id FROM planned_workouts WHERE associated_activity_id IS NOT NULL)
			ORDER BY abs(julianday(date) - julianday(?)) ASC LIMIT 1`

		var activityID int64
		err := db.conn.QueryRow(matchQuery, ul.sport, ul.date, ul.date, ul.date).Scan(&activityID)
		if err == sql.ErrNoRows {
			continue
		} else if err != nil {
			return linkedCount, err
		}

		// Link them!
		updateQuery := `UPDATE planned_workouts SET associated_activity_id = ?, status = 'Completed' WHERE id = ?`
		_, err = db.conn.Exec(updateQuery, activityID, ul.id)
		if err != nil {
			return linkedCount, err
		}
		linkedCount++
	}

	return linkedCount, nil
}

// SaveFutureRaces replaces the upcoming races list
func (db *DB) SaveFutureRaces(races []*models.FutureRace) error {
	// Clear existing list
	if _, err := db.conn.Exec("DELETE FROM future_races"); err != nil {
		return err
	}

	query := `INSERT INTO future_races (title, race_date, distance_display, location, website_url, latitude, longitude)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	for _, r := range races {
		_, err := db.conn.Exec(query, r.Title, r.RaceDate.Format("2006-01-02"), r.DistanceDisplay, r.Location, r.WebsiteURL, r.Latitude, r.Longitude)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetFutureRaces gets future races
func (db *DB) GetFutureRaces() ([]*models.FutureRace, error) {
	query := `SELECT id, title, race_date, distance_display, location, website_url, latitude, longitude
		FROM future_races
		ORDER BY race_date ASC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.FutureRace
	for rows.Next() {
		var r models.FutureRace
		var dateStr string
		err := rows.Scan(&r.ID, &r.Title, &dateStr, &r.DistanceDisplay, &r.Location, &r.WebsiteURL, &r.Latitude, &r.Longitude)
		if err != nil {
			return nil, err
		}
		r.RaceDate = parseDate(dateStr)
		list = append(list, &r)
	}
	return list, nil
}

// GetMergedRaces gets all merged activities that are races
func (db *DB) GetMergedRaces(startDate *time.Time) ([]*models.MergedActivity, error) {
	var query string
	var rows *sql.Rows
	var err error

	if startDate != nil {
		query = `SELECT id, date, start_time, sport, title, distance_meters, duration_seconds,
			avg_hr, max_hr, elevation_gain_meters, garmin_id, strava_id,
			description, location_name, latitude, longitude, is_race, sources
			FROM merged_activities
			WHERE is_race = 1 AND date >= ?
			ORDER BY date ASC`
		rows, err = db.conn.Query(query, startDate.Format("2006-01-02"))
	} else {
		query = `SELECT id, date, start_time, sport, title, distance_meters, duration_seconds,
			avg_hr, max_hr, elevation_gain_meters, garmin_id, strava_id,
			description, location_name, latitude, longitude, is_race, sources
			FROM merged_activities
			WHERE is_race = 1
			ORDER BY date ASC`
		rows, err = db.conn.Query(query)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.MergedActivity
	for rows.Next() {
		var ma models.MergedActivity
		var dateStr, startTimeStr, sourcesStr string
		var isRaceInt int

		err := rows.Scan(
			&ma.ID, &dateStr, &startTimeStr, &ma.Sport, &ma.Title, &ma.DistanceMeters, &ma.DurationSeconds,
			&ma.AvgHR, &ma.MaxHR, &ma.ElevationGainMeters, &ma.GarminID, &ma.StravaID,
			&ma.Description, &ma.LocationName, &ma.Latitude, &ma.Longitude, &isRaceInt, &sourcesStr,
		)
		if err != nil {
			return nil, err
		}

		ma.Date = parseDate(dateStr)
		ma.StartTime, _ = time.Parse(time.RFC3339, startTimeStr)
		ma.IsRace = isRaceInt == 1
		ma.Sources = strings.Split(sourcesStr, ",")

		list = append(list, &ma)
	}
	return list, nil
}

// GetMergedTrailRuns gets all trail runs (Sport=Run, Elevation>=609.6 meters, from Strava)
func (db *DB) GetMergedTrailRuns(startDate *time.Time) ([]*models.MergedActivity, error) {
	var query string
	var rows *sql.Rows
	var err error

	if startDate != nil {
		query = `SELECT id, date, start_time, sport, title, distance_meters, duration_seconds,
			avg_hr, max_hr, elevation_gain_meters, garmin_id, strava_id,
			description, location_name, latitude, longitude, is_race, sources
			FROM merged_activities
			WHERE sport = 'Run' AND elevation_gain_meters >= 609.6 AND sources LIKE '%strava%' AND date >= ?
			ORDER BY date ASC`
		rows, err = db.conn.Query(query, startDate.Format("2006-01-02"))
	} else {
		query = `SELECT id, date, start_time, sport, title, distance_meters, duration_seconds,
			avg_hr, max_hr, elevation_gain_meters, garmin_id, strava_id,
			description, location_name, latitude, longitude, is_race, sources
			FROM merged_activities
			WHERE sport = 'Run' AND elevation_gain_meters >= 609.6 AND sources LIKE '%strava%'
			ORDER BY date ASC`
		rows, err = db.conn.Query(query)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.MergedActivity
	for rows.Next() {
		var ma models.MergedActivity
		var dateStr, startTimeStr, sourcesStr string
		var isRaceInt int

		err := rows.Scan(
			&ma.ID, &dateStr, &startTimeStr, &ma.Sport, &ma.Title, &ma.DistanceMeters, &ma.DurationSeconds,
			&ma.AvgHR, &ma.MaxHR, &ma.ElevationGainMeters, &ma.GarminID, &ma.StravaID,
			&ma.Description, &ma.LocationName, &ma.Latitude, &ma.Longitude, &isRaceInt, &sourcesStr,
		)
		if err != nil {
			return nil, err
		}

		ma.Date = parseDate(dateStr)
		ma.StartTime, _ = time.Parse(time.RFC3339, startTimeStr)
		ma.IsRace = isRaceInt == 1
		ma.Sources = strings.Split(sourcesStr, ",")

		list = append(list, &ma)
	}
	return list, nil
}

// GetRawActivitiesForSync gets all raw activities in memory for merging
func (db *DB) GetRawActivitiesForSync(start, end time.Time) ([]*models.Activity, []*models.Activity, error) {
	garmin, err := db.GetActivitiesForRange("garmin", start, end)
	if err != nil {
		return nil, nil, err
	}
	strava, err := db.GetActivitiesForRange("strava", start, end)
	if err != nil {
		return nil, nil, err
	}
	return garmin, strava, nil
}
