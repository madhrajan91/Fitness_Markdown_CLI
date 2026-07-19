package merger

import (
	"testing"
	"time"

	"running_cli/pkg/models"
)

func TestMergeActivities(t *testing.T) {
	// Setup test data
	now := time.Now()

	// 1. Garmin run
	gRun := &models.Activity{
		ID:                  101,
		Provider:            "garmin",
		Sport:               "Run",
		Title:               "Garmin morning run",
		StartTime:           now,
		DistanceMeters:      8000, // 8 km
		DurationSeconds:     2400, // 40 mins
		AvgHR:               floatPointer(150),
		MaxHR:               floatPointer(170),
		ElevationGainMeters: 50,
	}

	// 2. Strava run (matches gRun)
	sRun := &models.Activity{
		ID:                  201,
		Provider:            "strava",
		Sport:               "Run",
		Title:               "Strava morning run", // Preferred title
		StartTime:           now.Add(2 * time.Minute), // ±10m
		DistanceMeters:      8100, // within 10%
		DurationSeconds:     2390,
		AvgHR:               floatPointer(148),
		MaxHR:               floatPointer(168),
		ElevationGainMeters: 45,
	}

	// 3. Garmin ride (no matching Strava)
	gRide := &models.Activity{
		ID:                  102,
		Provider:            "garmin",
		Sport:               "Ride",
		Title:               "Garmin ride",
		StartTime:           now.Add(2 * time.Hour),
		DistanceMeters:      20000,
		DurationSeconds:     3600,
	}

	// 4. Strava swim (no matching Garmin)
	sSwim := &models.Activity{
		ID:                  202,
		Provider:            "strava",
		Sport:               "Swim",
		Title:               "Strava swim",
		StartTime:           now.Add(4 * time.Hour),
		DistanceMeters:      1500,
		DurationSeconds:     1800,
	}

	garminList := []*models.Activity{gRun, gRide}
	stravaList := []*models.Activity{sRun, sSwim}

	merged := MergeActivities(garminList, stravaList)

	if len(merged) != 3 {
		t.Fatalf("Expected 3 merged activities, got %d", len(merged))
	}

	// First activity should be the merged run (chronological sorting)
	runAct := merged[0]
	if runAct.Sport != "Run" {
		t.Errorf("Expected first activity to be Run, got %s", runAct.Sport)
	}
	if runAct.Title != "Strava morning run" {
		t.Errorf("Expected title to be preferred from Strava ('Strava morning run'), got '%s'", runAct.Title)
	}
	if runAct.GarminID == nil || *runAct.GarminID != 101 {
		t.Errorf("Expected GarminID to be 101, got %v", runAct.GarminID)
	}
	if runAct.StravaID == nil || *runAct.StravaID != 201 {
		t.Errorf("Expected StravaID to be 201, got %v", runAct.StravaID)
	}
	if runAct.DistanceMeters != 8000 {
		t.Errorf("Expected distance preferred from Garmin (8000), got %.1f", runAct.DistanceMeters)
	}
	if len(runAct.Sources) != 2 || runAct.Sources[0] != "garmin" || runAct.Sources[1] != "strava" {
		t.Errorf("Expected sources to be ['garmin', 'strava'], got %v", runAct.Sources)
	}

	// Second activity should be the ride
	rideAct := merged[1]
	if rideAct.Sport != "Ride" {
		t.Errorf("Expected second activity to be Ride, got %s", rideAct.Sport)
	}
	if rideAct.GarminID == nil || *rideAct.GarminID != 102 {
		t.Errorf("Expected GarminID 102, got %v", rideAct.GarminID)
	}
	if rideAct.StravaID != nil {
		t.Errorf("Expected StravaID to be nil, got %v", rideAct.StravaID)
	}

	// Third activity should be the swim
	swimAct := merged[2]
	if swimAct.Sport != "Swim" {
		t.Errorf("Expected third activity to be Swim, got %s", swimAct.Sport)
	}
	if swimAct.StravaID == nil || *swimAct.StravaID != 202 {
		t.Errorf("Expected StravaID 202, got %v", swimAct.StravaID)
	}
	if swimAct.GarminID != nil {
		t.Errorf("Expected GarminID to be nil, got %v", swimAct.GarminID)
	}
}

func floatPointer(f float64) *float64 {
	return &f
}
