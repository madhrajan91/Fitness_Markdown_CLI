package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"running_cli/pkg/config"
	"running_cli/pkg/db"
	"running_cli/pkg/models"
)

func TestResolveSyncRange(t *testing.T) {
	now := time.Date(2026, 10, 4, 15, 0, 0, 0, time.Local)

	start, end, err := resolveSyncRange(7, "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if got := start.Format("2006-01-02 15:04"); got != "2026-09-27 00:00" {
		t.Errorf("days=7 start = %s", got)
	}
	if got := end.Format("2006-01-02 15:04:05"); got != "2026-10-04 23:59:59" {
		t.Errorf("days=7 end = %s", got)
	}

	start, end, err = resolveSyncRange(7, "2026-09-01", "2026-09-07", now)
	if err != nil {
		t.Fatal(err)
	}
	if start.Format("2006-01-02") != "2026-09-01" || end.Format("2006-01-02 15:04:05") != "2026-09-07 23:59:59" {
		t.Errorf("explicit range = %v .. %v", start, end)
	}

	for _, c := range []struct {
		days       int
		start, end string
	}{
		{0, "", ""},
		{3651, "", ""},
		{7, "2026-9-1", ""},
		{7, "", "tomorrow"},
		{7, "2026-10-05", "2026-10-01"},
	} {
		if _, _, err := resolveSyncRange(c.days, c.start, c.end, now); err == nil {
			t.Errorf("resolveSyncRange(%d, %q, %q) succeeded, want error", c.days, c.start, c.end)
		}
	}
}

func TestIsAuthError(t *testing.T) {
	auth := []string{
		`failed to refresh Strava token: Strava token refresh failed (400): {"message":"Bad Request","errors":[{"field":"refresh_token","code":"invalid"}]}`,
		`Strava activities call failed (401): {"message":"Authorization Error"}`,
		`Garmin Connect error: GarminConnectAuthenticationError: login failed`,
	}
	for _, msg := range auth {
		if !isAuthError(errors.New(msg)) {
			t.Errorf("isAuthError(%q) = false", msg)
		}
	}
	other := []string{
		"failed to fetch Strava page 2: dial tcp: i/o timeout",
		"Strava activities call failed (429): rate limit exceeded",
		"Garmin Connect python dependency missing. Please run: pip install garminconnect",
	}
	for _, msg := range other {
		if isAuthError(errors.New(msg)) {
			t.Errorf("isAuthError(%q) = true", msg)
		}
	}
	if isAuthError(nil) {
		t.Error("isAuthError(nil) = true")
	}
}

func TestSyncJob(t *testing.T) {
	j := newSyncJob()
	if s := j.snapshot(); s.State != "idle" || s.Log == nil {
		t.Fatalf("initial status = %+v", s)
	}

	start := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 10, 4, 23, 59, 59, 0, time.Local)
	release := make(chan struct{})
	status, err := j.start(start, end, func(logf func(string, ...interface{})) (*activitySyncResult, error) {
		logf("Fetching Strava activities...\n")
		<-release
		logf("  Strava fetch failed: token refresh failed\nMerged into %d unique activities.\n", 3)
		return &activitySyncResult{Merged: 3, NeedsAuth: []string{"strava"}}, nil
	})
	if err != nil || status.State != "running" || status.StartDate != "2026-09-28" || status.EndDate != "2026-10-04" {
		t.Fatalf("start = %+v, %v", status, err)
	}

	// A second sync is refused while the first runs.
	if s, err := j.start(start, end, nil); !errors.Is(err, errSyncRunning) || s.State != "running" {
		t.Errorf("second start = %+v, %v", s, err)
	}

	close(release)
	j.wait()
	s := j.snapshot()
	if s.State != "done" || !s.NeedsAuth || s.Result == nil || s.Result.Merged != 3 || s.FinishedAt == nil {
		t.Errorf("finished status = %+v", s)
	}
	want := []string{"Fetching Strava activities...", "  Strava fetch failed: token refresh failed", "Merged into 3 unique activities."}
	if strings.Join(s.Log, "|") != strings.Join(want, "|") {
		t.Errorf("log = %q", s.Log)
	}

	// The snapshot is a copy.
	s.Log[0] = "changed"
	if j.snapshot().Log[0] == "changed" {
		t.Error("snapshot shares the log slice")
	}

	// A failed run records the error, and a new run can start afterwards.
	if _, err := j.start(start, end, func(func(string, ...interface{})) (*activitySyncResult, error) {
		return nil, errors.New("database locked")
	}); err != nil {
		t.Fatal(err)
	}
	j.wait()
	if s := j.snapshot(); s.State != "failed" || s.Error != "database locked" || s.NeedsAuth || len(s.Log) != 0 {
		t.Errorf("failed status = %+v", s)
	}
}

func TestSyncJobLogCap(t *testing.T) {
	j := newSyncJob()
	j.start(time.Now(), time.Now(), func(logf func(string, ...interface{})) (*activitySyncResult, error) {
		for i := 0; i < maxSyncLogLines+20; i++ {
			logf("line %d\n", i)
		}
		return &activitySyncResult{}, nil
	})
	j.wait()
	s := j.snapshot()
	if len(s.Log) != maxSyncLogLines || s.Log[0] != "line 20" {
		t.Errorf("log has %d lines starting %q", len(s.Log), s.Log[0])
	}
}

func TestSyncActivities(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), config.ConfigDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	database, err := db.OpenDB()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	run := time.Date(2026, 10, 1, 7, 30, 0, 0, time.Local)
	defer func(g, s interface{}) {
		fetchGarminActivities = g.(func(string, string, time.Time, time.Time) ([]*models.Activity, error))
		fetchStravaActivities = s.(func(string, string, string, time.Time, time.Time) ([]*models.Activity, error))
	}(fetchGarminActivities, fetchStravaActivities)

	fetchGarminActivities = func(string, string, time.Time, time.Time) ([]*models.Activity, error) {
		return nil, errors.New("Garmin Connect error: GarminConnectAuthenticationError: login failed")
	}
	fetchStravaActivities = func(_, _, _ string, start, end time.Time) ([]*models.Activity, error) {
		return []*models.Activity{{
			ID: 20062347587, Provider: "strava", Sport: "Run", Title: "Morning Run",
			StartTime: run, DistanceMeters: 10000, DurationSeconds: 2700, RawData: "{}",
		}}, nil
	}

	cfg := &config.Config{
		Garmin: config.GarminConfig{Email: "me@example.com"},
		Strava: config.StravaConfig{ClientID: "1", ClientSecret: "s", RefreshToken: "r"},
	}
	start, end, _ := resolveSyncRange(0, "2026-09-28", "2026-10-04", run)
	var log []string
	res, err := syncActivities(cfg, database, start, end, func(f string, a ...interface{}) {
		log = append(log, strings.TrimSpace(f))
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Strava != 1 || res.Garmin != 0 || res.Merged != 1 {
		t.Errorf("result = %+v", res)
	}
	if len(res.NeedsAuth) != 1 || res.NeedsAuth[0] != "garmin" || len(res.Warnings) != 1 {
		t.Errorf("auth/warnings = %v / %v", res.NeedsAuth, res.Warnings)
	}

	// The merged activity is findable by its Strava ID.
	got, err := database.GetMergedActivitiesBySourceID("strava", 20062347587)
	if err != nil || len(got) != 1 || got[0].Title != "Morning Run" {
		t.Errorf("lookup after sync = %v, %v", got, err)
	}

	// Syncing again doesn't duplicate.
	if res, err := syncActivities(cfg, database, start, end, func(string, ...interface{}) {}); err != nil || res.Merged != 1 {
		t.Errorf("second sync = %+v, %v", res, err)
	}
	week, _ := database.GetMergedActivitiesForRange(start, end)
	if len(week) != 1 {
		t.Errorf("week has %d activities after two syncs, want 1", len(week))
	}
}
