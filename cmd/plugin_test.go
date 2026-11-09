package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"running_cli/pkg/models"
)

// go test ./cmd -run TestPluginSamples -update rewrites testdata/plugin. Those
// files are also what the outliner's tests use as their fake plugin's answers.
var updatePluginSamples = flag.Bool("update", false, "rewrite the plugin sample files")

func ptrF(f float64) *float64 { return &f }
func ptrI(i int64) *int64     { return &i }

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

// Made up, in no particular order. Coordinates and a description are set so
// the tests can check they stay out of the views.
func sampleActivities() []*models.MergedActivity {
	return []*models.MergedActivity{
		{Sport: "Run", Title: "Tempo | hill repeats", StartTime: at("2026-11-04 07:10"), DistanceMeters: 10000, DurationSeconds: 2820, AvgHR: ptrF(161),
			LocationName: "Riverside", Latitude: ptrF(12.3456), Longitude: ptrF(-65.4321), StravaID: ptrI(111), Sources: []string{"strava"}, Description: "secret note"},
		{Sport: "Run", Title: "Easy morning", StartTime: at("2026-11-02 06:45"), DistanceMeters: 8047, DurationSeconds: 2700, AvgHR: ptrF(138),
			LocationName: "Riverside", GarminID: ptrI(222), Sources: []string{"garmin"}},
		{Sport: "Ride", Title: "[[Commute]]", StartTime: at("2026-11-03 17:30"), DistanceMeters: 16093, DurationSeconds: 3000},
	}
}

func sampleWeather(t *testing.T) runningWeather {
	t.Helper()
	var forecast map[string]interface{}
	if err := json.Unmarshal([]byte(forecastFixture), &forecast); err != nil {
		t.Fatal(err)
	}
	return buildRunningWeather(forecast, "miles")
}

func testPluginRoutes(t *testing.T) pluginRoutes {
	t.Helper()
	return pluginRoutes{
		Unit: "miles",
		Now:  func() time.Time { return at("2026-11-05 12:00") }, // a Thursday
		Activities: func(start, end time.Time) ([]*models.MergedActivity, error) {
			var in []*models.MergedActivity
			for _, a := range sampleActivities() {
				if !a.StartTime.Before(start) && !a.StartTime.After(end) {
					in = append(in, a)
				}
			}
			return in, nil
		},
		Weather: func(time.Time) (runningWeather, string, error) { return sampleWeather(t), "Springfield", nil },
	}
}

func pluginGet(t *testing.T, routes pluginRoutes, method, path, origin string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	routes.handler().ServeHTTP(rec, req)
	return rec
}

func pluginViewAt(t *testing.T, routes pluginRoutes, path string) pluginView {
	t.Helper()
	rec := pluginGet(t, routes, http.MethodGet, path, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body)
	}
	var v pluginView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// The exact replies, so a change to the protocol or the tables shows up as a diff.
func TestPluginSamples(t *testing.T) {
	cases := []struct{ file, path string }{
		{"manifest.json", "/plugin/manifest"},
		{"week.json", "/plugin/views/week?at=2026-11-02"},
		{"weather.json", "/plugin/views/weather"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			rec := pluginGet(t, testPluginRoutes(t), http.MethodGet, c.path, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q", got)
			}
			path := filepath.Join("testdata", "plugin", c.file)
			if *updatePluginSamples {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, rec.Body.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if rec.Body.String() != string(want) {
				t.Errorf("reply differs from %s:\n got %s\nwant %s", path, rec.Body, want)
			}
		})
	}
}

func TestPluginWeekPaging(t *testing.T) {
	cases := []struct {
		name, path, title, prev, next string
	}{
		{"no cursor is this week, with nothing to page forward to", "/plugin/views/week", "Week of 2 Nov 2026", "2026-10-26", ""},
		{"any day finds its Monday", "/plugin/views/week?at=2026-11-05", "Week of 2 Nov 2026", "2026-10-26", ""},
		{"an earlier week pages both ways", "/plugin/views/week?at=2026-10-26", "Week of 26 Oct 2026", "2026-10-19", "2026-11-02"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := pluginViewAt(t, testPluginRoutes(t), c.path)
			if v.Title != c.title || v.Prev != c.prev || v.Next != c.next {
				t.Errorf("title, prev, next = %q, %q, %q", v.Title, v.Prev, v.Next)
			}
		})
	}
}

func TestPluginWeekContent(t *testing.T) {
	routes := testPluginRoutes(t)
	md := pluginViewAt(t, routes, "/plugin/views/week?at=2026-11-02").Markdown

	t.Run("Monday to Sunday, whatever order the database gives", func(t *testing.T) {
		mon, tue, wed := strings.Index(md, "Easy morning"), strings.Index(md, "Commute"), strings.Index(md, "Tempo")
		if !(mon < tue && tue < wed) || mon < 0 {
			t.Errorf("rows out of order in:\n%s", md)
		}
	})
	t.Run("totals per sport", func(t *testing.T) {
		for _, want := range []string{"**Run:** 11.21 mi · 1h 32m · 2 activities", "**Ride:** 10.00 mi · 50m 00s · 1 activity"} {
			if !strings.Contains(md, want) {
				t.Errorf("no %q in:\n%s", want, md)
			}
		}
	})
	t.Run("titles link to their source; no source, no link", func(t *testing.T) {
		if !strings.Contains(md, "(https://www.strava.com/activities/111)") || !strings.Contains(md, "(https://connect.garmin.com/modern/activity/222)") {
			t.Errorf("source links missing in:\n%s", md)
		}
		if strings.Contains(md, "\\]\\](http") {
			t.Errorf("a title with no source got a link:\n%s", md)
		}
	})
	t.Run("a title can't break the table or link to a page", func(t *testing.T) {
		if !strings.Contains(md, `Tempo \| hill repeats`) || !strings.Contains(md, `\[\[Commute\]\]`) {
			t.Errorf("title not escaped in:\n%s", md)
		}
		bar := regexp.MustCompile(`(^|[^\\])\|`)
		var cells int
		for _, line := range strings.Split(md, "\n") {
			if !strings.HasPrefix(line, "|") {
				continue
			}
			n := len(bar.FindAllString(line, -1))
			if cells == 0 {
				cells = n
			} else if n != cells {
				t.Errorf("row has %d bars, header has %d: %s", n, cells, line)
			}
		}
		if cells == 0 {
			t.Errorf("no table in:\n%s", md)
		}
	})
	t.Run("coordinates and descriptions stay out", func(t *testing.T) {
		for _, secret := range []string{"12.34", "65.43", "secret note"} {
			if strings.Contains(md, secret) {
				t.Errorf("%q appears in:\n%s", secret, md)
			}
		}
	})
	t.Run("an empty week says so and can still page", func(t *testing.T) {
		v := pluginViewAt(t, routes, "/plugin/views/week?at=2026-10-05")
		if v.Markdown != "No activities this week." || v.Prev == "" || v.Next == "" {
			t.Errorf("empty week = %+v", v)
		}
	})
}

func TestPluginWeatherContent(t *testing.T) {
	v := pluginViewAt(t, testPluginRoutes(t), "/plugin/views/weather")
	if v.Title != "Weather for Springfield" || v.Prev != "" || v.Next != "" {
		t.Errorf("title, prev, next = %q, %q, %q", v.Title, v.Prev, v.Next)
	}
	for _, want := range []string{"06:51 / 18:22", "40–82°F", "20 mph", "07:00"} {
		if !strings.Contains(v.Markdown, want) {
			t.Errorf("no %q in:\n%s", want, v.Markdown)
		}
	}
	// The fixture's third day has no hours, so no best hour.
	lines := strings.Split(strings.TrimSpace(v.Markdown), "\n")
	if last := lines[len(lines)-1]; !strings.HasSuffix(last, "| - | - |") {
		t.Errorf("a day with no best hour = %q", last)
	}
}

func TestPluginRequests(t *testing.T) {
	cases := []struct {
		name, method, path string
		want               int
	}{
		{"unknown view", http.MethodGet, "/plugin/views/races", http.StatusNotFound},
		{"unknown route", http.MethodGet, "/plugin/nope", http.StatusNotFound},
		{"bad cursor", http.MethodGet, "/plugin/views/week?at=last-week", http.StatusBadRequest},
		{"plugins are read-only: POST", http.MethodPost, "/plugin/views/week", http.StatusMethodNotAllowed},
		{"plugins are read-only: DELETE", http.MethodDelete, "/plugin/manifest", http.StatusMethodNotAllowed},
		{"the browser's preflight", http.MethodOptions, "/plugin/manifest", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if rec := pluginGet(t, testPluginRoutes(t), c.method, c.path, ""); rec.Code != c.want {
				t.Errorf("status = %d, want %d", rec.Code, c.want)
			}
		})
	}
}

func TestPluginOrigins(t *testing.T) {
	cases := []struct{ origin, want string }{
		{"http://localhost:5420", "http://localhost:5420"},
		{"http://127.0.0.1:8421", "http://127.0.0.1:8421"},
		{"https://example.com", ""},
	}
	for _, c := range cases {
		rec := pluginGet(t, testPluginRoutes(t), http.MethodGet, "/plugin/manifest", c.origin)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != c.want {
			t.Errorf("origin %s: allowed = %q, want %q", c.origin, got, c.want)
		}
	}
}

func TestPluginUpstreamErrors(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*pluginRoutes)
		path   string
		want   int
	}{
		{"the database fails", func(r *pluginRoutes) {
			r.Activities = func(time.Time, time.Time) ([]*models.MergedActivity, error) { return nil, errors.New("locked") }
		}, "/plugin/views/week", http.StatusInternalServerError},
		{"the forecast can't be fetched", func(r *pluginRoutes) {
			r.Weather = func(time.Time) (runningWeather, string, error) { return runningWeather{}, "", errors.New("offline") }
		}, "/plugin/views/weather", http.StatusBadGateway},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			routes := testPluginRoutes(t)
			c.break_(&routes)
			if rec := pluginGet(t, routes, http.MethodGet, c.path, ""); rec.Code != c.want {
				t.Errorf("status = %d, want %d", rec.Code, c.want)
			}
		})
	}
}

func TestMdText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Easy run", "Easy run"},
		{"a|b", `a\|b`},
		{"[[Page]]", `\[\[Page\]\]`},
		{"*bold* _it_ `code` ~x~", "\\*bold\\* \\_it\\_ \\`code\\` \\~x\\~"},
		{"<script>", `\<script\>`},
		{`back\slash`, `back\\slash`},
		{"two\nlines\r\nhere", "two lines here"},
	}
	for _, c := range cases {
		if got := mdText(c.in); got != c.want {
			t.Errorf("mdText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
