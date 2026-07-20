package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"running_cli/pkg/config"
	"running_cli/pkg/models"
)

const GarminHelperScript = `import sys
import json
import os
from datetime import date
from garminconnect import Garmin

def main():
    if len(sys.argv) < 5:
        print(json.dumps({"error": "Invalid arguments. Expected: email, password, start_date, end_date, tokenstore"}))
        sys.exit(1)

    email = sys.argv[1]
    password = sys.argv[2]
    start_date_str = sys.argv[3]
    end_date_str = sys.argv[4]
    tokenstore = sys.argv[5]

    try:
        # Initialize client
        client = Garmin()
        
        # Try logging in using tokenstore
        try:
            if os.listdir(tokenstore):
                client.login(tokenstore=tokenstore)
        except Exception:
            # Tokenstore login failed, try with credentials
            if not email or not password:
                print(json.dumps({"error": "Garmin session expired and no credentials provided"}))
                sys.exit(1)
            
            # If MFA is required, the library prompts on stdin.
            # We bypass prompt and let it throw exception if stdin is closed,
            # or it can take stdin if run interactively.
            client = Garmin(email=email, password=password)
            client.login(tokenstore=tokenstore)

        activities = client.get_activities_by_date(start_date_str, end_date_str)
        print(json.dumps(activities))
    except Exception as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)

if __name__ == "__main__":
    main()
`

// GetGarminHelperPath returns the path to the garmin helper script in the config dir
// Check logic is clean.
func GetGarminHelperPath() (string, error) {
	dir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "garmin_helper.py"), nil
}

// WriteGarminHelper writes the python helper script to the config directory
func WriteGarminHelper() error {
	path, err := GetGarminHelperPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(GarminHelperScript), 0755)
}

// FetchGarminActivities fetches activities from Garmin Connect via the python helper
func FetchGarminActivities(email, password string, start, end time.Time) ([]*models.Activity, error) {
	if err := WriteGarminHelper(); err != nil {
		return nil, fmt.Errorf("failed to write Garmin python helper: %w", err)
	}

	helperPath, err := GetGarminHelperPath()
	if err != nil {
		return nil, err
	}

	configDir, err := config.GetConfigDir()
	if err != nil {
		return nil, err
	}
	tokenstore := filepath.Join(configDir, "garmin_session")
	if err := os.MkdirAll(tokenstore, 0755); err != nil {
		return nil, err
	}

	startDateStr := start.Format("2006-01-02")
	endDateStr := end.Format("2006-01-02")

	// Determine python binary. Try workspace venv first, then system python3
	pythonCmd := "python3"
	if runtime.GOOS == "windows" {
		pythonCmd = "python"
	}

	// Try checking if virtualenv python exists
	cwd, err := os.Getwd()
	if err == nil {
		venvPython := filepath.Join(cwd, "venv", "bin", "python3")
		if runtime.GOOS == "windows" {
			venvPython = filepath.Join(cwd, "venv", "Scripts", "python.exe")
		}
		if _, err := os.Stat(venvPython); err == nil {
			pythonCmd = venvPython
		} else {
			// Check in legacy_python venv
			legacyVenvPython := filepath.Join(cwd, "legacy_python", "venv", "bin", "python3")
			if _, err := os.Stat(legacyVenvPython); err == nil {
				pythonCmd = legacyVenvPython
			}
		}
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(pythonCmd, helperPath, email, password, startDateStr, endDateStr, tokenstore)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin // Pass stdin in case Garmin prompts for MFA code

	err = cmd.Run()
	if err != nil {
		// If command execution failed (e.g. python not found)
		return nil, fmt.Errorf("python execution error: %s (stderr: %s)", err.Error(), stderr.String())
	}

	output := stdout.Bytes()

	// Parse either as error dict or activity slice
	var errResponse struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(output, &errResponse); err == nil && errResponse.Error != "" {
		// If it's a garminconnect dependency missing error, suggest install
		if strings.Contains(errResponse.Error, "No module named 'garminconnect'") {
			return nil, fmt.Errorf("Garmin Connect python dependency missing. Please run: pip install garminconnect")
		}
		return nil, fmt.Errorf("Garmin Connect error: %s", errResponse.Error)
	}

	var rawActivities []map[string]interface{}
	if err := json.Unmarshal(output, &rawActivities); err != nil {
		return nil, fmt.Errorf("failed to parse Garmin activities output: %w (output: %s)", err, string(output))
	}

	var list []*models.Activity
	for _, raw := range rawActivities {
		act, err := ParseGarminActivity(raw)
		if err != nil {
			// Skip or log parsing errors
			continue
		}
		list = append(list, act)
	}

	return list, nil
}

func ParseGarminActivity(data map[string]interface{}) (*models.Activity, error) {
	activityIDVal, ok := data["activityId"]
	if !ok {
		return nil, fmt.Errorf("missing activityId")
	}

	var activityID int64
	switch v := activityIDVal.(type) {
	case float64:
		activityID = int64(v)
	case int64:
		activityID = v
	default:
		return nil, fmt.Errorf("invalid activityId type")
	}

	title, _ := data["activityName"].(string)
	if title == "" {
		title = "Garmin Activity"
	}

	// Parse sport type
	sport := "Other"
	if activityType, ok := data["activityType"].(map[string]interface{}); ok {
		typeKey, _ := activityType["typeKey"].(string)
		typeKey = strings.ToLower(typeKey)
		if strings.Contains(typeKey, "running") || strings.Contains(typeKey, "run") {
			sport = "Run"
		} else if strings.Contains(typeKey, "cycling") || strings.Contains(typeKey, "biking") || strings.Contains(typeKey, "ride") {
			sport = "Ride"
		} else if strings.Contains(typeKey, "swimming") || strings.Contains(typeKey, "swim") {
			sport = "Swim"
		} else if strings.Contains(typeKey, "walking") || strings.Contains(typeKey, "walk") {
			sport = "Walk"
		} else if strings.Contains(typeKey, "hiking") || strings.Contains(typeKey, "hike") {
			sport = "Hike"
		} else {
			if typeKey != "" {
				sport = strings.Title(typeKey)
			}
		}
	}

	// Parse start time (e.g. "2026-05-22 07:16:33")
	startTimeStr, _ := data["startTimeLocal"].(string)
	var startTime time.Time
	var err error
	fmts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, f := range fmts {
		if startTime, err = time.ParseInLocation(f, startTimeStr, time.Local); err == nil {
			break
		}
	}
	if err != nil {
		startTime = time.Now()
	}

	distance, _ := data["distance"].(float64)
	duration, _ := data["duration"].(float64)

	var avgHR, maxHR *float64
	if val, ok := data["averageHR"].(float64); ok {
		avgHR = &val
	}
	if val, ok := data["maxHR"].(float64); ok {
		maxHR = &val
	}

	elevationGain, _ := data["elevationGain"].(float64)
	description, _ := data["description"].(string)
	locationName, _ := data["locationName"].(string)

	var latitude, longitude *float64
	if val, ok := data["startLatitude"].(float64); ok {
		latitude = &val
	}
	if val, ok := data["startLongitude"].(float64); ok {
		longitude = &val
	}

	if sport == "Run" && elevationGain >= 365.76 {
		sport = "Trail Run"
	}

	isRace := false
	if eventType, ok := data["eventType"].(map[string]interface{}); ok {
		typeKey, _ := eventType["typeKey"].(string)
		isRace = strings.ToLower(typeKey) == "race"
	}
	if strings.Contains(strings.ToLower(title), "race") {
		isRace = true
	}

	rawJSON, _ := json.Marshal(data)

	return &models.Activity{
		ID:                  activityID,
		Provider:            "garmin",
		Sport:               sport,
		Title:               title,
		StartTime:           startTime,
		DistanceMeters:      distance,
		DurationSeconds:     duration,
		AvgHR:               avgHR,
		MaxHR:               maxHR,
		ElevationGainMeters: elevationGain,
		Description:         description,
		LocationName:        locationName,
		Latitude:            latitude,
		Longitude:           longitude,
		IsRace:              isRace,
		RawData:             string(rawJSON),
	}, nil
}
