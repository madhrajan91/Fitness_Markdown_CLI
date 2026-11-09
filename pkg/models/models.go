package models

import (
	"fmt"
	"time"
)

// Activity represents a normalized activity fetched from a specific provider (Garmin or Strava)
type Activity struct {
	ID                  int64     `json:"id"`
	Provider            string    `json:"provider"` // "garmin" or "strava"
	Sport               string    `json:"sport"`    // "Run", "Ride", "Swim", "Walk", "Hike", "Other"
	Title               string    `json:"title"`
	StartTime           time.Time `json:"start_time"`
	DistanceMeters      float64   `json:"distance_meters"`
	DurationSeconds     float64   `json:"duration_seconds"`
	AvgHR               *float64  `json:"avg_hr,omitempty"`
	MaxHR               *float64  `json:"max_hr,omitempty"`
	ElevationGainMeters float64   `json:"elevation_gain_meters"`
	Description         string    `json:"description,omitempty"`
	LocationName        string    `json:"location_name,omitempty"`
	Latitude            *float64  `json:"latitude,omitempty"`
	Longitude           *float64  `json:"longitude,omitempty"`
	IsRace              bool      `json:"is_race"`
	Links               string    `json:"links,omitempty"`
	RawData             string    `json:"raw_data,omitempty"` // Store raw JSON payload as text
}

// MergedActivity represents a unified activity that can combine Garmin and Strava entries
type MergedActivity struct {
	ID                  int64     `json:"id"` // Generated/incremental id
	Date                time.Time `json:"date"`
	StartTime           time.Time `json:"start_time"`
	Sport               string    `json:"sport"`
	Title               string    `json:"title"`
	DistanceMeters      float64   `json:"distance_meters"`
	DurationSeconds     float64   `json:"duration_seconds"`
	AvgHR               *float64  `json:"avg_hr,omitempty"`
	MaxHR               *float64  `json:"max_hr,omitempty"`
	ElevationGainMeters float64   `json:"elevation_gain_meters"`
	GarminID            *int64    `json:"garmin_id,omitempty"`
	StravaID            *int64    `json:"strava_id,omitempty"`
	Description         string    `json:"description,omitempty"`
	LocationName        string    `json:"location_name,omitempty"`
	Latitude            *float64  `json:"latitude,omitempty"`
	Longitude           *float64  `json:"longitude,omitempty"`
	IsRace              bool      `json:"is_race"`
	Links               string    `json:"links,omitempty"`
	Sources             []string  `json:"sources"` // "garmin", "strava"

	// Weather at the activity's start, backfilled from Open-Meteo's archive
	// API. Nil when the activity has no coordinates or hasn't been backfilled.
	TempC       *float64 `json:"temp_c,omitempty"`
	HumidityPct *float64 `json:"humidity_pct,omitempty"`
	WindKph     *float64 `json:"wind_kph,omitempty"`
	WeatherCode *int     `json:"weather_code,omitempty"`
}

// HasWeather reports whether weather has been resolved for this activity.
func (a *MergedActivity) HasWeather() bool {
	return a.TempC != nil
}

// SourceLink is a link to an activity's page on the provider it came from.
type SourceLink struct {
	Source string `json:"source"` // "garmin" or "strava"
	URL    string `json:"url"`
}

// SourceURL returns the public web page for this activity on the given
// provider, or "" if the activity has no ID for it.
func (a *MergedActivity) SourceURL(source string) string {
	switch source {
	case "garmin":
		if a.GarminID != nil {
			return fmt.Sprintf("https://connect.garmin.com/modern/activity/%d", *a.GarminID)
		}
	case "strava":
		if a.StravaID != nil {
			return fmt.Sprintf("https://www.strava.com/activities/%d", *a.StravaID)
		}
	}
	return ""
}

// SourceLinks returns one link per source that has a usable provider ID.
func (a *MergedActivity) SourceLinks() []SourceLink {
	links := make([]SourceLink, 0, len(a.Sources))
	for _, s := range a.Sources {
		if url := a.SourceURL(s); url != "" {
			links = append(links, SourceLink{Source: s, URL: url})
		}
	}
	return links
}

// TrainingPlan represents a scheduled plan (e.g. 5K, Marathon Plan)
type TrainingPlan struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
}

// PlannedWorkout represents a workout scheduled inside a training plan
type PlannedWorkout struct {
	ID                    int64     `json:"id"`
	PlanID                int64     `json:"plan_id"`
	PlannedDate           time.Time `json:"planned_date"`
	Sport                 string    `json:"sport"`
	TargetDistanceMiles   *float64  `json:"target_distance_miles,omitempty"`
	TargetDurationMinutes *float64  `json:"target_duration_minutes,omitempty"`
	TargetIntensity       string    `json:"target_intensity,omitempty"` // e.g. "Tempo", "Easy", "Interval", "Long"
	TargetNotes           string    `json:"target_notes,omitempty"`
	AssociatedActivityID  *int64    `json:"associated_activity_id,omitempty"` // nullable FK to MergedActivity ID
	Status                string    `json:"status"`                           // "Scheduled", "Completed", "Missed"
}

// FutureRace represents an upcoming race found via RunSignup
type FutureRace struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	RaceDate        time.Time `json:"race_date"`
	DistanceDisplay string    `json:"distance_display"`
	Location        string    `json:"location"`
	WebsiteURL      string    `json:"website_url"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	Price           string    `json:"price,omitempty"`
	Status          string    `json:"status,omitempty"`
	ProximityMiles  float64   `json:"proximity_miles,omitempty"`
}
