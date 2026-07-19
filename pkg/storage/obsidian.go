package storage

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"running_cli/pkg/client"
	"running_cli/pkg/models"

	"gopkg.in/yaml.v3"
)

var SportEmojis = map[string]string{
	"Run":   "🏃",
	"Ride":  "🚴",
	"Swim":  "🏊",
	"Walk":  "🚶",
	"Hike":  "🥾",
	"Other": "🏋️",
}

// GetMondayOfWeek returns the date of Monday in the same week
func GetMondayOfWeek(d time.Time) time.Time {
	offset := int(d.Weekday()) - int(time.Monday)
	if offset < 0 {
		offset += 7
	}
	return d.AddDate(0, 0, -offset)
}

func FormatDuration(seconds float64) string {
	hrs := int(seconds / 3600.0)
	mins := int(math.Mod(seconds, 3600.0) / 60.0)
	secs := int(math.Mod(seconds, 60.0))
	if hrs > 0 {
		return fmt.Sprintf("%dh %02dm", hrs, mins)
	}
	return fmt.Sprintf("%dm %02ds", mins, secs)
}

func FormatDurationCompact(seconds float64) string {
	hrs := int(seconds / 3600.0)
	mins := int(math.Mod(seconds, 3600.0) / 60.0)
	secs := int(math.Mod(seconds, 60.0))
	if hrs > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hrs, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func FormatDistance(sport string, meters float64, unit string) string {
	sportLower := strings.ToLower(sport)
	if sportLower == "swim" {
		if unit == "miles" {
			yards := meters * 1.09361
			return fmt.Sprintf("%d yd", int(math.Round(yards)))
		}
		return fmt.Sprintf("%d m", int(math.Round(meters)))
	}

	if unit == "miles" {
		miles := meters * 0.000621371
		return fmt.Sprintf("%.2f mi", miles)
	}
	km := meters * 0.001
	return fmt.Sprintf("%.2f km", km)
}

func FormatPaceOrSpeed(sport string, distanceMeters, durationSeconds float64, unit string) string {
	if distanceMeters <= 0 || durationSeconds <= 0 {
		return "-"
	}

	sportLower := strings.ToLower(sport)
	if sportLower == "run" || sportLower == "walk" || sportLower == "hike" {
		var dist float64
		label := "/mi"
		if unit == "miles" {
			dist = distanceMeters * 0.000621371
		} else {
			dist = distanceMeters * 0.001
			label = "/km"
		}

		if dist <= 0 {
			return "-"
		}

		totalMinutes := durationSeconds / 60.0
		paceDec := totalMinutes / dist
		paceMin := int(paceDec)
		paceSec := int((paceDec - float64(paceMin)) * 60)
		return fmt.Sprintf("%d:%02d%s", paceMin, paceSec, label)
	}

	if sportLower == "ride" || sportLower == "cycling" {
		var dist float64
		label := " mph"
		if unit == "miles" {
			dist = distanceMeters * 0.000621371
		} else {
			dist = distanceMeters * 0.001
			label = " km/h"
		}

		hours := durationSeconds / 3600.0
		if hours <= 0 {
			return "-"
		}
		speed := dist / hours
		return fmt.Sprintf("%.1f%s", speed, label)
	}

	if sportLower == "swim" {
		if unit == "miles" {
			yards := distanceMeters * 1.09361
			if yards <= 0 {
				return "-"
			}
			hundredYards := yards / 100.0
			totalMinutes := durationSeconds / 60.0
			paceDec := totalMinutes / hundredYards
			paceMin := int(paceDec)
			paceSec := int((paceDec - float64(paceMin)) * 60)
			return fmt.Sprintf("%d:%02d/100yd", paceMin, paceSec)
		}

		if distanceMeters <= 0 {
			return "-"
		}
		hundredMeters := distanceMeters / 100.0
		totalMinutes := durationSeconds / 60.0
		paceDec := totalMinutes / hundredMeters
		paceMin := int(paceDec)
		paceSec := int((paceDec - float64(paceMin)) * 60)
		return fmt.Sprintf("%d:%02d/100m", paceMin, paceSec)
	}

	// default speed
	var dist float64
	label := " mph"
	if unit == "miles" {
		dist = distanceMeters * 0.000621371
	} else {
		dist = distanceMeters * 0.001
		label = " km/h"
	}
	hours := durationSeconds / 3600.0
	if hours <= 0 {
		return "-"
	}
	speed := dist / hours
	return fmt.Sprintf("%.1f%s", speed, label)
}

func FormatElevation(meters float64, unit string) string {
	if meters <= 0 {
		return "-"
	}
	if unit == "miles" {
		feet := meters * 3.28084
		return fmt.Sprintf("+%d ft", int(math.Round(feet)))
	}
	return fmt.Sprintf("+%d m", int(math.Round(meters)))
}

func CleanFilename(title string) string {
	reg := regexp.MustCompile(`[\\/*?:"<>|]`)
	return strings.TrimSpace(reg.ReplaceAllString(title, ""))
}

// SafeUpdateMarkdown updates YAML frontmatter and placeholder contents, preserving custom notes
func SafeUpdateMarkdown(fileContent string, newFrontmatter map[string]interface{}, newSummary string, startTag, endTag, missingTagsTemplate string) string {
	// Find frontmatter
	reFront := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n(.*)$`)
	matches := reFront.FindStringSubmatch(fileContent)

	existingYAML := make(map[string]interface{})
	body := fileContent

	if len(matches) >= 3 {
		yamlContent := matches[1]
		body = matches[2]
		yaml.Unmarshal([]byte(yamlContent), &existingYAML)
	}

	// Merge frontmatter keys
	for k, v := range newFrontmatter {
		existingYAML[k] = v
	}

	updatedYAMLBytes, err := yaml.Marshal(existingYAML)
	updatedYAMLStr := ""
	if err == nil {
		updatedYAMLStr = strings.TrimSpace(string(updatedYAMLBytes))
	} else {
		// Fallback manual representation
		var lines []string
		for k, v := range existingYAML {
			lines = append(lines, fmt.Sprintf("%s: %v", k, v))
		}
		updatedYAMLStr = strings.Join(lines, "\n")
	}

	tagRegex := regexp.MustCompile(fmt.Sprintf(`(?s)%s.*?%s`, regexp.QuoteMeta(startTag), regexp.QuoteMeta(endTag)))
	replacement := fmt.Sprintf("%s\n\n%s\n\n%s", startTag, newSummary, endTag)

	updatedBody := ""
	if tagRegex.MatchString(body) {
		updatedBody = tagRegex.ReplaceAllString(body, replacement)
	} else {
		// Insert tags based on missing template
		updatedBody = strings.Replace(missingTagsTemplate, "{replacement}", replacement, 1)
		updatedBody = strings.Replace(updatedBody, "{body}", strings.TrimLeft(body, " \t\n\r"), 1)
	}

	return fmt.Sprintf("---\n%s\n---\n\n%s", updatedYAMLStr, strings.TrimLeft(updatedBody, " \t\n\r"))
}

// WriteWeeklyNote formats and writes a weekly activity summary file
func WriteWeeklyNote(vaultPath, folder string, mondayDate time.Time, activities []*models.MergedActivity, unit string) (string, error) {
	// Sort activities chronologically
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].StartTime.Before(activities[j].StartTime)
	})

	type stats struct {
		distance float64
		duration float64
		count    int
	}

	totalsBySport := make(map[string]*stats)
	totalDuration := 0.0
	sourcesMap := make(map[string]bool)

	for _, a := range activities {
		totalDuration += a.DurationSeconds
		for _, s := range a.Sources {
			sourcesMap[s] = true
		}

		if _, ok := totalsBySport[a.Sport]; !ok {
			totalsBySport[a.Sport] = &stats{}
		}
		totalsBySport[a.Sport].distance += a.DistanceMeters
		totalsBySport[a.Sport].duration += a.DurationSeconds
		totalsBySport[a.Sport].count++
	}

	frontmatterDistances := make(map[string]float64)
	for sport, st := range totalsBySport {
		var val float64
		if strings.ToLower(sport) == "swim" {
			mult := 1.0
			if unit == "miles" {
				mult = 1.09361 // yards
			}
			val = st.distance * mult
		} else {
			mult := 0.001 // km
			if unit == "miles" {
				mult = 0.000621371 // miles
			}
			val = st.distance * mult
		}
		frontmatterDistances[strings.ToLower(sport)] = math.Round(val*100) / 100
	}

	var sources []string
	for s := range sourcesMap {
		sources = append(sources, s)
	}
	sort.Strings(sources)

	newFrontmatter := map[string]interface{}{
		"type":                    "weekly-activities",
		"week_of":                 mondayDate.Format("2006-01-02"),
		"activity_count":          len(activities),
		"total_duration_seconds":  int(totalDuration),
		"total_duration_friendly": FormatDuration(totalDuration),
		"distances":               frontmatterDistances,
		"distance_unit":           unit,
		"sources":                 sources,
	}

	// Build markdown breakdown lines
	var breakdownLines []string
	var sportsSorted []string
	for sport := range totalsBySport {
		sportsSorted = append(sportsSorted, sport)
	}
	sort.Strings(sportsSorted)

	for _, sport := range sportsSorted {
		st := totalsBySport[sport]
		emoji := SportEmojis[sport]
		if emoji == "" {
			emoji = SportEmojis["Other"]
		}
		distStr := FormatDistance(sport, st.distance, unit)
		durStr := FormatDuration(st.duration)
		actLabel := "activity"
		if st.count != 1 {
			actLabel = "activities"
		}
		breakdownLines = append(breakdownLines, fmt.Sprintf("- **%s %s**: %s | %s | %d %s", emoji, sport, distStr, durStr, st.count, actLabel))
	}
	breakdownSection := strings.Join(breakdownLines, "\n")

	// Build activity log table
	tableHeaders := []string{"Date", "Sport", "Title", "Distance", "Time", "Pace/Speed", "HR (Avg/Max)", "Elev", "Location", "Source"}
	var tableRows []string
	tableRows = append(tableRows, "| "+strings.Join(tableHeaders, " | ")+" |")
	tableRows = append(tableRows, "| "+strings.Join([]string{"---", "---", "---", "---", "---", "---", "---", "---", "---", "---"}, " | ")+" |")

	var notesSectionLines []string

	for _, a := range activities {
		dateStr := a.StartTime.Format("Mon, Jan 02")
		emoji := SportEmojis[a.Sport]
		if emoji == "" {
			emoji = SportEmojis["Other"]
		}
		sportStr := fmt.Sprintf("%s %s", emoji, a.Sport)
		distStr := FormatDistance(a.Sport, a.DistanceMeters, unit)
		timeStr := FormatDurationCompact(a.DurationSeconds)
		paceStr := FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, unit)

		hrStr := "-"
		if a.AvgHR != nil {
			maxHRVal := ""
			if a.MaxHR != nil {
				maxHRVal = fmt.Sprintf("/%d", int(*a.MaxHR))
			}
			hrStr = fmt.Sprintf("%d%s", int(*a.AvgHR), maxHRVal)
		}

		elevStr := FormatElevation(a.ElevationGainMeters, unit)

		locStr := "-"
		if a.Latitude != nil && a.Longitude != nil {
			mapURL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%.6f,%.6f", *a.Latitude, *a.Longitude)
			locName := a.LocationName
			if locName == "" {
				locName = "Map"
			}
			locStr = fmt.Sprintf("📍 [%s](%s)", locName, mapURL)
		}

		var srcLinks []string
		for _, s := range a.Sources {
			if s == "garmin" && a.GarminID != nil {
				srcLinks = append(srcLinks, fmt.Sprintf("[Garmin](https://connect.garmin.com/modern/activity/%d)", *a.GarminID))
			} else if s == "strava" && a.StravaID != nil {
				srcLinks = append(srcLinks, fmt.Sprintf("[Strava](https://www.strava.com/activities/%d)", *a.StravaID))
			} else {
				srcLinks = append(srcLinks, strings.Title(s))
			}
		}
		srcStr := strings.Join(srcLinks, ", ")

		// Format race or trail links
		cleanedTitle := CleanFilename(a.Title)
		dateISO := a.StartTime.Format("2006-01-02")
		isTrail := a.Sport == "Run" && a.ElevationGainMeters >= 609.6 && hasSource(a.Sources, "strava")

		titleStr := a.Title
		if a.IsRace && isTrail {
			titleStr = fmt.Sprintf("[[Races/%s - %s|%s]] (also [[TrailRuns/%s - %s|Trail Run]])", dateISO, cleanedTitle, a.Title, dateISO, cleanedTitle)
		} else if a.IsRace {
			titleStr = fmt.Sprintf("[[Races/%s - %s|%s]]", dateISO, cleanedTitle, a.Title)
		} else if isTrail {
			titleStr = fmt.Sprintf("[[TrailRuns/%s - %s|%s]]", dateISO, cleanedTitle, a.Title)
		}

		rowFields := []string{dateStr, sportStr, titleStr, distStr, timeStr, paceStr, hrStr, elevStr, locStr, srcStr}
		tableRows = append(tableRows, "| "+strings.Join(rowFields, " | ")+" |")

		// Compile descriptions/notes
		if strings.TrimSpace(a.Description) != "" {
			notesSectionLines = append(notesSectionLines, "<details>")
			notesSectionLines = append(notesSectionLines, fmt.Sprintf("<summary><b>%s - %s %s</b></summary>", dateStr, emoji, a.Title))
			notesSectionLines = append(notesSectionLines, "")
			notesSectionLines = append(notesSectionLines, strings.TrimSpace(a.Description))
			notesSectionLines = append(notesSectionLines, "</details>")
		}
	}

	notesSection := ""
	if len(notesSectionLines) > 0 {
		notesSection = "\n\n### Activity Notes\n" + strings.Join(notesSectionLines, "\n")
	}

	tableSection := strings.Join(tableRows, "\n")
	newSummary := fmt.Sprintf("### Weekly Breakdown\n%s\n\n### Activity Log\n%s%s", breakdownSection, tableSection, notesSection)

	destDir := filepath.Join(vaultPath, folder)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("Week of %s.md", mondayDate.Format("2006-01-02"))
	filePath := filepath.Join(destDir, filename)

	existingContent := ""
	if _, err := os.Stat(filePath); err == nil {
		data, _ := os.ReadFile(filePath)
		existingContent = string(data)
	}

	missingTemplate := "\n# Weekly Summary\n\n{replacement}\n\n{body}"
	updatedContent := SafeUpdateMarkdown(existingContent, newFrontmatter, newSummary, "%% START_WEEKLY_SUMMARY %%", "%% END_WEEKLY_SUMMARY %%", missingTemplate)

	err := os.WriteFile(filePath, []byte(updatedContent), 0644)
	return filePath, err
}

func hasSource(sources []string, target string) bool {
	for _, s := range sources {
		if s == target {
			return true
		}
	}
	return false
}

// WriteRaceNotes creates/updates race notes and the master race index summary
func WriteRaceNotes(vaultPath, folder string, activities []*models.MergedActivity, unit string) ([]string, error) {
	var races []*models.MergedActivity
	for _, a := range activities {
		if a.IsRace {
			races = append(races, a)
		}
	}

	sort.Slice(races, func(i, j int) bool {
		return races[i].StartTime.Before(races[j].StartTime)
	})

	racesDir := filepath.Join(vaultPath, folder, "Races")
	if err := os.MkdirAll(racesDir, 0755); err != nil {
		return nil, err
	}

	var writtenPaths []string

	// Write individual race notes
	for _, a := range races {
		filename := fmt.Sprintf("%s - %s.md", a.StartTime.Format("2006-01-02"), CleanFilename(a.Title))
		filePath := filepath.Join(racesDir, filename)

		emoji := SportEmojis[a.Sport]
		if emoji == "" {
			emoji = SportEmojis["Other"]
		}
		distStr := FormatDistance(a.Sport, a.DistanceMeters, unit)
		timeStr := FormatDurationCompact(a.DurationSeconds)
		paceStr := FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, unit)

		hrStr := "-"
		if a.AvgHR != nil {
			maxHRVal := ""
			if a.MaxHR != nil {
				maxHRVal = fmt.Sprintf("/%d", int(*a.MaxHR))
			}
			hrStr = fmt.Sprintf("%d%s", int(*a.AvgHR), maxHRVal)
		}

		elevStr := FormatElevation(a.ElevationGainMeters, unit)

		locStr := "-"
		if a.Latitude != nil && a.Longitude != nil {
			mapURL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%.6f,%.6f", *a.Latitude, *a.Longitude)
			locName := a.LocationName
			if locName == "" {
				locName = "Map"
			}
			locStr = fmt.Sprintf("📍 [%s](%s)", locName, mapURL)
		}

		var srcLinks []string
		for _, s := range a.Sources {
			if s == "garmin" && a.GarminID != nil {
				srcLinks = append(srcLinks, fmt.Sprintf("[Garmin](https://connect.garmin.com/modern/activity/%d)", *a.GarminID))
			} else if s == "strava" && a.StravaID != nil {
				srcLinks = append(srcLinks, fmt.Sprintf("[Strava](https://www.strava.com/activities/%d)", *a.StravaID))
			} else {
				srcLinks = append(srcLinks, strings.Title(s))
			}
		}
		srcStr := strings.Join(srcLinks, ", ")

		mondayISO := GetMondayOfWeek(a.StartTime).Format("2006-01-02")
		isTrail := a.Sport == "Run" && a.ElevationGainMeters >= 609.6 && hasSource(a.Sources, "strava")

		summaryLines := []string{
			"### Race Stats",
			fmt.Sprintf("- **Sport**: %s %s", emoji, a.Sport),
			fmt.Sprintf("- **Distance**: %s", distStr),
			fmt.Sprintf("- **Duration**: %s", timeStr),
			fmt.Sprintf("- **Pace/Speed**: %s", paceStr),
			fmt.Sprintf("- **Elevation**: %s", elevStr),
			fmt.Sprintf("- **Heart Rate**: %s", hrStr),
			fmt.Sprintf("- **Location**: %s", locStr),
			fmt.Sprintf("- **Source**: %s", srcStr),
			fmt.Sprintf("- **Weekly Log**: [[../Week of %s|Week of %s]]", mondayISO, mondayISO),
		}

		if isTrail {
			summaryLines = append(summaryLines, fmt.Sprintf("- **Trail Run Note**: [[../TrailRuns/%s - %s|Trail Run Note]]", a.StartTime.Format("2006-01-02"), CleanFilename(a.Title)))
		}

		if strings.TrimSpace(a.Description) != "" {
			summaryLines = append(summaryLines, "", "### Description", strings.TrimSpace(a.Description))
		}

		newSummary := strings.Join(summaryLines, "\n")

		mult := 0.001
		if unit == "miles" {
			mult = 0.000621371
		}
		distDisplay := math.Round(a.DistanceMeters*mult*100) / 100

		frontmatter := map[string]interface{}{
			"type":             "race-activity",
			"date":             a.StartTime.Format("2006-01-02"),
			"sport":            a.Sport,
			"title":            a.Title,
			"distance":         distDisplay,
			"distance_unit":    unit,
			"duration_seconds": int(a.DurationSeconds),
			"location":         a.LocationName,
			"sources":          a.Sources,
		}

		existingContent := ""
		if _, err := os.Stat(filePath); err == nil {
			data, _ := os.ReadFile(filePath)
			existingContent = string(data)
		}

		missingTemplate := fmt.Sprintf("\n# %s\n\n{replacement}\n\n## Personal Notes\n*Write your personal race report or notes here...*\n\n{body}", a.Title)
		updatedContent := SafeUpdateMarkdown(existingContent, frontmatter, newSummary, "%% START_RACE_SUMMARY %%", "%% END_RACE_SUMMARY %%", missingTemplate)

		if err := os.WriteFile(filePath, []byte(updatedContent), 0644); err == nil {
			writtenPaths = append(writtenPaths, filePath)
		}
	}

	// Write summary Races file
	summaryPath := filepath.Join(racesDir, "summary.md")
	tableHeaders := []string{"Date", "Sport", "Race", "Distance", "Time", "Pace", "Location", "Source"}
	var tableRows []string
	tableRows = append(tableRows, "| "+strings.Join(tableHeaders, " | ")+" |")
	tableRows = append(tableRows, "| "+strings.Join([]string{"---", "---", "---", "---", "---", "---", "---", "---"}, " | ")+" |")

	if len(races) == 0 {
		tableRows = append(tableRows, "| - | - | No races recorded yet. | - | - | - | - | - |")
	} else {
		for _, a := range races {
			dateStr := a.StartTime.Format("Mon, Jan 02, 2006")
			emoji := SportEmojis[a.Sport]
			if emoji == "" {
				emoji = SportEmojis["Other"]
			}
			sportStr := fmt.Sprintf("%s %s", emoji, a.Sport)
			distStr := FormatDistance(a.Sport, a.DistanceMeters, unit)
			timeStr := FormatDurationCompact(a.DurationSeconds)
			paceStr := FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, unit)

			locStr := "-"
			if a.Latitude != nil && a.Longitude != nil {
				mapURL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%.6f,%.6f", *a.Latitude, *a.Longitude)
				locName := a.LocationName
				if locName == "" {
					locName = "Map"
				}
				locStr = fmt.Sprintf("📍 [%s](%s)", locName, mapURL)
			}

			var srcLinks []string
			for _, s := range a.Sources {
				if s == "garmin" && a.GarminID != nil {
					srcLinks = append(srcLinks, fmt.Sprintf("[Garmin](https://connect.garmin.com/modern/activity/%d)", *a.GarminID))
				} else if s == "strava" && a.StravaID != nil {
					srcLinks = append(srcLinks, fmt.Sprintf("[Strava](https://www.strava.com/activities/%d)", *a.StravaID))
				} else {
					srcLinks = append(srcLinks, strings.Title(s))
				}
			}
			srcStr := strings.Join(srcLinks, ", ")

			raceFileLinkName := fmt.Sprintf("%s - %s", a.StartTime.Format("2006-01-02"), CleanFilename(a.Title))
			raceLink := fmt.Sprintf("[[%s|%s]]", raceFileLinkName, a.Title)

			isTrail := a.Sport == "Run" && a.ElevationGainMeters >= 609.6 && hasSource(a.Sources, "strava")
			if isTrail {
				raceLink += fmt.Sprintf(" (also [[../TrailRuns/%s|Trail Run]])", raceFileLinkName)
			}

			row := []string{dateStr, sportStr, raceLink, distStr, timeStr, paceStr, locStr, srcStr}
			tableRows = append(tableRows, "| "+strings.Join(row, " | ")+" |")
		}
	}

	tableContent := strings.Join(tableRows, "\n")
	summaryFrontmatter := map[string]interface{}{
		"type":        "race-summary",
		"total_races": len(races),
	}

	existingSummaryContent := ""
	if _, err := os.Stat(summaryPath); err == nil {
		data, _ := os.ReadFile(summaryPath)
		existingSummaryContent = string(data)
	}

	missingSummaryTemplate := "\n# Races Index\n\n{replacement}\n\n{body}"
	updatedSummary := SafeUpdateMarkdown(existingSummaryContent, summaryFrontmatter, tableContent, "%% START_RACES_LIST %%", "%% END_RACES_LIST %%", missingSummaryTemplate)

	if err := os.WriteFile(summaryPath, []byte(updatedSummary), 0644); err == nil {
		writtenPaths = append(writtenPaths, summaryPath)
	}

	return writtenPaths, nil
}

// WriteTrailRunNotes creates/updates trail run reports and index summary
func WriteTrailRunNotes(vaultPath, folder string, activities []*models.MergedActivity, unit string) ([]string, error) {
	var trailRuns []*models.MergedActivity
	for _, a := range activities {
		if a.Sport == "Run" && a.ElevationGainMeters >= 609.6 && hasSource(a.Sources, "strava") {
			trailRuns = append(trailRuns, a)
		}
	}

	sort.Slice(trailRuns, func(i, j int) bool {
		return trailRuns[i].StartTime.Before(trailRuns[j].StartTime)
	})

	trailRunsDir := filepath.Join(vaultPath, folder, "TrailRuns")
	if err := os.MkdirAll(trailRunsDir, 0755); err != nil {
		return nil, err
	}

	var writtenPaths []string

	for _, a := range trailRuns {
		filename := fmt.Sprintf("%s - %s.md", a.StartTime.Format("2006-01-02"), CleanFilename(a.Title))
		filePath := filepath.Join(trailRunsDir, filename)

		distStr := FormatDistance(a.Sport, a.DistanceMeters, unit)
		timeStr := FormatDurationCompact(a.DurationSeconds)
		paceStr := FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, unit)

		hrStr := "-"
		if a.AvgHR != nil {
			maxHRVal := ""
			if a.MaxHR != nil {
				maxHRVal = fmt.Sprintf("/%d", int(*a.MaxHR))
			}
			hrStr = fmt.Sprintf("%d%s", int(*a.AvgHR), maxHRVal)
		}

		elevStr := FormatElevation(a.ElevationGainMeters, unit)

		locStr := "-"
		if a.Latitude != nil && a.Longitude != nil {
			mapURL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%.6f,%.6f", *a.Latitude, *a.Longitude)
			locName := a.LocationName
			if locName == "" {
				locName = "Map"
			}
			locStr = fmt.Sprintf("📍 [%s](%s)", locName, mapURL)
		}

		var srcLinks []string
		for _, s := range a.Sources {
			if s == "garmin" && a.GarminID != nil {
				srcLinks = append(srcLinks, fmt.Sprintf("[Garmin](https://connect.garmin.com/modern/activity/%d)", *a.GarminID))
			} else if s == "strava" && a.StravaID != nil {
				srcLinks = append(srcLinks, fmt.Sprintf("[Strava](https://www.strava.com/activities/%d)", *a.StravaID))
			} else {
				srcLinks = append(srcLinks, strings.Title(s))
			}
		}
		srcStr := strings.Join(srcLinks, ", ")

		mondayISO := GetMondayOfWeek(a.StartTime).Format("2006-01-02")
		isRace := a.IsRace

		summaryLines := []string{
			"### Trail Run Stats",
			fmt.Sprintf("- **Distance**: %s", distStr),
			fmt.Sprintf("- **Duration**: %s", timeStr),
			fmt.Sprintf("- **Pace/Speed**: %s", paceStr),
			fmt.Sprintf("- **Elevation**: %s", elevStr),
			fmt.Sprintf("- **Heart Rate**: %s", hrStr),
			fmt.Sprintf("- **Location**: %s", locStr),
			fmt.Sprintf("- **Source**: %s", srcStr),
			fmt.Sprintf("- **Weekly Log**: [[../Week of %s|Week of %s]]", mondayISO, mondayISO),
		}

		if isRace {
			summaryLines = append(summaryLines, fmt.Sprintf("- **Race Note**: [[../Races/%s - %s|Race Report]]", a.StartTime.Format("2006-01-02"), CleanFilename(a.Title)))
		}

		if strings.TrimSpace(a.Description) != "" {
			summaryLines = append(summaryLines, "", "### Description", strings.TrimSpace(a.Description))
		}

		newSummary := strings.Join(summaryLines, "\n")

		mult := 0.001
		if unit == "miles" {
			mult = 0.000621371
		}
		distDisplay := math.Round(a.DistanceMeters*mult*100) / 100

		frontmatter := map[string]interface{}{
			"type":             "trail-run-activity",
			"date":             a.StartTime.Format("2006-01-02"),
			"title":            a.Title,
			"distance":         distDisplay,
			"distance_unit":    unit,
			"elevation":        FormatElevation(a.ElevationGainMeters, unit),
			"duration_seconds": int(a.DurationSeconds),
			"location":         a.LocationName,
			"sources":          a.Sources,
		}

		existingContent := ""
		if _, err := os.Stat(filePath); err == nil {
			data, _ := os.ReadFile(filePath)
			existingContent = string(data)
		}

		missingTemplate := fmt.Sprintf("\n# %s\n\n{replacement}\n\n## Trail Notes\n*Write your notes regarding the trail conditions, gear used, or climbs here...*\n\n{body}", a.Title)
		updatedContent := SafeUpdateMarkdown(existingContent, frontmatter, newSummary, "%% START_TRAIL_RUN_SUMMARY %%", "%% END_TRAIL_RUN_SUMMARY %%", missingTemplate)

		if err := os.WriteFile(filePath, []byte(updatedContent), 0644); err == nil {
			writtenPaths = append(writtenPaths, filePath)
		}
	}

	// Write summary file
	summaryPath := filepath.Join(trailRunsDir, "summary.md")
	tableHeaders := []string{"Date", "Trail Run", "Distance", "Time", "Pace", "Elevation", "Location", "Source"}
	var tableRows []string
	tableRows = append(tableRows, "| "+strings.Join(tableHeaders, " | ")+" |")
	tableRows = append(tableRows, "| "+strings.Join([]string{"---", "---", "---", "---", "---", "---", "---", "---"}, " | ")+" |")

	if len(trailRuns) == 0 {
		tableRows = append(tableRows, "| - | No trail runs recorded yet. | - | - | - | - | - | - |")
	} else {
		for _, a := range trailRuns {
			dateStr := a.StartTime.Format("Mon, Jan 02, 2006")
			distStr := FormatDistance(a.Sport, a.DistanceMeters, unit)
			timeStr := FormatDurationCompact(a.DurationSeconds)
			paceStr := FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, unit)
			elevStr := FormatElevation(a.ElevationGainMeters, unit)

			locStr := "-"
			if a.Latitude != nil && a.Longitude != nil {
				mapURL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%.6f,%.6f", *a.Latitude, *a.Longitude)
				locName := a.LocationName
				if locName == "" {
					locName = "Map"
				}
				locStr = fmt.Sprintf("📍 [%s](%s)", locName, mapURL)
			}

			var srcLinks []string
			for _, s := range a.Sources {
				if s == "garmin" && a.GarminID != nil {
					srcLinks = append(srcLinks, fmt.Sprintf("[Garmin](https://connect.garmin.com/modern/activity/%d)", *a.GarminID))
				} else if s == "strava" && a.StravaID != nil {
					srcLinks = append(srcLinks, fmt.Sprintf("[Strava](https://www.strava.com/activities/%d)", *a.StravaID))
				} else {
					srcLinks = append(srcLinks, strings.Title(s))
				}
			}
			srcStr := strings.Join(srcLinks, ", ")

			trailFileLinkName := fmt.Sprintf("%s - %s", a.StartTime.Format("2006-01-02"), CleanFilename(a.Title))
			trailLink := fmt.Sprintf("[[%s|%s]]", trailFileLinkName, a.Title)
			if a.IsRace {
				trailLink += fmt.Sprintf(" (also [[../Races/%s|Race]])", trailFileLinkName)
			}

			row := []string{dateStr, trailLink, distStr, timeStr, paceStr, elevStr, locStr, srcStr}
			tableRows = append(tableRows, "| "+strings.Join(row, " | ")+" |")
		}
	}

	tableContent := strings.Join(tableRows, "\n")
	summaryFrontmatter := map[string]interface{}{
		"type":             "trail-run-summary",
		"total_trail_runs": len(trailRuns),
	}

	existingSummaryContent := ""
	if _, err := os.Stat(summaryPath); err == nil {
		data, _ := os.ReadFile(summaryPath)
		existingSummaryContent = string(data)
	}

	missingSummaryTemplate := "\n# Trail Runs Index\n\n{replacement}\n\n{body}"
	updatedSummary := SafeUpdateMarkdown(existingSummaryContent, summaryFrontmatter, tableContent, "%% START_TRAIL_RUNS_LIST %%", "%% END_TRAIL_RUNS_LIST %%", missingSummaryTemplate)

	if err := os.WriteFile(summaryPath, []byte(updatedSummary), 0644); err == nil {
		writtenPaths = append(writtenPaths, summaryPath)
	}

	return writtenPaths, nil
}

// WriteWeeklyWeatherNote fetches a 7-day forecast and outputs a parsed Obsidian weather schedule
func WriteWeeklyWeatherNote(vaultPath, folder string, configMap map[string]interface{}, unit string) (string, error) {
	// Parse coords from configuration map
	lat, _ := configMap["weather_lat"].(float64)
	lon, _ := configMap["weather_lon"].(float64)
	locName, _ := configMap["weather_location"].(string)

	if lat == 0.0 && lon == 0.0 {
		res, err := client.GeolocateByIP()
		if err == nil {
			lat = res["lat"].(float64)
			lon = res["lon"].(float64)
			locName = res["name"].(string)
		} else {
			// fallback
			lat = 42.3709
			lon = -71.1828
			locName = "Watertown, Massachusetts, United States"
		}
	}

	today := time.Now()
	startDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 0, 6)

	weatherData, err := client.FetchWeather(lat, lon, startDate, endDate, unit)
	if err != nil {
		return "", fmt.Errorf("weather forecast fetch error: %w", err)
	}

	daily, _ := weatherData["daily"].(map[string]interface{})
	dailyTimes, _ := daily["time"].([]interface{})
	weatherCodes, _ := daily["weather_code"].([]interface{})
	tempsMax, _ := daily["temperature_2m_max"].([]interface{})
	tempsMin, _ := daily["temperature_2m_min"].([]interface{})
	sunrises, _ := daily["sunrise"].([]interface{})
	sunsets, _ := daily["sunset"].([]interface{})
	windsMax, _ := daily["wind_speed_10m_max"].([]interface{})

	// Hourly variables
	hourly, _ := weatherData["hourly"].(map[string]interface{})
	hourlyTimes, _ := hourly["time"].([]interface{})
	hourlyTemps, _ := hourly["temperature_2m"].([]interface{})
	hourlyApparentTemps, _ := hourly["apparent_temperature"].([]interface{})
	hourlyHumidities, _ := hourly["relative_humidity_2m"].([]interface{})
	hourlyWindSpeeds, _ := hourly["wind_speed_10m"].([]interface{})
	hourlyWindDirs, _ := hourly["wind_direction_10m"].([]interface{})
	hourlyPrecipProbs, _ := hourly["precipitation_probability"].([]interface{})
	hourlyWeatherCodes, _ := hourly["weather_code"].([]interface{})

	type hourlyHour struct {
		hour         int
		timeVal      time.Time
		temp         float64
		apparentTemp float64
		humidity     float64
		windSpeed    float64
		windDir      float64
		precipProb   float64
		weatherCode  int
	}

	hourlyByDate := make(map[string][]hourlyHour)
	for i := 0; i < len(hourlyTimes); i++ {
		tStr, _ := hourlyTimes[i].(string)
		tVal, err := time.Parse("2006-01-02T15:04", tStr)
		if err != nil {
			// fallback
			tVal, _ = time.Parse(time.RFC3339, tStr)
		}

		dStr := tVal.Format("2006-01-02")
		var hh hourlyHour
		hh.hour = tVal.Hour()
		hh.timeVal = tVal

		if i < len(hourlyTemps) {
			hh.temp, _ = hourlyTemps[i].(float64)
		}
		if i < len(hourlyApparentTemps) {
			hh.apparentTemp, _ = hourlyApparentTemps[i].(float64)
		}
		if i < len(hourlyHumidities) {
			hh.humidity, _ = hourlyHumidities[i].(float64)
		}
		if i < len(hourlyWindSpeeds) {
			hh.windSpeed, _ = hourlyWindSpeeds[i].(float64)
		}
		if i < len(hourlyWindDirs) {
			hh.windDir, _ = hourlyWindDirs[i].(float64)
		}
		if i < len(hourlyPrecipProbs) {
			hh.precipProb, _ = hourlyPrecipProbs[i].(float64)
		}
		if i < len(hourlyWeatherCodes) {
			v, _ := hourlyWeatherCodes[i].(float64)
			hh.weatherCode = int(v)
		}

		hourlyByDate[dStr] = append(hourlyByDate[dStr], hh)
	}

	frontmatter := map[string]interface{}{
		"type":          "weather-summary",
		"location":      locName,
		"latitude":      math.Round(lat*10000) / 10000,
		"longitude":     math.Round(lon*10000) / 10000,
		"last_updated":  time.Now().Format("2006-01-02 15:04:05"),
		"distance_unit": unit,
	}

	tableHeaders := []string{"Date", "Condition", "Temp Range", "Max Wind", "Avg Hum", "Sun Times", "Best Run Window"}
	var tableRows []string
	tableRows = append(tableRows, "| "+strings.Join(tableHeaders, " | ")+" |")
	tableRows = append(tableRows, "| "+strings.Join([]string{"---", "---", "---", "---", "---", "---", "---"}, " | ")+" |")

	var detailsSections []string

	for i := 0; i < len(dailyTimes); i++ {
		dStr, _ := dailyTimes[i].(string)
		dVal, err := time.Parse("2006-01-02", dStr)
		dateDisplay := dStr
		if err == nil {
			dateDisplay = dVal.Format("Mon, Jan 02")
		}

		wCodeFloat, _ := weatherCodes[i].(float64)
		wCode := int(wCodeFloat)
		cond := client.WMOCodes[wCode]
		condStr := "❓ Unknown"
		if cond[0] != "" {
			condStr = fmt.Sprintf("%s %s", cond[1], cond[0])
		}

		tMax, _ := tempsMax[i].(float64)
		tMin, _ := tempsMin[i].(float64)
		tempSuffix := "°F"
		if unit == "km" {
			tempSuffix = "°C"
		}
		tempStr := fmt.Sprintf("%d%s - %d%s", int(math.Round(tMin)), tempSuffix, int(math.Round(tMax)), tempSuffix)

		wMax, _ := windsMax[i].(float64)
		windSuffix := "mph"
		if unit == "km" {
			windSuffix = "kmh"
		}
		windStr := fmt.Sprintf("%d %s", int(math.Round(wMax)), windSuffix)

		// Sunrise/Sunset formatting
		sunriseStr := "-"
		sunsetStr := "-"
		if i < len(sunrises) && i < len(sunsets) {
			srStr, _ := sunrises[i].(string)
			ssStr, _ := sunsets[i].(string)
			srTime, err1 := time.Parse("2006-01-02T15:04", srStr)
			ssTime, err2 := time.Parse("2006-01-02T15:04", ssStr)
			if err1 == nil && err2 == nil {
				sunriseStr = strings.TrimPrefix(srTime.Format("03:04PM"), "0")
				sunsetStr = strings.TrimPrefix(ssTime.Format("03:04PM"), "0")
			}
		}
		sunTimesStr := fmt.Sprintf("%s - %s", sunriseStr, sunsetStr)

		// Filter for active running hours (5:00 AM to 7:00 PM)
		var runHours []hourlyHour
		for _, hh := range hourlyByDate[dStr] {
			if hh.hour >= 5 && hh.hour <= 19 {
				runHours = append(runHours, hh)
			}
		}

		avgHumVal := 0.0
		bestHourStr := "No window"
		bestRPEScore := math.MaxFloat64

		if len(runHours) > 0 {
			humSum := 0.0
			for _, hh := range runHours {
				humSum += hh.humidity
			}
			avgHumVal = humSum / float64(len(runHours))

			for _, hh := range runHours {
				tVal := hh.temp
				wVal := hh.windSpeed
				rhVal := hh.humidity

				tF := tVal
				wMPH := wVal
				if unit == "km" {
					tF = tVal*9.0/5.0 + 32.0
					wMPH = wVal * 0.621371
				}

				rpeScore, rpeLabel, _ := client.CalculatePerceivedExertion(tF, rhVal, wMPH)
				if rpeScore < bestRPEScore {
					bestRPEScore = rpeScore
					bestHourStr = fmt.Sprintf("%s (%s)", strings.TrimPrefix(hh.timeVal.Format("03:04 PM"), "0"), rpeLabel)
				}
			}
		}

		tableRows = append(tableRows, fmt.Sprintf("| %s | %s | %s | %s | %d%% | %s | %s |", dateDisplay, condStr, tempStr, windStr, int(math.Round(avgHumVal)), sunTimesStr, bestHourStr))

		// Build detailed hourly breakdown table
		dayHourlyLines := []string{
			fmt.Sprintf("### %s", dateDisplay),
			"",
			"| Hour | Condition | Temp (Feels) | Humidity | Wind | Precip | Perceived Exertion (RPE) |",
			"| --- | --- | --- | --- | --- | --- | --- |",
		}

		for _, hh := range hourlyByDate[dStr] {
			if hh.hour < 5 || hh.hour > 19 {
				continue
			}

			hrDisp := strings.TrimPrefix(hh.timeVal.Format("03:04 PM"), "0")
			hCond := client.WMOCodes[hh.weatherCode]
			hCondStr := "❓ Unknown"
			if hCond[0] != "" {
				hCondStr = fmt.Sprintf("%s %s", hCond[1], hCond[0])
			}

			hTempStr := fmt.Sprintf("%d%s (%d%s)", int(math.Round(hh.temp)), tempSuffix, int(math.Round(hh.apparentTemp)), tempSuffix)
			hHumStr := fmt.Sprintf("%d%%", int(math.Round(hh.humidity)))
			hWindStr := fmt.Sprintf("%d %s %s", int(math.Round(hh.windSpeed)), windSuffix, client.DegreesToCardinal(hh.windDir))
			hPrecipStr := fmt.Sprintf("%d%%", int(math.Round(hh.precipProb)))

			tVal := hh.temp
			wVal := hh.windSpeed
			rhVal := hh.humidity
			tF := tVal
			wMPH := wVal
			if unit == "km" {
				tF = tVal*9.0/5.0 + 32.0
				wMPH = wVal * 0.621371
			}
			rpeScore, rpeLabel, _ := client.CalculatePerceivedExertion(tF, rhVal, wMPH)
			hRPEStr := fmt.Sprintf("**%s** (Score: %.1f)", rpeLabel, rpeScore)

			dayHourlyLines = append(dayHourlyLines, fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |", hrDisp, hCondStr, hTempStr, hHumStr, hWindStr, hPrecipStr, hRPEStr))
		}
		dayHourlyLines = append(dayHourlyLines, "")
		detailsSections = append(detailsSections, strings.Join(dayHourlyLines, "\n"))
	}

	newTableSummary := strings.Join(tableRows, "\n")
	newHourlySummary := strings.Join(detailsSections, "\n")

	destDir := filepath.Join(vaultPath, folder)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	filePath := filepath.Join(destDir, "WeeklyWeather.md")

	existingContent := ""
	if _, err := os.Stat(filePath); err == nil {
		data, _ := os.ReadFile(filePath)
		existingContent = string(data)
	}

	if existingContent == "" {
		existingContent = fmt.Sprintf(
			"---\ntype: weather-summary\n---\n\n"+
				"# Weekly Weather & Running Exertion\n\n"+
				"**Location**: %s\n"+
				"**Last Updated**: %s\n\n"+
				"%% START_WEATHER_OUTLOOK %%\n"+
				"%% END_WEATHER_OUTLOOK %%\n\n"+
				"## 📋 Exertion Index Legend\n"+
				"- **Ideal** (0-1.5): Perfect running conditions. RPE is baseline.\n"+
				"- **Moderate** (1.6-3.0): Slightly increased effort.\n"+
				"- **Hard** (3.1-5.0): Noticeable exertion increase.\n"+
				"- **Very Hard** (5.1-7.0): High cardiovascular strain.\n"+
				"- **Extreme** (>7.0): Severe strain.\n\n"+
				"## 📅 Detailed Hourly Outlooks (5:00 AM - 7:00 PM)\n\n"+
				"%% START_HOURLY_OUTLOOK %%\n"+
				"%% END_HOURLY_OUTLOOK %%\n\n"+
				"## 📝 Training & Running Notes\n"+
				"*Write your training notes or plans for the week here...*\n",
			locName, frontmatter["last_updated"],
		)
	}

	content1 := SafeUpdateMarkdown(
		existingContent,
		frontmatter,
		newTableSummary,
		"%% START_WEATHER_OUTLOOK %%",
		"%% END_WEATHER_OUTLOOK %%",
		"\n# Weather Outlook\n\n{replacement}\n\n{body}",
	)

	finalContent := SafeUpdateMarkdown(
		content1,
		nil,
		newHourlySummary,
		"%% START_HOURLY_OUTLOOK %%",
		"%% END_HOURLY_OUTLOOK %%",
		"\n# Hourly Outlooks\n\n{replacement}\n\n{body}",
	)

	err = os.WriteFile(filePath, []byte(finalContent), 0644)
	return filePath, err
}
