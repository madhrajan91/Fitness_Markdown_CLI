package merger

import (
	"math"
	"sort"
	"time"

	"running_cli/pkg/models"
)

// MergeActivities merges Garmin and Strava activities, deduplicating overlaps
func MergeActivities(garmin []*models.Activity, strava []*models.Activity) []*models.MergedActivity {
	var merged []*models.MergedActivity
	matchedStravaIDs := make(map[int64]bool)

	for _, g := range garmin {
		var matchedStrava *models.Activity

		for _, s := range strava {
			if matchedStravaIDs[s.ID] {
				continue
			}

			// 1. Sport must match (allow "Other" fallback)
			if g.Sport != s.Sport {
				if g.Sport != "Other" && s.Sport != "Other" {
					continue
				}
			}

			// 2. Start time difference must be within 10 minutes (600s)
			// Compare start times using local wall-clock times to avoid timezone offset mismatches (e.g. UTC vs local timezone)
			gLocal := time.Date(g.StartTime.Year(), g.StartTime.Month(), g.StartTime.Day(), g.StartTime.Hour(), g.StartTime.Minute(), g.StartTime.Second(), 0, time.UTC)
			sLocal := time.Date(s.StartTime.Year(), s.StartTime.Month(), s.StartTime.Day(), s.StartTime.Hour(), s.StartTime.Minute(), s.StartTime.Second(), 0, time.UTC)
			timeDiff := math.Abs(gLocal.Sub(sLocal).Seconds())
			if timeDiff > 600 {
				continue
			}

			// 3. Distance difference must be within 10% or within 500 meters
			distDiff := math.Abs(g.DistanceMeters - s.DistanceMeters)
			avgDist := (g.DistanceMeters + s.DistanceMeters) / 2.0
			isDistMatch := false

			if avgDist > 0 {
				isDistMatch = (distDiff / avgDist) <= 0.10
			} else {
				isDistMatch = distDiff <= 500
			}

			if isDistMatch || distDiff <= 500 {
				matchedStrava = s
				break
			}
		}

		if matchedStrava != nil {
			matchedStravaIDs[matchedStrava.ID] = true

			// Choose Strava title/description if present
			title := g.Title
			if matchedStrava.Title != "" {
				title = matchedStrava.Title
			}

			description := g.Description
			if matchedStrava.Description != "" {
				description = matchedStrava.Description
			}

			locationName := g.LocationName
			if locationName == "" {
				locationName = matchedStrava.LocationName
			}

			var latitude, longitude *float64
			if g.Latitude != nil {
				latitude = g.Latitude
			} else {
				latitude = matchedStrava.Latitude
			}

			if g.Longitude != nil {
				longitude = g.Longitude
			} else {
				longitude = matchedStrava.Longitude
			}

			var avgHR, maxHR *float64
			if g.AvgHR != nil {
				avgHR = g.AvgHR
			} else {
				avgHR = matchedStrava.AvgHR
			}

			if g.MaxHR != nil {
				maxHR = g.MaxHR
			} else {
				maxHR = matchedStrava.MaxHR
			}

			links := g.Links
			if links == "" {
				links = matchedStrava.Links
			}

			merged = append(merged, &models.MergedActivity{
				Date:                time.Date(g.StartTime.Year(), g.StartTime.Month(), g.StartTime.Day(), 0, 0, 0, 0, time.Local),
				StartTime:           g.StartTime,
				Sport:               g.Sport,
				Title:               title,
				DistanceMeters:      g.DistanceMeters, // Garmin GPS is preferred
				DurationSeconds:     g.DurationSeconds,
				AvgHR:               avgHR,
				MaxHR:               maxHR,
				ElevationGainMeters: g.ElevationGainMeters,
				GarminID:            &g.ID,
				StravaID:            &matchedStrava.ID,
				Description:         description,
				LocationName:        locationName,
				Latitude:            latitude,
				Longitude:           longitude,
				IsRace:              g.IsRace || matchedStrava.IsRace,
				Links:               links,
				Sources:             []string{"garmin", "strava"},
			})
		} else {
			// Garmin only
			merged = append(merged, &models.MergedActivity{
				Date:                time.Date(g.StartTime.Year(), g.StartTime.Month(), g.StartTime.Day(), 0, 0, 0, 0, time.Local),
				StartTime:           g.StartTime,
				Sport:               g.Sport,
				Title:               g.Title,
				DistanceMeters:      g.DistanceMeters,
				DurationSeconds:     g.DurationSeconds,
				AvgHR:               g.AvgHR,
				MaxHR:               g.MaxHR,
				ElevationGainMeters: g.ElevationGainMeters,
				GarminID:            &g.ID,
				Description:         g.Description,
				LocationName:        g.LocationName,
				Latitude:            g.Latitude,
				Longitude:           g.Longitude,
				IsRace:              g.IsRace,
				Links:               g.Links,
				Sources:             []string{"garmin"},
			})
		}
	}

	// Add remaining Strava-only activities (including local manual activities)
	for _, s := range strava {
		if matchedStravaIDs[s.ID] {
			continue
		}

		merged = append(merged, &models.MergedActivity{
			Date:                time.Date(s.StartTime.Year(), s.StartTime.Month(), s.StartTime.Day(), 0, 0, 0, 0, time.Local),
			StartTime:           s.StartTime,
			Sport:               s.Sport,
			Title:               s.Title,
			DistanceMeters:      s.DistanceMeters,
			DurationSeconds:     s.DurationSeconds,
			AvgHR:               s.AvgHR,
			MaxHR:               s.MaxHR,
			ElevationGainMeters: s.ElevationGainMeters,
			StravaID:            &s.ID,
			Description:         s.Description,
			LocationName:        s.LocationName,
			Latitude:            s.Latitude,
			Longitude:           s.Longitude,
			IsRace:              s.IsRace,
			Links:               s.Links,
			Sources:             []string{s.Provider},
		})
	}

	// Sort chronologically
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].StartTime.Before(merged[j].StartTime)
	})

	return merged
}
