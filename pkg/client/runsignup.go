package client

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"running_cli/pkg/models"
)

type RunSignupEvent struct {
	EventID            string `json:"event_id"`
	Name               string `json:"name"`
	Distance           string `json:"distance"`
	Volunteer          string `json:"volunteer"`
	StartTime          string `json:"start_time"`
	RegistrationOpen   string `json:"registration_opens"`
	RegistrationClose  string `json:"registration_closes"`
	RegistrationPeriods []struct {
		Opens   string `json:"registration_opens"`
		Closes  string `json:"registration_closes"`
		RaceFee string `json:"race_fee"`
	} `json:"registration_periods"`
}

type RunSignupRace struct {
	RaceID        int             `json:"race_id"`
	Name          string          `json:"name"`
	NextDate      string          `json:"next_date"`
	IsDraftRace   string          `json:"is_draft_race"`
	IsPrivateRace string          `json:"is_private_race"`
	Address       struct {
		City  string `json:"city"`
		State string `json:"state"`
	} `json:"address"`
	Events        []RunSignupEvent `json:"events"`
	URL           string          `json:"url"`
}

type RunSignupResponse struct {
	Races []struct {
		Race RunSignupRace `json:"race"`
	} `json:"races"`
}

// FetchFutureRaces queries RunSignup API for upcoming events matching criteria
func FetchFutureRaces(zipcode string, radius int, start, end time.Time) ([]*models.FutureRace, error) {
	var collected []RunSignupRace
	page := 1

	startDateStr := start.Format("2006-01-02")
	endDateStr := end.Format("2006-01-02")

	for {
		apiURL := fmt.Sprintf(
			"https://api.runsignup.com/rest/races?format=json&zipcode=%s&radius=%d&start_date=%s&end_date=%s&events=T&results_per_page=100&page=%d",
			zipcode, radius, startDateStr, endDateStr, page,
		)

		resp, err := http.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch RunSignup API page %d: %w", page, err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("RunSignup API returned status %d", resp.StatusCode)
		}

		var data RunSignupResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode RunSignup page %d: %w", page, err)
		}
		resp.Body.Close()

		if len(data.Races) == 0 {
			break
		}

		for _, r := range data.Races {
			collected = append(collected, r.Race)
		}

		if len(data.Races) < 100 {
			break
		}
		page++
	}

	var results []*models.FutureRace
	for _, race := range collected {
		if race.IsDraftRace == "T" || race.IsPrivateRace == "T" {
			continue
		}

		if race.NextDate == "" {
			continue
		}

		raceDate, err := time.Parse("01/02/2006", race.NextDate)
		if err != nil {
			continue
		}

		if raceDate.Before(start) || raceDate.After(end) {
			continue
		}

		// Filter for matching distances and collect display strings
		var matchingDists []string
		for _, ev := range race.Events {
			if ev.Volunteer == "T" {
				continue
			}
			nameLower := strings.ToLower(ev.Name)
			if strings.Contains(nameLower, "virtual") {
				continue
			}

			// Ensure event matches the next race date
			if ev.StartTime != "" && !strings.HasPrefix(ev.StartTime, race.NextDate) {
				continue
			}

			normDist := getNormalizedDistance(ev.Name, ev.Distance)
			if normDist != "" {
				// Detail status/fee parsing can be printed or saved, but for models.FutureRace we store main events
				matchingDists = append(matchingDists, normDist)
			}
		}

		if len(matchingDists) == 0 {
			continue
		}

		// Deduplicate matching distances
		distMap := make(map[string]bool)
		var uniqueDists []string
		for _, d := range matchingDists {
			if !distMap[d] {
				distMap[d] = true
				uniqueDists = append(uniqueDists, d)
			}
		}

		location := fmt.Sprintf("%s, %s", race.Address.City, race.Address.State)
		if race.Address.City == "" {
			location = race.Address.State
		}

		// Check if we can geocode coordinates for sorting/mapping (optional default 0)
		lat, lon := 0.0, 0.0
		geocoded, err := GeocodeLocation(location)
		if err == nil {
			lat, _ = geocoded["lat"].(float64)
			lon, _ = geocoded["lon"].(float64)
		}

		results = append(results, &models.FutureRace{
			Title:           race.Name,
			RaceDate:        raceDate,
			DistanceDisplay: strings.Join(uniqueDists, ", "),
			Location:        location,
			WebsiteURL:      race.URL,
			Latitude:        lat,
			Longitude:       lon,
		})
	}

	return results, nil
}

func getNormalizedDistance(eventName, distanceStr string) string {
	text := strings.ToLower(eventName + " " + distanceStr)
	if strings.Contains(text, "half marathon") || strings.Contains(text, "half-marathon") || strings.Contains(text, "13.1") {
		return "Half Marathon"
	}
	if strings.Contains(text, "marathon") || strings.Contains(text, "26.2") {
		return "Marathon"
	}
	if strings.Contains(text, "10k") || strings.Contains(text, "10 k") || strings.Contains(text, "6.2") {
		return "10K"
	}
	if strings.Contains(text, "5k") || strings.Contains(text, "5 k") || strings.Contains(text, "3.1") {
		return "5K"
	}
	return ""
}

// GetRaceCapacities fetches details and participant capacities for a race
func GetRaceCapacities(raceID int, eventsToCheck []RunSignupEvent) map[string]string {
	caps := make(map[string]int)
	detailURL := fmt.Sprintf("https://api.runsignup.com/rest/race/%d?format=json&include_participant_caps=T", raceID)

	resp, err := http.Get(detailURL)
	if err == nil {
		defer resp.Body.Close()
		var detailRes struct {
			Race struct {
				Events []struct {
					EventID        string      `json:"event_id"`
					ParticipantCap interface{} `json:"participant_cap"` // cap can be string or int in JSON sometimes
				} `json:"events"`
			} `json:"race"`
		}
		if json.NewDecoder(resp.Body).Decode(&detailRes) == nil {
			for _, ev := range detailRes.Race.Events {
				if ev.ParticipantCap != nil {
					var capVal int
					switch v := ev.ParticipantCap.(type) {
					case float64:
						capVal = int(v)
					case string:
						capVal, _ = strconv.Atoi(v)
					}
					if capVal > 0 {
						caps[ev.EventID] = capVal
					}
				}
			}
		}
	}

	// Scrape total registered count from HTML FindARunner list page
	findURL := fmt.Sprintf("https://runsignup.com/Race/FindARunner/?raceId=%d", raceID)
	counts := make(map[string]int)
	totalParticipants := -1

	respHTML, errHTML := http.Get(findURL)
	if errHTML == nil {
		defer respHTML.Body.Close()
		bodyBytes, _ := io.ReadAll(respHTML.Body)
		htmlContent := string(bodyBytes)

		reDtDd := regexp.MustCompile(`(?s)<dt>\s*(.*?)\s*:</dt>\s*<dd>\s*(\d+)\s*</dd>`)
		matches := reDtDd.FindAllStringSubmatch(htmlContent, -1)
		for _, m := range matches {
			if len(m) >= 3 {
				cnt, _ := strconv.Atoi(m[2])
				counts[strings.ToLower(strings.TrimSpace(m[1]))] = cnt
			}
		}

		reTotal := regexp.MustCompile(`Total Event Participants:\s*<b>\s*(\d+)\s*</b>`)
		totalMatch := reTotal.FindStringSubmatch(htmlContent)
		if len(totalMatch) >= 2 {
			totalParticipants, _ = strconv.Atoi(totalMatch[1])
		}
	}

	results := make(map[string]string)
	for _, ev := range eventsToCheck {
		count, ok := counts[strings.ToLower(ev.Name)]
		if !ok {
			if len(eventsToCheck) == 1 && totalParticipants != -1 {
				count = totalParticipants
			} else {
				count = 0
			}
		}

		capVal, hasCap := caps[ev.EventID]
		if hasCap {
			pct := 0.0
			if capVal > 0 {
				pct = (float64(count) / float64(capVal)) * 100.0
			}
			if pct >= 100.0 {
				results[ev.EventID] = fmt.Sprintf("Sold Out (%d/%d)", count, capVal)
			} else {
				results[ev.EventID] = fmt.Sprintf("%d/%d (%.1f%%)", count, capVal, pct)
			}
		} else {
			if count > 0 {
				results[ev.EventID] = fmt.Sprintf("%d reg", count)
			} else {
				results[ev.EventID] = "Open"
			}
		}
	}

	return results
}

// GetEventFee returns current fee
func GetEventFee(ev RunSignupEvent, currentTime time.Time) string {
	if len(ev.RegistrationPeriods) == 0 {
		return ""
	}

	for _, p := range ev.RegistrationPeriods {
		if p.Opens == "" || p.Closes == "" || p.RaceFee == "" {
			continue
		}

		fmts := []string{
			"01/02/2006 15:04",
			"2006-01-02 15:04:05",
		}

		var opensTime, closesTime time.Time
		var errOpen, errClose error
		for _, f := range fmts {
			if opensTime, errOpen = time.Parse(f, p.Opens); errOpen == nil {
				break
			}
		}
		for _, f := range fmts {
			if closesTime, errClose = time.Parse(f, p.Closes); errClose == nil {
				break
			}
		}

		if errOpen == nil && errClose == nil {
			if (currentTime.After(opensTime) || currentTime.Equal(opensTime)) && (currentTime.Before(closesTime) || currentTime.Equal(closesTime)) {
				return p.RaceFee
			}
		}
	}

	return ev.RegistrationPeriods[0].RaceFee
}

// GetEventStatus returns description of reg status
func GetEventStatus(ev RunSignupEvent, currentTime time.Time) string {
	if len(ev.RegistrationPeriods) == 0 {
		return "Open"
	}

	type parsedPeriod struct {
		openTime  time.Time
		closeTime time.Time
		fee       float64
		feeStr    string
	}

	var parsed []parsedPeriod
	for _, p := range ev.RegistrationPeriods {
		if p.Opens == "" || p.Closes == "" || p.RaceFee == "" {
			continue
		}

		fmts := []string{
			"01/02/2006 15:04",
			"2006-01-02 15:04:05",
		}

		var opensTime, closesTime time.Time
		var errOpen, errClose error
		for _, f := range fmts {
			if opensTime, errOpen = time.Parse(f, p.Opens); errOpen == nil {
				break
			}
		}
		for _, f := range fmts {
			if closesTime, errClose = time.Parse(f, p.Closes); errClose == nil {
				break
			}
		}

		if errOpen == nil && errClose == nil {
			cleanFee := strings.ReplaceAll(p.RaceFee, "$", "")
			cleanFee = strings.ReplaceAll(cleanFee, ",", "")
			feeFloat, _ := strconv.ParseFloat(cleanFee, 64)
			parsed = append(parsed, parsedPeriod{
				openTime:  opensTime,
				closeTime: closesTime,
				fee:       feeFloat,
				feeStr:    p.RaceFee,
			})
		}
	}

	if len(parsed) == 0 {
		return "Open"
	}

	// Sort by opensTime
	for i := 0; i < len(parsed); i++ {
		for j := i + 1; j < len(parsed); j++ {
			if parsed[i].openTime.After(parsed[j].openTime) {
				parsed[i], parsed[j] = parsed[j], parsed[i]
			}
		}
	}

	var activePeriod *parsedPeriod
	var nextPeriod *parsedPeriod

	for idx, p := range parsed {
		if (currentTime.After(p.openTime) || currentTime.Equal(p.openTime)) && (currentTime.Before(p.closeTime) || currentTime.Equal(p.closeTime)) {
			activePeriod = &parsed[idx]
			if idx+1 < len(parsed) {
				nextPeriod = &parsed[idx+1]
			}
			break
		}
	}

	if activePeriod != nil {
		if nextPeriod != nil && nextPeriod.fee > activePeriod.fee {
			return fmt.Sprintf("Open (Price increases %s)", activePeriod.closeTime.Format("Jan 02"))
		}
		return "Open"
	}

	allFuture := true
	for _, p := range parsed {
		if !p.openTime.After(currentTime) {
			allFuture = false
			break
		}
	}
	if allFuture {
		return fmt.Sprintf("TBD (Reg opens %s)", parsed[0].openTime.Format("Jan 02"))
	}

	allPast := true
	for _, p := range parsed {
		if !p.closeTime.Before(currentTime) {
			allPast = false
			break
		}
	}
	if allPast {
		return "Closed"
	}

	return "Open"
}

// HaversineDistance calculates distance in miles between two coordinates
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 3958.8 // Radius of earth in miles
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return R * c
}
