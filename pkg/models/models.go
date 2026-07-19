package models

import (
	"time"
)

// Activity represents a normalized activity fetched from a specific provider (Garmin or Strava)
type Activity struct {
	ID                   int64     `json:"id"`
	Provider             string    `json:"provider"` // "garmin" or "strava"
	Sport                string    `json:"sport"`    // "Run", "Ride", "Swim", "Walk", "Hike", "Other"
	Title                string    `json:"title"`
	StartTime            time.Time `json:"start_time"`
	DistanceMeters       float64   `json:"distance_meters"`
	DurationSeconds      float64   `json:"duration_seconds"`
	AvgHR                *float64  `json:"avg_hr,omitempty"`
	MaxHR                *float64  `json:"max_hr,omitempty"`
	ElevationGainMeters  float64   `json:"elevation_gain_meters"`
	Description          string    `json:"description,omitempty"`
	LocationName         string    `json:"location_name,omitempty"`
	Latitude             *float64  `json:"latitude,omitempty"`
	Longitude            *float64  `json:"longitude,omitempty"`
	IsRace               bool      `json:"is_race"`
	RawData              string    `json:"raw_data,omitempty"` // Store raw JSON payload as text
}

// MergedActivity represents a unified activity that can combine Garmin and Strava entries
type MergedActivity struct {
	ID                   int64     `json:"id"` // Generated/incremental id
	Date                 time.Time `json:"date"`
	StartTime            time.Time `json:"start_time"`
	Sport                string    `json:"sport"`
	Title                string    `json:"title"`
	DistanceMeters       float64   `json:"distance_meters"`
	DurationSeconds      float64   `json:"duration_seconds"`
	AvgHR                *float64  `json:"avg_hr,omitempty"`
	MaxHR                *float64  `json:"max_hr,omitempty"`
	ElevationGainMeters  float64   `json:"elevation_gain_meters"`
	GarminID             *int64    `json:"garmin_id,omitempty"`
	StravaID             *int64    `json:"strava_id,omitempty"`
	Description          string    `json:"description,omitempty"`
	LocationName         string    `json:"location_name,omitempty"`
	Latitude             *float64  `json:"latitude,omitempty"`
	Longitude            *float64  `json:"longitude,omitempty"`
	IsRace               bool      `json:"is_race"`
	Sources              []string  `json:"sources"` // "garmin", "strava"
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
	ID                    int64      `json:"id"`
	PlanID                int64      `json:"plan_id"`
	PlannedDate           time.Time  `json:"planned_date"`
	Sport                 string     `json:"sport"`
	TargetDistanceMiles   *float64   `json:"target_distance_miles,omitempty"`
	TargetDurationMinutes *float64   `json:"target_duration_minutes,omitempty"`
	TargetIntensity       string     `json:"target_intensity,omitempty"` // e.g. "Tempo", "Easy", "Interval", "Long"
	TargetNotes           string     `json:"target_notes,omitempty"`
	AssociatedActivityID  *int64     `json:"associated_activity_id,omitempty"` // nullable FK to MergedActivity ID
	Status                string     `json:"status"`                           // "Scheduled", "Completed", "Missed"
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
}
