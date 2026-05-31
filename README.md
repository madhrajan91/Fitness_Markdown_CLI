# running-cli

A Python command-line interface (CLI) tool that fetches your sports activity data from Garmin Connect and Strava, caches the raw activities locally, merges and deduplicates them, and summarizes them into a local Markdown directory/vault (which naturally fits as a folder inside an Obsidian vault) as rich weekly notes (`Week of YYYY-MM-DD.md`).

## Features
- **All Sports Supported**: Supports Runs, Rides, Swims, Walks, Hikes, and more.
- **Weekly Summarization**: Consolidates activities for a week (Monday to Sunday) into a single Markdown file.
- **In-place Sync (Safe Update)**: Keeps weekly logs updated if you request syncs again, preserving your custom journaling or manual notes written in those files.
- **Robust Deduplication**: Automatically merges Garmin and Strava entries representing the same activity by comparing start times and distance thresholds.
- **Custom Units**: Configurable measurement units (default is miles, supports km).
- **Offline Raw Cache**: Retains raw JSON files in `~/.running_cli/cache/` so you can regenerate or reformat weekly entries without hitting external APIs again.

---

## Prerequisites
- **Python 3.8+** (Tested on Python 3.14)
- **Garmin Connect Account** (Password is required only during initial setup; session cookies/tokens are stored securely in `~/.running_cli/garmin_session/` to enable passwordless sync and support Multi-Factor Authentication)
- **Strava Developer Account**: Create an application at [Strava settings/api](https://www.strava.com/settings/api) to get a `Client ID` and `Client Secret`.

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
Run the setup wizard to connect your accounts and configure your local Markdown vault directory path (which naturally fits as a folder inside an Obsidian vault):
```bash
running-cli setup
```
*Notes:*
- *For Garmin, your password will be requested to generate session tokens under `~/.running_cli/garmin_session/` but **will not** be saved to your `config.json`.*
- *For Strava, the CLI will open your browser to authorize access, automatically retrieving tokens using a temporary local server listening on `http://localhost:8000`.*

### 2. Check Connection Status
Verify your settings and check your connection health with:
```bash
running-cli status
```

### 3. Sync Activities
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
