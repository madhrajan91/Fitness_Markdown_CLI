package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"running_cli/pkg/models"
	"running_cli/pkg/storage"
)

// The plugin routes let another app show running-cli's views read-only: first
// the shared outliner (its docs/specs/26-plugins.md). A view is plain Markdown,
// so the app needs to know nothing about running, and copying a view into a
// note is an ordinary paste.

// pluginProtocol is bumped whenever these routes change incompatibly.
const pluginProtocol = 1

const pluginWeatherDays = 7

// pluginManifest describes the plugin. Start is the command that starts it: the
// app keeps it and shows it when this server can't be reached.
type pluginManifest struct {
	Protocol int              `json:"protocol"`
	Name     string           `json:"name"`
	Start    string           `json:"start,omitempty"`
	Views    []pluginViewInfo `json:"views"`
}

type pluginViewInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Pager bool   `json:"pager"`
}

// pluginView is one rendered view. Prev and Next are cursors for ?at=, left out
// when there's nothing that way.
type pluginView struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
	Prev     string `json:"prev,omitempty"`
	Next     string `json:"next,omitempty"`
}

// pluginRoutes holds what the views need, so tests can pass fakes.
type pluginRoutes struct {
	Unit       string
	Start      string
	Now        func() time.Time
	Activities func(start, end time.Time) ([]*models.MergedActivity, error)
	Weather    func(today time.Time) (runningWeather, string, error)
}

// handler serves GET /plugin/manifest and GET /plugin/views/{id}. Everything is
// read-only: nothing here starts a sync or changes data.
func (p pluginRoutes) handler() http.Handler {
	views := map[string]func(*http.Request) (pluginView, int, error){
		"week":    p.weekView,
		"weather": p.weatherView,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enableCors(w, r)
		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusOK)
			return
		case http.MethodGet:
		default:
			http.Error(w, "plugin routes are read-only", http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path == "/plugin/manifest" {
			writePluginJSON(w, pluginManifest{
				Protocol: pluginProtocol,
				Name:     "Running",
				Start:    p.Start,
				Views: []pluginViewInfo{
					{ID: "week", Title: "Week", Pager: true},
					{ID: "weather", Title: "Weather"},
				},
			})
			return
		}

		view, ok := views[strings.TrimPrefix(r.URL.Path, "/plugin/views/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		rendered, status, err := view(r)
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		writePluginJSON(w, rendered)
	})
}

func writePluginJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func (p pluginRoutes) weekView(r *http.Request) (pluginView, int, error) {
	now := p.Now()
	cursor := r.URL.Query().Get("at")
	if cursor == "" {
		cursor = now.Format("2006-01-02")
	}
	start, end, err := parseWeekParam(cursor)
	if err != nil {
		return pluginView{}, http.StatusBadRequest, err
	}
	activities, err := p.Activities(start, end)
	if err != nil {
		return pluginView{}, http.StatusInternalServerError, err
	}

	view := pluginView{
		Title:    "Week of " + start.Format("2 Jan 2006"),
		Markdown: renderWeekMarkdown(activities, p.Unit),
		Prev:     start.AddDate(0, 0, -7).Format("2006-01-02"),
	}
	if next := start.AddDate(0, 0, 7); !next.After(now) {
		view.Next = next.Format("2006-01-02")
	}
	return view, http.StatusOK, nil
}

func (p pluginRoutes) weatherView(*http.Request) (pluginView, int, error) {
	weather, location, err := p.Weather(p.Now())
	if err != nil {
		return pluginView{}, http.StatusBadGateway, fmt.Errorf("fetching the forecast: %w", err)
	}
	title := "Weather"
	if location != "" {
		title = "Weather for " + location
	}
	return pluginView{Title: title, Markdown: renderWeatherMarkdown(weather)}, http.StatusOK, nil
}

// renderWeekMarkdown is totals per sport, then one row per activity from Monday
// to Sunday. Coordinates, descriptions and raw provider data are left out on
// purpose: a copy lands in notes other people can read.
func renderWeekMarkdown(activities []*models.MergedActivity, unit string) string {
	if len(activities) == 0 {
		return "No activities this week."
	}
	sorted := append([]*models.MergedActivity(nil), activities...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].StartTime.Before(sorted[j].StartTime) })

	type total struct {
		distance, duration float64
		count              int
	}
	totals := map[string]*total{}
	var sports []string
	for _, a := range sorted {
		t, ok := totals[a.Sport]
		if !ok {
			t = &total{}
			totals[a.Sport] = t
			sports = append(sports, a.Sport)
		}
		t.distance += a.DistanceMeters
		t.duration += a.DurationSeconds
		t.count++
	}
	sort.Strings(sports)
	sort.SliceStable(sports, func(i, j int) bool { return totals[sports[i]].count > totals[sports[j]].count })

	var b strings.Builder
	for _, sport := range sports {
		t := totals[sport]
		label := "activities"
		if t.count == 1 {
			label = "activity"
		}
		fmt.Fprintf(&b, "- **%s:** %s · %s · %d %s\n", mdText(sport), storage.FormatDistance(sport, t.distance, unit), storage.FormatDuration(t.duration), t.count, label)
	}

	b.WriteString("\n| Day | Activity | Sport | Distance | Time | Pace | Avg HR | Location |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, a := range sorted {
		title := mdText(a.Title)
		if links := a.SourceLinks(); len(links) > 0 {
			title = fmt.Sprintf("[%s](%s)", title, links[0].URL)
		}
		hr := "-"
		if a.AvgHR != nil {
			hr = fmt.Sprintf("%.0f", *a.AvgHR)
		}
		location := "-"
		if a.LocationName != "" {
			location = mdText(a.LocationName)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			a.StartTime.Format("Mon 2 Jan"), title, mdText(a.Sport),
			storage.FormatDistance(a.Sport, a.DistanceMeters, unit),
			storage.FormatDurationCompact(a.DurationSeconds),
			storage.FormatPaceOrSpeed(a.Sport, a.DistanceMeters, a.DurationSeconds, unit),
			hr, location)
	}
	return b.String()
}

// renderWeatherMarkdown is one row per forecast day.
func renderWeatherMarkdown(weather runningWeather) string {
	var b strings.Builder
	b.WriteString("| Day | Conditions | Low–High | Wind | Sunrise / Sunset | Best hour | RPE |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, d := range weather.Days {
		day := d.Date
		if t, err := time.Parse("2006-01-02", d.Date); err == nil {
			day = t.Format("Mon 2 Jan")
		}
		best, rpe := "-", "-"
		if d.Best != nil {
			best = clockTime(d.Best.Time)
			rpe = fmt.Sprintf("%g %s", d.Best.RPE, mdText(d.Best.RPELabel))
		}
		fmt.Fprintf(&b, "| %s | %s %s | %.0f–%.0f%s | %.0f %s | %s / %s | %s | %s |\n",
			day, d.Emoji, mdText(d.Condition),
			d.TempMin, d.TempMax, weather.TempUnit,
			d.WindMax, weather.WindUnit,
			clockTime(d.Sunrise), clockTime(d.Sunset),
			best, rpe)
	}
	return b.String()
}

// clockTime is the HH:MM of a YYYY-MM-DDTHH:MM timestamp.
func clockTime(ts string) string {
	if _, after, ok := strings.Cut(ts, "T"); ok {
		return after
	}
	return ts
}

var mdEscaper = strings.NewReplacer(
	`\`, `\\`, `|`, `\|`, `[`, `\[`, `]`, `\]`, `*`, `\*`, `_`, `\_`, "`", "\\`", `~`, `\~`, `<`, `\<`, `>`, `\>`,
	"\r\n", " ", "\n", " ", "\r", " ",
)

// mdText makes text from an activity safe inside a Markdown table: a | can't
// add a column, a newline can't end the row, and [[Title]] can't become a link
// to a page in the app showing it.
func mdText(s string) string {
	return mdEscaper.Replace(s)
}
