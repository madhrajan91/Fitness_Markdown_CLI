package storage

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"running_cli/pkg/models"
)

// ParseTrainingPlanCSV parses a training plan CSV file and returns the metadata and workouts list
func ParseTrainingPlanCSV(filePath string, planName, planDesc string) (*models.TrainingPlan, []*models.PlannedWorkout, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	
	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Build header index map
	headerMap := make(map[string]int)
	for idx, h := range headers {
		clean := strings.ToLower(strings.TrimSpace(h))
		headerMap[clean] = idx
	}

	// Validate required fields
	reqCols := []string{"date", "sport"}
	for _, col := range reqCols {
		if _, ok := headerMap[col]; !ok {
			return nil, nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	var workouts []*models.PlannedWorkout
	var minDate, maxDate time.Time

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("error reading CSV record: %w", err)
		}

		// Skip empty records
		if len(record) == 0 || record[headerMap["date"]] == "" {
			continue
		}

		dateStr := record[headerMap["date"]]
		sport := record[headerMap["sport"]]

		var dateVal time.Time
		fmts := []string{"2006-01-02", "01/02/2006", "01-02-2006"}
		for _, f := range fmts {
			if t, err := time.Parse(f, dateStr); err == nil {
				dateVal = t
				break
			}
		}
		if dateVal.IsZero() {
			return nil, nil, fmt.Errorf("failed to parse date: %s", dateStr)
		}

		// Track date bounds for plan
		if minDate.IsZero() || dateVal.Before(minDate) {
			minDate = dateVal
		}
		if maxDate.IsZero() || dateVal.After(maxDate) {
			maxDate = dateVal
		}

		// Optional fields parsing
		var targetDistanceMiles *float64
		if idx, ok := headerMap["distance_miles"]; ok && idx < len(record) && record[idx] != "" {
			if dist, err := strconv.ParseFloat(record[idx], 64); err == nil {
				targetDistanceMiles = &dist
			}
		} else if idx, ok := headerMap["distance"]; ok && idx < len(record) && record[idx] != "" {
			if dist, err := strconv.ParseFloat(record[idx], 64); err == nil {
				targetDistanceMiles = &dist
			}
		}

		var targetDurationMin *float64
		if idx, ok := headerMap["duration_minutes"]; ok && idx < len(record) && record[idx] != "" {
			if dur, err := strconv.ParseFloat(record[idx], 64); err == nil {
				targetDurationMin = &dur
			}
		} else if idx, ok := headerMap["duration"]; ok && idx < len(record) && record[idx] != "" {
			if dur, err := strconv.ParseFloat(record[idx], 64); err == nil {
				targetDurationMin = &dur
			}
		}

		intensity := ""
		if idx, ok := headerMap["intensity"]; ok && idx < len(record) {
			intensity = strings.Title(strings.ToLower(strings.TrimSpace(record[idx])))
		}

		notes := ""
		if idx, ok := headerMap["notes"]; ok && idx < len(record) {
			notes = strings.TrimSpace(record[idx])
		}

		// Normalize sport type casing
		sportClean := strings.Title(strings.ToLower(strings.TrimSpace(sport)))

		workouts = append(workouts, &models.PlannedWorkout{
			PlannedDate:           dateVal,
			Sport:                 sportClean,
			TargetDistanceMiles:   targetDistanceMiles,
			TargetDurationMinutes: targetDurationMin,
			TargetIntensity:       intensity,
			TargetNotes:           notes,
			Status:                "Scheduled",
		})
	}

	plan := &models.TrainingPlan{
		Name:        planName,
		Description: planDesc,
		StartDate:   minDate,
		EndDate:     maxDate,
	}

	return plan, workouts, nil
}
