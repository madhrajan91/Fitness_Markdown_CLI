# running-cli

A Go command-line interface (CLI) tool that fetches your sports activity data from Garmin Connect and Strava, caches and merges them in a local SQLite database, auto-links activities with imported CSV training plans, and synchronizes the results into your Obsidian Markdown vault as beautiful, structured weekly notes, race summaries, trail logs, and weather forecasts.

---

## 🏃‍♂️ Features

- **SQLite Database Backend**: Fast, structured cache stored locally at `~/.running_cli/running_cli.db` (replaces legacy raw JSON files).
- **Auto-Linking & Compliance**: Import custom training plans from a CSV file. The CLI automatically links actual activities with planned workouts based on date (±1 day) and sport type, calculating compliance rates.
- **Garmin + Strava Merger**: Deduplicates activities recorded on both Garmin and Strava, preferring titles and detailed metadata from Strava while preserving Garmin's primary stats.
- **Weather Forecast & Perceived Exertion (RPE)**: Integrates with Open-Meteo to fetch daily forecasts and hourly details. Automatically computes a Running Perceived Exertion (RPE) index estimating how weather affects run difficulty.
- **Upcoming Race Finder**: Queries RunSignup's API to fetch, geocode, and sync local upcoming races within a configurable radius of your location.
- **Obsidian Sync**: Generates and updates:
  - Weekly Summaries (`Week of YYYY-MM-DD.md`)
  - Race Summaries (`Races/` folder)
  - Trail Run logs (`TrailRuns/` folder)
  - Weather Forecast updates (`WeeklyWeather.md`)
  - Future Races list (`FutureRaces.md`)
- **Obsidian In-place Sync (Safe Update)**: All Obsidian formats use safe markdown parser updates to keep logs synced while preserving your custom manual notes and journaling.

---

## 📋 Prerequisites

- **Go 1.25+** (for building the project)
- **Python 3.8+** (with `pip install garminconnect` to run the Garmin authentication helper)
- **Garmin Connect Account**
- **Strava Developer Account**: Create an application at [Strava settings/api](https://www.strava.com/settings/api) to get a `Client ID` and `Client Secret`.

---

## ⚙️ Installation

1. Clone or navigate to the project directory:
   ```bash
   cd /Users/madhavrajan/Documents/AIProjects/running_cli
   ```

2. Install Python dependency for Garmin Connect API access:
   ```bash
   pip install garminconnect
   ```

3. Build the Go binary:
   ```bash
   go build -o running-cli main.go
   ```

---

## 🚀 Usage

### 1. Interactive Setup
Run the setup wizard to configure Obsidian directories, measure units, credentials, and default weather location:
```bash
./running-cli setup
```
- For Garmin, login is verified and session credentials are saved to `~/.running_cli/garmin_session` to enable passwordless sync.
- For Strava, the CLI starts a temporary OAuth redirect server on port 8000 and launches your browser to authorize access.

### 2. Connection Health Check
Check configured credentials, directories, and test connection statuses:
```bash
./running-cli status
```

### 3. Import Historical Caches
If you have local cached JSON files from the legacy Python tool, sync them directly into SQLite and Obsidian in one go:
```bash
./running-cli import-cache
```

### 4. Sync Activities & Weather
Pulls Garmin/Strava runs, merges them, auto-links training plans, queries weather, and writes weekly and future race notes to Obsidian:
- **Default (Last 7 days)**:
  ```bash
  ./running-cli sync
  ```
- **Custom lookback window**:
  ```bash
  ./running-cli sync --days 14
  ```
- **Specific week or date range**:
  ```bash
  ./running-cli sync --start-date 2026-07-20 --end-date 2026-07-26
  ```

### 5. View Activities (Terminal)
Query database activities from the terminal:
- **Weekly activity log**:
  ```bash
  ./running-cli view --week 2026-07-20
  ```
- **Master Races list**:
  ```bash
  ./running-cli races
  ```
- **Master Trail Runs list**:
  ```bash
  ./running-cli trail-runs
  ```
- **Open activity in browser** (opens Garmin Connect/Strava page):
  ```bash
  ./running-cli open --week 2026-07-20
  ```

### 6. Weather Forecast (Terminal)
Display upcoming forecasts with sunrise, sunset, temperatures, wind, and Running Perceived Exertion (RPE):
```bash
./running-cli weather --days 3
```

### 7. Training Plan Compliance
Manage and track training schedules:
- **Import training plan (CSV)**:
  ```bash
  ./running-cli plan import test_5k_plan.csv --name "5K Target Plan" --desc "My 5K plan"
  ```
- **List active plans**:
  ```bash
  ./running-cli plan list
  ```
- **Show compliance & workout completion status**:
  ```bash
  ./running-cli plan status 1
  ```
- **Trigger auto-linking manually**:
  ```bash
  ./running-cli plan link
  ```

---

## 🧪 Running Tests

Execute the Go test suite:
```bash
go test ./...
```
The tests verify config management, activity merging rules, database schema migrations, and workout plan auto-linking.
