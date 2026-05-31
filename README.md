# running-cli

A Python command-line interface (CLI) tool that fetches your sports activity data from Garmin Connect and Strava, caches the raw activities locally, merges and deduplicates them, and summarizes them into a local Markdown directory/vault (which naturally fits as a folder inside an Obsidian vault) as rich weekly notes (`Week of YYYY-MM-DD.md`) and weather forecast summaries (`WeeklyWeather.md`).

## Features
- **All Sports Supported**: Supports Runs, Rides, Swims, Walks, Hikes, and more.
- **Weekly Summarization**: Consolidates activities for a week (Monday to Sunday) into a single Markdown file.
- **In-place Sync (Safe Update)**: Keeps weekly logs updated if you request syncs again, preserving your custom journaling or manual notes written in those files.
- **Robust Deduplication**: Automatically merges Garmin and Strava entries representing the same activity by comparing start times and distance thresholds.
- **Weather Forecast & Perceived Exertion (RPE)**: Integrates with Open-Meteo to fetch hourly weather data and automatically estimates a Running Perceived Exertion (RPE) impact score based on heat/humidity/dew point and wind speed.
- **Custom Units**: Configurable measurement units (default is miles, supports km).
- **Offline Raw Cache**: Retains raw JSON files in `~/.running_cli/cache/` so you can regenerate or reformat weekly entries without hitting external APIs again.

---

## Prerequisites
- **Python 3.8+** (Tested on Python 3.14)
- **Garmin Connect Account** (Password is required only during initial setup; session cookies/tokens are stored securely in `~/.running_cli/garmin_session/` to enable passwordless sync and support Multi-Factor Authentication)
- **Strava Developer Account**: Create an application at [Strava settings/api](https://www.strava.com/settings/api) to get a `Client ID` and `Client Secret`.
- **Open-Meteo API**: Used for weather geocoding and forecasts (no API key required).

---

## Installation

1. Navigate to the project directory:
   ```bash
   cd /Users/madhavrajan/Documents/AIProjects/running_cli
   ```

2. Create and activate a virtual environment:
   ```bash
   python3 -m venv venv
   source venv/bin/activate
   ```

3. Install dependencies and the package:
   ```bash
   pip install -r requirements.txt
   pip install -e .
   ```

---

## Usage

### 1. Interactive Setup
Run the setup wizard to configure your directories, connect Garmin and Strava accounts, and set your default weather location (which can be geocoded automatically or determined via IP address):
```bash
running-cli setup
```
*Notes:*
- *For Garmin, your password will be requested to generate session tokens under `~/.running_cli/garmin_session/` but **will not** be saved to your `config.json`.*
- *For Strava, the CLI will open your browser to authorize access, automatically retrieving tokens using a temporary local server listening on `http://localhost:8000`.*
- *For Weather, the setup configures your default city/location, coordinates, and resolves them via the Open-Meteo Geocoding API.*

### 2. Check Connection Status
Verify your settings, check Garmin session status, Strava OAuth token health, and weather configuration with:
```bash
running-cli status
```

### 3. Sync Activities & Weather
The sync command pulls activities from Garmin and Strava, merges/deduplicates them, and updates your Obsidian weekly log. In addition, it fetches weather forecast data for the corresponding period and writes/safely-updates a `WeeklyWeather.md` note in your vault containing:
- Sunrise and sunset times.
- Daily weather conditions (emoji-coded).
- An hourly weather breakdown between **5:30 AM and 7:00 PM** including temperatures, wind details, humidity, and precipitation probability.
- A **Running Perceived Exertion (RPE) impact score** (from 0 to 10+) estimating how heat, humidity, and wind speed affect run difficulty.

Commands:
- **Default (Last 7 days)**:
  ```bash
  running-cli sync
  ```
- **Custom lookback window**:
  ```bash
  running-cli sync --days 14
  ```
- **Specific week or date range**:
  ```bash
  running-cli sync --start-date 2026-05-18 --end-date 2026-05-24
  ```

### 4. View Cached Activities
Once activities are synced and stored in the local cache, you can retrieve and view any week's activity breakdown and log directly in the terminal without hitting external APIs:
- **Current week**:
  ```bash
  running-cli view
  ```
- **Specific week (by date)**:
  ```bash
  running-cli view --week 2026-05-22
  ```

---

## Running Tests
Run the unit test suite using `pytest`:
```bash
pytest
```
The test suite covers configuration checking, activity deduplication logic, unit conversion formatters, and markdown safe-updating.
