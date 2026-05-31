# Technical Implementation Specification

This document details the software design, data flows, deduplication logic, and file schemas for the `running-cli` sync tool.

---

## Architecture Overview

```mermaid
graph TD
    A[CLI: running-cli sync] --> B[Retrieval Engine]
    B --> C[Garmin Fetcher]
    B --> D[Strava Fetcher]
    B --> WeatherFetch[Weather Fetcher]
    C -->|Raw JSON| E[Local Cache: ~/.running_cli/cache/garmin]
    D -->|Raw JSON| F[Local Cache: ~/.running_cli/cache/strava]
    E --> G[Merger & Deduplicator]
    F --> G
    G -->|Merged Activity List| H[Storage Engine]
    WeatherFetch -->|Forecast Data| H
    H -->|Load complete week from Cache| H
    H -->|Generate/Safe Update Markdown Notes| I[Obsidian Vault Folder]
```

The tool splits responsibilities into three distinct modules:
1. **Retrieval**: Connects to third-party Garmin, Strava, and Open-Meteo APIs, pulls raw activity and forecast data, and caches activity payloads offline.
2. **Core / Merger**: Normalizes the raw schema from Garmin and Strava and performs clock-drift and distance-threshold matching to link overlapping records.
3. **Storage**: Groups activities into ISO-weeks, updates metadata YAML frontmatter, generates weather tables and running perceived exertion estimates, and formats markdown summary logs. It edits existing Obsidian files in-place using tag-based safe boundaries.

---

## Module Specifications

### 1. Configuration & Directories
Configuration settings are stored in `~/.running_cli/config.json`:
- `obsidian_vault_path` (string): Absolute path to target vault.
- `obsidian_folder` (string): Vault subfolder where weekly files are saved.
- `distance_unit` (enum: `"miles"`, `"km"`): Target system units.
- `weather_location` (string): Configured or geocoded default weather location name.
- `weather_lat` (float): Latitude of the default weather location.
- `weather_lon` (float): Longitude of the default weather location.
- `garmin`: Email (Garmin password is never stored).
- `strava`: Client ID, Client Secret, and Refresh Token.

Caches and session states:
- `~/.running_cli/garmin_session/`: Tokenstore directory managed by `garminconnect` to preserve session cookies and avoid repeated logins/MFA.
- `~/.running_cli/cache/garmin/`: Raw JSON activity summaries fetched from Garmin (named by `{activityId}.json`).
- `~/.running_cli/cache/strava/`: Raw JSON activity summaries fetched from Strava (named by `{id}.json`).

### 2. Retrieval Engine

#### Garmin Connect Retrieval (`src/running_cli/retrieval/garmin.py`)
- Initializes client using `garminconnect.Garmin` utilizing a local tokenstore session under `~/.running_cli/garmin_session/` via the underlying `garth` library.
- Attempts passwordless connection using stored session tokens first.
- Prompts for password and Multi-Factor Authentication (MFA) via standard input only when initializing or when the tokenstore session expires.
- Method `get_activities_by_date(start_date, end_date)` queries activities.
- Saves raw responses in cache directory.

#### Strava Retrieval (`src/running_cli/retrieval/strava.py`)
- Uses standard HTTP POST to refresh the Strava token using `client_id`, `client_secret`, and `refresh_token`.
- Automatically checks for refresh token rotations and writes back to `config.json`.
- Queries `GET https://www.strava.com/api/v3/athlete/activities` filtering via `before` and `after` epoch timestamps.
- Caches raw responses.

#### Weather & Geolocation Retrieval (`src/running_cli/retrieval/weather.py`)
- **Geocoding lookup**: Method `geocode_location(location_name)` queries the **Open-Meteo Geocoding API** (`https://geocoding-api.open-meteo.com/v1/search`) to resolve human-readable city names to latitude and longitude coordinates.
- **IP-based Geolocation fallback**: Method `geolocate_by_ip()` queries public endpoints `http://ip-api.com/json/` (with fallback to `https://ipapi.co/json/`) to automatically detect the user's location and coordinates based on their public IP.
- **Forecast Fetcher**: Method `fetch_weather(lat, lon, date_str, unit, end_date_str)` queries the **Open-Meteo Forecast API** (`https://api.open-meteo.com/v1/forecast`) for:
  - Hourly features: `temperature_2m`, `apparent_temperature`, `relative_humidity_2m`, `wind_speed_10m`, `wind_direction_10m`, `precipitation_probability`, and WMO `weather_code`.
  - Daily features: `sunrise`, `sunset`, `temperature_2m_max`, `temperature_2m_min`, `wind_speed_10m_max`, and `weather_code`.
  - Converts temperature units (Fahrenheit/Celsius) and wind speeds (mph/kmh) dynamically based on the configuration unit (`miles`/`km`).

### 3. Core Merger & Deduplication (`src/running_cli/core/merger.py`)
Because running logs recorded on Garmin typically sync automatically to Strava, Garmin and Strava raw activities often duplicate the same session.
The `merge_activities` function compares activities from both providers to link duplicates.

**Deduplication Criteria**:
1. **Sport compatibility**: The activity sports must match (e.g. Run to Run, Swim to Swim) unless one of them is marked as `"Other"`.
2. **Start time difference**: Absolute start time difference is less than **10 minutes** (600 seconds) to account for different GPS startup times, upload latency, and clock drifts.
3. **Distance difference**: Absolute distance difference must be within **10%** of the average distance, or less than **500 meters** (useful for short activities).

**Precedence Rules for Merged Activities**:
- **Title**: Prioritize Strava's custom title (e.g., `"Morning Trail with friends"`) over Garmin's automatic title (e.g., `"Boston Running"`).
- **Distance**: Prioritize Garmin's raw device GPS distance.
- **Duration**: Prioritize Garmin's raw device elapsed duration.
- **Heart Rate**: Use Garmin's average and max heart rate, falling back to Strava.
- **Elevation**: Use Garmin's barometric elevation gain.
- **Description**: Prioritize Strava's description, falling back to Garmin.

### 4. Storage Engine (`src/running_cli/storage/obsidian.py`)

#### Unit & Format Standards
- **Distances**: 
  - Standard activities: Displays in miles (`mi`) or kilometers (`km`).
  - Swims: Displays in yards (`yd`) if unit preference is `miles`, or meters (`m`) if preference is `km`.
- **Pace**:
  - Running/Walking/Hiking: Minutes per unit (e.g., `8:02/mi` or `5:00/km`).
  - Cycling: Speed in unit-per-hour (e.g., `15.4 mph` or `24.0 km/h`).
  - Swimming: Minutes per 100 units (e.g., `1:45/100yd` or `2:00/100m`).
- **Elevation**: Feet (`ft`) or meters (`m`).
- **Duration**: `H:MM:SS` or `MM:SS`.

#### Starting Locations & Map Pins
- **Coordinates & Location Names**: Starting coordinates (latitude/longitude) and location names are extracted from:
  - Garmin: `locationName`, `startLatitude`, `startLongitude`
  - Strava: `location_city`/`location_state`, `start_latlng`
- **Map Pin Link**: A Google Maps link is generated using the starting coordinates: `https://www.google.com/maps/search/?api=1&query={lat},{lon}`
- **Obsidian Column**: The Obsidian log table features a `Location` column rendering the map pin and location name as a link: `📍 [Location Name](google_maps_url)` (falling back to `📍 [Map](google_maps_url)` if no city/location name is available).

#### Clickable Links & Collapsible Descriptions
- **Table Links**: The "Source" column in the weekly activity log renders clickable Markdown links to each activity page if IDs are available:
  - Garmin: `[Garmin](https://connect.garmin.com/modern/activity/{garmin_id})`
  - Strava: `[Strava](https://www.strava.com/activities/{strava_id})`
- **Descriptions Block**: Displays a dedicated `### Activity Notes` section below the weekly table containing a collapsible `<details>` and `<summary>` element for each activity with a description.

#### Races Synchronization
- **Race Detection**: Race status is determined during activity parsing:
  - Garmin: Evaluates `eventType.typeKey == "race"`.
  - Strava: Evaluates `workout_type == 1`.
  - Merger: The unified activity is flagged as a race if either source is a race.
- **Races Folder**: Created as a subfolder `Races/` inside the configured Obsidian vault folder.
- **Individual Race Notes**: Written as `Races/YYYY-MM-DD - <Sanitized Title>.md`. Frontmatter and stats are stored inside `%% START_RACE_SUMMARY %%` and `%% END_RACE_SUMMARY %%` blocks. User-written journal entries under a `## Personal Notes` header are preserved across syncs.
- **Central Index (`summary.md`)**: Maintained at `Races/summary.md` listing all historical races in a summary table. Wikilinks (e.g. `[[2026-05-18 - Boston Marathon|Boston Marathon]]`) connect the index table to individual race files. The table is updated inside `%% START_RACES_LIST %%` and `%% END_RACES_LIST %%` comments to preserve any manual edits.

#### Trail Runs Synchronization
- **Trail Run Detection**: Filters merged activities that are runs (`sport == "Run"`), have `elevation_gain_meters >= 609.6` (2000 ft), and include `strava` as a source.
- **TrailRuns Folder**: Created as a subfolder `TrailRuns/` inside the configured Obsidian vault folder.
- **Individual Trail Run Notes**: Written as `TrailRuns/YYYY-MM-DD - <Sanitized Title>.md`. Frontmatter and stats are stored inside `%% START_TRAIL_RUN_SUMMARY %%` and `%% END_TRAIL_RUN_SUMMARY %%` blocks. User-written journal entries under a `## Personal Notes` header are preserved across syncs.
- **Central Index (`summary.md`)**: Maintained at `TrailRuns/summary.md` listing all historical trail runs in a summary table. Wikilinks (e.g., `[[2026-05-18 - Mount Washington trail run|Mount Washington trail run]]`) connect the index table to individual trail run files. The table is updated inside `%% START_TRAIL_RUNS_LIST %%` and `%% END_TRAIL_RUNS_LIST %%` comments to preserve any manual edits.

#### Weekly Weather Outlook & Exertion Index
- **File Name**: `WeeklyWeather.md` inside the configured vault folder.
- **Weather Window**: Estimates running weather parameters for the daylight hours (**5:30 AM to 7:00 PM**).
- **Running Perceived Exertion (RPE)**: Estimating running effort scores (`0.0` to `10.0+`) based on:
  - **Humidity/Dew Point (Max 6.0 points)**: Uses a simplified dew point approximation: $T_d = T - \frac{100 - RH}{5}$. Adds points up to 6.0 for dew points exceeding $50^\circ\text{F}$.
  - **Temperature (Max 5.0 points)**: Ideal range is $32^\circ\text{F}$ to $60^\circ\text{F}$ (+0.0 points). Points are added for cold ($<32^\circ\text{F}$) or warm ($>60^\circ\text{F}$) extremes.
  - **Wind Speed (Max 3.5 points)**: Points added for resistance starting at 10 mph.
- **RPE Category Outputs**:
  - `Ideal` (Score $\le 1.5$): Perfect running conditions.
  - `Moderate` (Score $1.6 - 3.0$): Slightly increased effort.
  - `Hard` (Score $3.1 - 5.0$): Slow pace slightly.
  - `Very Hard` (Score $5.1 - 7.0$): Run by feel, cardiovascular strain.
  - `Extreme` (Score $> 7.0$): Avoid peak hours or run indoors.
- **Outlooks Table**: Outputs a 7-day weather summary listing the condition name/emoji, temperature range, maximum wind speed, average daylight humidity, sunrise/sunset times, and the "Best Run Window" (hour with the lowest RPE score).
- **Hourly Detailed Tables**: A collapsible section displaying weather variables and RPE ratings hourly.

#### Safe-Update Markdown Parser
To ensure manual note modifications in Obsidian are preserved:
1. **Frontmatter Parsing**: Uses `PyYAML` to read frontmatter. When updating, it merges updated program metrics while preserving other custom fields manually written by the user.
2. **Summary Replacements**: Searches for comment boundaries:
   - Weekly notes: `%% START_WEEKLY_SUMMARY %%` / `%% END_WEEKLY_SUMMARY %%`
   - Race summaries: `%% START_RACE_SUMMARY %%` / `%% END_RACE_SUMMARY %%`
   - Trail run summaries: `%% START_TRAIL_RUN_SUMMARY %%` / `%% END_TRAIL_RUN_SUMMARY %%`
   - Races Central Index list: `%% START_RACES_LIST %%` / `%% END_RACES_LIST %%`
   - Trail Runs Central Index list: `%% START_TRAIL_RUNS_LIST %%` / `%% END_TRAIL_RUNS_LIST %%`
   - Weather Outlook table: `%% START_WEATHER_OUTLOOK %%` / `%% END_WEATHER_OUTLOOK %%`
   - Weather Hourly breakdown: `%% START_HOURLY_OUTLOOK %%` / `%% END_HOURLY_OUTLOOK %%`
   If boundaries exist, it replaces *only* the content between the tags, keeping everything else in the body unchanged. If boundaries do not exist, it uses a predefined missing tags template to construct the file.

---

### 5. CLI Command Reference
- **`setup`**: Interactive wizard to configure Obsidian vault paths, system unit configurations, Garmin Connect credentials, Strava browser OAuth redirects, and default weather location preferences (geocoded via Open-Meteo).
- **`status`**: Displays configurations, checks Garmin session file integrity, tests Strava access token health, prints default weather location geocoded metrics, and reports public IP detection details.
- **`sync`**: Fetches activities in the specified date range, caches raw JSONs locally, merges and deduplicates overlapping Garmin/Strava activities, syncs race and trail run documents, and automatically generates/safely-updates the `WeeklyWeather.md` forecast summary. Supports `--days`, `--start-date`/`--end-date`, and `--races-only`.
- **`view`**: Reads local cache for a specified week (defaulting to the current week) and prints a formatted weekly summary and activity log table directly in the terminal, completely offline. The output includes an interactive `Activity Links` section mapping to clickable terminal hyperlinks using ANSI escape sequences (`\033]8;;{url}\033\\{label}\033]8;;\033\\`), while hiding activity descriptions.
- **`open`**: Prompts the user to select an activity from a specified week (defaulting to the current week) and opens the activity's source page in the default web browser via `click.launch`. If the activity is merged from multiple sources, the command prompts the user to select which provider's page to launch.
- **`races`**: Reads all local cache files, filters for race activities, sorts them chronologically, and prints a formatted summary log table and source hyperlinks in the terminal. Supports an optional `--start-date YYYY-MM-DD` flag to filter the displayed race history starting from that date.
- **`trail-runs`**: Reads all local cache files, filters for trail runs, sorts them chronologically, and prints a formatted summary log table and source hyperlinks in the terminal. Supports an optional `--start-date YYYY-MM-DD` flag to filter the displayed trail run history starting from that date.


