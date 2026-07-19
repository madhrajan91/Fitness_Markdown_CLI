package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"running_cli/pkg/models"
)

func TestDBAutoLinkWorkouts(t *testing.T) {
	// Override DB path for testing
	tmpDir, err := os.MkdirTemp("", "running-cli-test")
	if err != nil {
		t.Fatalf("Failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testDBPath := filepath.Join(tmpDir, "test.db")
	conn, err := sql.Open("sqlite", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer conn.Close()

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// 1. Create a training plan
	plan := &models.TrainingPlan{
		Name:        "Test Plan",
		Description: "A test plan",
		StartDate:   time.Now().AddDate(0, 0, -2),
		EndDate:     time.Now().AddDate(0, 0, 2),
	}
	planID, err := db.SaveTrainingPlan(plan)
	if err != nil {
		t.Fatalf("SaveTrainingPlan failed: %v", err)
	}

	// 2. Create planned workouts
	workout1 := &models.PlannedWorkout{
		PlanID:      planID,
		PlannedDate: time.Now().AddDate(0, 0, -1), // Yesterday
		Sport:       "Run",
		Status:      "Scheduled",
	}
	workout2 := &models.PlannedWorkout{
		PlanID:      planID,
		PlannedDate: time.Now(), // Today
		Sport:       "Run",
		Status:      "Scheduled",
	}
	if err := db.SavePlannedWorkout(workout1); err != nil {
		t.Fatalf("SavePlannedWorkout failed: %v", err)
	}
	if err := db.SavePlannedWorkout(workout2); err != nil {
		t.Fatalf("SavePlannedWorkout failed: %v", err)
	}

	// 3. Create a synced merged activity matching workout1 (Yesterday's run)
	mergedAct := &models.MergedActivity{
		Date:                time.Now().AddDate(0, 0, -1),
		StartTime:           time.Now().AddDate(0, 0, -1),
		Sport:               "Run",
		Title:               "Yesterday's Run",
		DistanceMeters:      5000,
		DurationSeconds:     1500,
		Sources:             []string{"strava"},
		IsRace:              false,
		ElevationGainMeters: 20,
	}
	activityID, err := db.SaveMergedActivity(mergedAct)
	if err != nil {
		t.Fatalf("SaveMergedActivity failed: %v", err)
	}

	// 4. Trigger auto-linking
	linkedCount, err := db.AutoLinkWorkouts()
	if err != nil {
		t.Fatalf("AutoLinkWorkouts failed: %v", err)
	}

	if linkedCount != 1 {
		t.Errorf("Expected 1 workout to be linked, got %d", linkedCount)
	}

	// 5. Query workouts and verify links
	workouts, err := db.GetPlannedWorkoutsForPlan(planID)
	if err != nil {
		t.Fatalf("GetPlannedWorkoutsForPlan failed: %v", err)
	}

	if len(workouts) != 2 {
		t.Fatalf("Expected 2 workouts, got %d", len(workouts))
	}

	// First workout (yesterday) should be completed and linked
	wYesterday := workouts[0]
	if wYesterday.Status != "Completed" {
		t.Errorf("Expected yesterday's workout to be completed, got %s", wYesterday.Status)
	}
	if wYesterday.AssociatedActivityID == nil || *wYesterday.AssociatedActivityID != activityID {
		t.Errorf("Expected yesterday's workout linked to activity %d, got %v", activityID, wYesterday.AssociatedActivityID)
	}

	// Second workout (today) should still be Scheduled and unlinked
	wToday := workouts[1]
	if wToday.Status != "Scheduled" {
		t.Errorf("Expected today's workout to be Scheduled, got %s", wToday.Status)
	}
	if wToday.AssociatedActivityID != nil {
		t.Errorf("Expected today's workout unlinked, got %v", wToday.AssociatedActivityID)
	}
}
