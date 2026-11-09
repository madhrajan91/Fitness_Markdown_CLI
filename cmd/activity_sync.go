package cmd

import (
	"fmt"
	"strings"
	"time"

	"running_cli/pkg/client"
	"running_cli/pkg/config"
	"running_cli/pkg/db"
	"running_cli/pkg/merger"
	"running_cli/pkg/models"
)

// Fetchers are variables so tests can replace the network calls.
var (
	fetchGarminActivities = client.FetchGarminActivities
	fetchStravaActivities = client.FetchStravaActivities
)

// activitySyncResult summarizes one run of syncActivities.
type activitySyncResult struct {
	Garmin   int      `json:"garmin"`
	Strava   int      `json:"strava"`
	Merged   int      `json:"merged"`
	Linked   int      `json:"linked"`
	Warnings []string `json:"warnings"`
	// NeedsAuth lists providers whose login must be redone with `running-cli setup`.
	NeedsAuth []string `json:"needs_auth"`

	merged []*models.MergedActivity
}

// isAuthError reports whether a provider error means the stored login no
// longer works (expired or revoked token, bad credentials).
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"token refresh failed", "(401)", "(403)", "unauthorized",
		"authentication", "invalid_grant", "login", "credentials",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// resolveSyncRange returns the range to sync: startDate/endDate (YYYY-MM-DD)
// when given, otherwise the last `days` days up to the end of today.
func resolveSyncRange(days int, startDate, endDate string, now time.Time) (time.Time, time.Time, error) {
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)
	if endDate != "" {
		d, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end date %q: use YYYY-MM-DD", endDate)
		}
		end = time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, time.Local)
	}

	var start time.Time
	if startDate != "" {
		d, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start date %q: use YYYY-MM-DD", startDate)
		}
		start = d
	} else {
		if days < 1 || days > 3650 {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid days %d: use 1-3650", days)
		}
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -days)
	}

	if start.After(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("start date %s is after end date %s",
			start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	return start, end, nil
}

// syncActivities fetches Garmin and Strava activities in [start, end], stores
// them, re-merges the range and links planned workouts. Progress goes to logf.
// Provider failures are reported in the result rather than as an error, so one
// provider failing doesn't stop the other.
func syncActivities(cfg *config.Config, database *db.DB, start, end time.Time, logf func(string, ...interface{})) (*activitySyncResult, error) {
	res := &activitySyncResult{Warnings: []string{}, NeedsAuth: []string{}}
	providerFailed := func(name string, err error) {
		msg := fmt.Sprintf("%s fetch failed: %v", name, err)
		logf("  %s\n", msg)
		res.Warnings = append(res.Warnings, msg)
		if isAuthError(err) {
			res.NeedsAuth = append(res.NeedsAuth, strings.ToLower(name))
		}
	}

	logf("Syncing activities from %s to %s...\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

	if cfg.IsGarminConfigured() {
		logf("Fetching Garmin activities...\n")
		garminActs, err := fetchGarminActivities(cfg.Garmin.Email, "", start, end)
		if err != nil {
			providerFailed("Garmin", err)
		} else {
			res.Garmin = len(garminActs)
			logf("  Fetched %d Garmin activities.\n", len(garminActs))
			for _, a := range garminActs {
				// Skip if a Strava activity exists on this date
				hasStrava, err := database.HasStravaActivityOnDate(a.StartTime, a.Sport)
				if err == nil && hasStrava {
					continue
				}
				if err := database.InsertOrUpdateActivity(a); err != nil {
					logf("  Error saving Garmin activity %d: %v\n", a.ID, err)
				}
			}
		}
	}

	if cfg.IsStravaConfigured() {
		logf("Fetching Strava activities...\n")
		stravaActs, err := fetchStravaActivities(cfg.Strava.ClientID, cfg.Strava.ClientSecret, cfg.Strava.RefreshToken, start, end)
		if err != nil {
			providerFailed("Strava", err)
		} else {
			res.Strava = len(stravaActs)
			logf("  Fetched %d Strava activities.\n", len(stravaActs))
			for _, a := range stravaActs {
				if err := database.InsertOrUpdateActivity(a); err != nil {
					logf("  Error saving Strava activity %d: %v\n", a.ID, err)
				} else {
					// Remove matching Garmin activities to avoid duplicates
					database.DeleteMatchingGarminActivity(a.StartTime, a.Sport)
				}
			}
		}
	}

	gRaw, sRaw, err := database.GetRawActivitiesForSync(start, end)
	if err != nil {
		return res, fmt.Errorf("reading raw activities: %w", err)
	}

	mergedList := merger.MergeActivities(gRaw, sRaw)
	res.merged = mergedList
	res.Merged = len(mergedList)
	logf("Merged into %d unique activities.\n", len(mergedList))

	if err := database.ClearMergedActivitiesRange(start, end); err != nil {
		return res, fmt.Errorf("clearing merged activities: %w", err)
	}
	for _, ma := range mergedList {
		id, err := database.SaveMergedActivity(ma)
		if err != nil {
			logf("Error saving merged activity: %v\n", err)
		} else {
			ma.ID = id
		}
	}

	linked, err := database.AutoLinkWorkouts()
	if err != nil {
		logf("Warning: Failed to auto-link workouts: %v\n", err)
		res.Warnings = append(res.Warnings, fmt.Sprintf("auto-linking failed: %v", err))
	} else {
		res.Linked = linked
		if linked > 0 {
			logf("Auto-linked %d synced activities to scheduled training plans.\n", linked)
		}
	}

	return res, nil
}
