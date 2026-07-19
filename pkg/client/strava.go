package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"running_cli/pkg/config"
	"running_cli/pkg/models"
)

// StravaTokenResponse maps the response from token exchange or refresh
type StravaTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshStravaToken exchanges the refresh token for a new access token
func RefreshStravaToken(clientID, clientSecret, refreshToken string) (string, error) {
	apiURL := "https://www.strava.com/oauth/token"
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	resp, err := http.PostForm(apiURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to call Strava token endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Strava token refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var tr StravaTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("failed to decode Strava token response: %w", err)
	}

	// If the refresh token was updated, save it to the config
	if tr.RefreshToken != "" && tr.RefreshToken != refreshToken {
		cfg, err := config.LoadConfig()
		if err == nil {
			cfg.Strava.RefreshToken = tr.RefreshToken
			config.SaveConfig(cfg)
		}
	}

	return tr.AccessToken, nil
}

// ExchangeStravaAuthCode exchanges authorization code for tokens
func ExchangeStravaAuthCode(clientID, clientSecret, authCode string) (*StravaTokenResponse, error) {
	apiURL := "https://www.strava.com/oauth/token"
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", authCode)
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm(apiURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange Strava auth code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Strava token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tr StravaTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("failed to decode exchange response: %w", err)
	}

	return &tr, nil
}

// StartStravaAuthServer spins up a local server to capture redirect authorization code
func StartStravaAuthServer() (string, error) {
	server := &http.Server{Addr: ":8000"}
	authCodeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body><h1 style='color:#008000;'>Authorization Successful!</h1><p>You can close this browser tab and return to the terminal.</p></body></html>"))
			authCodeChan <- code
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("<html><body><h1 style='color:#FF0000;'>Error</h1><p>No authorization code found in URL redirect.</p></body></html>"))
			errChan <- fmt.Errorf("no authorization code in redirect query")
		}
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for code or error
	var capturedCode string
	var err error
	select {
	case capturedCode = <-authCodeChan:
		// Success
	case err = <-errChan:
		// Server error
	case <-time.After(120 * time.Second):
		err = fmt.Errorf("timeout waiting for browser authorization (2 minutes)")
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	return capturedCode, err
}

// FetchStravaActivities fetches activities from Strava API for the date range
func FetchStravaActivities(clientID, clientSecret, refreshToken string, start, end time.Time) ([]*models.Activity, error) {
	accessToken, err := RefreshStravaToken(clientID, clientSecret, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Strava token: %w", err)
	}

	afterEpoch := start.Unix()
	beforeEpoch := end.Unix()

	var allActivities []map[string]interface{}
	page := 1

	for {
		apiURL := fmt.Sprintf("https://www.strava.com/api/v3/athlete/activities?before=%d&after=%d&page=%d&per_page=100", beforeEpoch, afterEpoch, page)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Strava page %d: %w", page, err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("Strava activities call failed (%d): %s", resp.StatusCode, string(body))
		}

		var pageActs []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&pageActs); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to parse page %d output: %w", page, err)
		}
		resp.Body.Close()

		if len(pageActs) == 0 {
			break
		}

		allActivities = append(allActivities, pageActs...)

		if len(pageActs) < 100 {
			break
		}

		page++
	}

	var list []*models.Activity
	for _, raw := range allActivities {
		act, err := parseStravaActivity(raw)
		if err != nil {
			continue
		}
		list = append(list, act)
	}

	return list, nil
}

func parseStravaActivity(data map[string]interface{}) (*models.Activity, error) {
	idVal, ok := data["id"]
	if !ok {
		return nil, fmt.Errorf("missing activity id")
	}

	var activityID int64
	switch v := idVal.(type) {
	case float64:
		activityID = int64(v)
	case int64:
		activityID = v
	default:
		return nil, fmt.Errorf("invalid activity id type")
	}

	title, _ := data["name"].(string)
	if title == "" {
		title = "Strava Activity"
	}

	// Parse sport type
	sportType := ""
	if st, ok := data["sport_type"].(string); ok {
		sportType = strings.ToLower(st)
	} else if t, ok := data["type"].(string); ok {
		sportType = strings.ToLower(t)
	}

	sport := "Other"
	if strings.Contains(sportType, "run") {
		sport = "Run"
	} else if strings.Contains(sportType, "ride") || strings.Contains(sportType, "cycling") {
		sport = "Ride"
	} else if strings.Contains(sportType, "swim") {
		sport = "Swim"
	} else if strings.Contains(sportType, "walk") {
		sport = "Walk"
	} else if strings.Contains(sportType, "hike") {
		sport = "Hike"
	} else {
		if sportType != "" {
			sport = strings.Title(sportType)
		}
	}

	// Parse start time (e.g. "2026-05-22T07:16:33Z")
	startTimeStr, _ := data["start_date_local"].(string)
	var startTime time.Time
	var err error
	fmts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, f := range fmts {
		if startTime, err = time.Parse(f, startTimeStr); err == nil {
			break
		}
	}
	if err != nil {
		startTime = time.Now()
	}

	distance, _ := data["distance"].(float64)
	duration, _ := data["moving_time"].(float64)
	if duration <= 0 {
		duration, _ = data["elapsed_time"].(float64)
	}

	var avgHR, maxHR *float64
	if val, ok := data["average_heartrate"].(float64); ok {
		avgHR = &val
	}
	if val, ok := data["max_heartrate"].(float64); ok {
		maxHR = &val
	}

	elevationGain, _ := data["total_elevation_gain"].(float64)
	description, _ := data["description"].(string)

	locationName := ""
	city, _ := data["location_city"].(string)
	state, _ := data["location_state"].(string)
	if city != "" && state != "" {
		locationName = city + ", " + state
	} else if city != "" {
		locationName = city
	} else if state != "" {
		locationName = state
	}

	var latitude, longitude *float64
	if startLatLng, ok := data["start_latlng"].([]interface{}); ok && len(startLatLng) >= 2 {
		if lat, ok := startLatLng[0].(float64); ok {
			latitude = &lat
		}
		if lon, ok := startLatLng[1].(float64); ok {
			longitude = &lon
		}
	}

	// Parse workout type for race status (workout_type = 1 is race)
	isRace := false
	if wt, ok := data["workout_type"].(float64); ok {
		isRace = int(wt) == 1
	}

	rawJSON, _ := json.Marshal(data)

	return &models.Activity{
		ID:                  activityID,
		Provider:            "strava",
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
