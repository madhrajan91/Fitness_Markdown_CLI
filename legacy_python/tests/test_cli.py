import os
import json
import pytest
from click.testing import CliRunner
from running_cli.main import cli

def test_view_command_no_activities(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock load cached methods to return empty lists
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda start, end: [])
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda start, end: [])
    
    result = runner.invoke(cli, ["view", "--week", "2026-05-22"])
    
    assert result.exit_code == 0
    assert "Retrieving cached activities for Week of 2026-05-18" in result.output
    assert "No cached activities found for this week." in result.output

def test_view_command_with_activities(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "km"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock loaded activities
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Boston Running",
        "activityType": {"typeKey": "running"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10000.0,
        "duration": 3000.0,
        "averageHR": 150.0,
        "maxHR": 170.0,
        "elevationGain": 150.0,
        "locationName": "Belmont"
    }]
    
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda start, end: garmin_raw)
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda start, end: [])
    
    result = runner.invoke(cli, ["view", "--week", "2026-05-22"])
    
    assert result.exit_code == 0
    assert "Retrieving cached activities for Week of 2026-05-18" in result.output
    assert "Weekly Breakdown:" in result.output
    assert "Run: 10.00 km" in result.output
    assert "Boston Running" in result.output
    assert "Belmont" in result.output
    assert "garmin" in result.output
    assert "Activity Links:" in result.output
    assert "https://connect.garmin.com/modern/activity/1111" in result.output


def test_open_command_no_activities(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock load cached methods to return empty lists
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda start, end: [])
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda start, end: [])
    
    result = runner.invoke(cli, ["open", "--week", "2026-05-22"])
    
    assert result.exit_code == 0
    assert "Retrieving cached activities for Week of 2026-05-18" in result.output
    assert "No cached activities found for this week." in result.output


def test_open_command_single_source(monkeypatch, tmp_path):
    runner = CliRunner()
    
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "km"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Boston Running",
        "activityType": {"typeKey": "running"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10000.0,
        "duration": 3000.0,
        "averageHR": 150.0,
        "maxHR": 170.0,
        "elevationGain": 150.0
    }]
    
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda start, end: garmin_raw)
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda start, end: [])
    
    import click
    launched_url = None
    def mock_launch(url):
        nonlocal launched_url
        launched_url = url
    monkeypatch.setattr(click, "launch", mock_launch)
    
    result = runner.invoke(cli, ["open", "--week", "2026-05-22"], input="1\n")
    
    assert result.exit_code == 0
    assert "Retrieving cached activities for Week of 2026-05-18" in result.output
    assert "1. Fri, May 22 - 🏃 Run: Boston Running (garmin)" in result.output
    assert "Launching Garmin activity in browser: https://connect.garmin.com/modern/activity/1111" in result.output
    assert launched_url == "https://connect.garmin.com/modern/activity/1111"


def test_open_command_merged_sources(monkeypatch, tmp_path):
    runner = CliRunner()
    
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "km"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Boston Running",
        "activityType": {"typeKey": "running"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10000.0,
        "duration": 3000.0,
    }]
    strava_raw = [{
        "id": 2222,
        "name": "Boston Marathon Workout",
        "sport_type": "Run",
        "start_date_local": "2026-05-22T08:00:00Z",
        "distance": 10000.0,
        "moving_time": 3000.0,
    }]
    
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda start, end: garmin_raw)
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda start, end: strava_raw)
    
    import click
    launched_url = None
    def mock_launch(url):
        nonlocal launched_url
        launched_url = url
    monkeypatch.setattr(click, "launch", mock_launch)
    
    # Select activity 1, and then select 'strava' provider
    result = runner.invoke(cli, ["open", "--week", "2026-05-22"], input="1\nstrava\n")
    
    assert result.exit_code == 0
    assert "Launching Strava activity in browser: https://www.strava.com/activities/2222" in result.output
    assert launched_url == "https://www.strava.com/activities/2222"
    
    # Test again, selecting 'garmin' provider
    result_garmin = runner.invoke(cli, ["open", "--week", "2026-05-22"], input="1\ngarmin\n")
    assert result_garmin.exit_code == 0
    assert "Launching Garmin activity in browser: https://connect.garmin.com/modern/activity/1111" in result_garmin.output
    assert launched_url == "https://connect.garmin.com/modern/activity/1111"

def test_sync_command_flow(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    vault_dir = tmp_path / "vault"
    vault_dir.mkdir()
    temp_config.write_text(json.dumps({
        "obsidian_vault_path": str(vault_dir),
        "obsidian_folder": "Running/Weekly",
        "distance_unit": "miles",
        "garmin": {"email": "test@example.com"},
        "strava": {"client_id": "1", "client_secret": "2", "refresh_token": "3"}
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock fetchers to do nothing and return empty list or small lists
    monkeypatch.setattr("running_cli.main.fetch_and_cache_garmin_activities", lambda *args, **kwargs: [])
    monkeypatch.setattr("running_cli.main.fetch_and_cache_strava_activities", lambda *args, **kwargs: [])
    
    # Mock load cached to return a race activity
    garmin_raw = [{
        "activityId": 123,
        "activityName": "Championship Race",
        "activityType": {"typeKey": "running"},
        "eventType": {"typeKey": "race"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10000.0,
        "duration": 3000.0,
    }]
    
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda *args: garmin_raw)
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda *args: [])
    
    race_paths_called = []
    def mock_write_race_notes(vault, folder, activities, unit):
        nonlocal race_paths_called
        # Verify that we got the race activity
        assert len(activities) == 1
        assert activities[0].is_race is True
        assert activities[0].title == "Championship Race"
        return ["races/summary.md", f"races/{activities[0].title}.md"]
        
    monkeypatch.setattr("running_cli.main.write_race_notes", mock_write_race_notes)
    
    # Run the sync command
    result = runner.invoke(cli, ["sync", "--days", "1"])
    
    assert result.exit_code == 0
    assert "Syncing activities between" in result.output
    assert "Processing race activities..." in result.output
    assert "Synced 1 race(s). Updated index at Races/summary.md" in result.output

def test_sync_command_flow_races_only(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    vault_dir = tmp_path / "vault"
    vault_dir.mkdir()
    temp_config.write_text(json.dumps({
        "obsidian_vault_path": str(vault_dir),
        "obsidian_folder": "Running/Weekly",
        "distance_unit": "miles",
        "garmin": {"email": "test@example.com"},
        "strava": {"client_id": "1", "client_secret": "2", "refresh_token": "3"}
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock fetchers
    monkeypatch.setattr("running_cli.main.fetch_and_cache_garmin_activities", lambda *args, **kwargs: [])
    monkeypatch.setattr("running_cli.main.fetch_and_cache_strava_activities", lambda *args, **kwargs: [])
    
    # Mock load cached
    garmin_raw = [{
        "activityId": 123,
        "activityName": "Championship Race",
        "activityType": {"typeKey": "running"},
        "eventType": {"typeKey": "race"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10000.0,
        "duration": 3000.0,
    }]
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda *args: garmin_raw)
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda *args: [])
    
    # We want to check that write_weekly_note is NOT called
    weekly_note_called = False
    def mock_write_weekly_note(*args, **kwargs):
        nonlocal weekly_note_called
        weekly_note_called = True
        return "dummy_weekly_note.md"
    monkeypatch.setattr("running_cli.main.write_weekly_note", mock_write_weekly_note)
    
    race_notes_called = False
    def mock_write_race_notes(vault, folder, activities, unit):
        nonlocal race_notes_called
        race_notes_called = True
        return ["races/summary.md"]
    monkeypatch.setattr("running_cli.main.write_race_notes", mock_write_race_notes)
    
    # Run the sync command with --races-only
    result = runner.invoke(cli, ["sync", "--days", "1", "--races-only"])
    
    assert result.exit_code == 0
    assert "Skipping weekly notes generation (--races-only)." in result.output
    assert weekly_note_called is False
    assert race_notes_called is True

def test_races_command(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock cached activities (one race, one non-race, one later race)
    garmin_raw = [
        {
            "activityId": 111,
            "activityName": "Early Race",
            "activityType": {"typeKey": "running"},
            "eventType": {"typeKey": "race"},
            "startTimeLocal": "2026-05-10 08:00:00",
            "distance": 10000.0,
            "duration": 3000.0,
        },
        {
            "activityId": 222,
            "activityName": "Regular Run",
            "activityType": {"typeKey": "running"},
            "eventType": {"typeKey": "uncategorized"},
            "startTimeLocal": "2026-05-15 08:00:00",
            "distance": 8000.0,
            "duration": 2400.0,
        },
        {
            "activityId": 333,
            "activityName": "Championship Race",
            "activityType": {"typeKey": "running"},
            "eventType": {"typeKey": "race"},
            "startTimeLocal": "2026-05-22 08:00:00",
            "distance": 10000.0,
            "duration": 3000.0,
        }
    ]
    
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda *args: garmin_raw)
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda *args: [])
    
    # Test races command showing all
    result = runner.invoke(cli, ["races"])
    assert result.exit_code == 0
    assert "=== Cached Races ===" in result.output
    assert "Showing all 2 historical race(s):" in result.output
    assert "Early Race" in result.output
    assert "Championship Race" in result.output
    assert "Regular Run" not in result.output
    
    # Test races command with --start-date filtering out Early Race
    result_filter = runner.invoke(cli, ["races", "--start-date", "2026-05-15"])
    assert result_filter.exit_code == 0
    assert "=== Cached Races ===" in result_filter.output
    assert "Filtering starting from: 2026-05-15" in result_filter.output
    assert "Early Race" not in result_filter.output
    assert "Championship Race" in result_filter.output
    
    # Test invalid date formatting
    result_invalid = runner.invoke(cli, ["races", "--start-date", "invalid"])
    assert result_invalid.exit_code == 0
    assert "Error: Invalid --start-date format" in result_invalid.output

def test_trail_runs_command(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock cached activities
    # 1. Trail run (Run, from strava, elevation >= 609.6m)
    strava_raw = [
        {
            "id": 111,
            "name": "Mount Washington Trail Run",
            "type": "Run",
            "start_date_local": "2026-05-18T08:00:00Z",
            "distance": 15000.0,
            "moving_time": 7200.0,
            "total_elevation_gain": 1200.0, # meters
        },
        # 2. Non-trail run (Run, from strava, low elevation)
        {
            "id": 222,
            "name": "Flat Run",
            "type": "Run",
            "start_date_local": "2026-05-20T08:00:00Z",
            "distance": 10000.0,
            "moving_time": 3000.0,
            "total_elevation_gain": 10.0,
        }
    ]
    
    # Garmin-only mountain run (no strava)
    garmin_raw = [
        {
            "activityId": 333,
            "activityName": "Garmin-only Climb",
            "activityType": {"typeKey": "running"},
            "startTimeLocal": "2026-05-22 08:00:00",
            "distance": 12000.0,
            "duration": 5400.0,
            "elevationGain": 800.0
        }
    ]
    
    monkeypatch.setattr("running_cli.main.load_cached_garmin_activities", lambda *args: garmin_raw)
    monkeypatch.setattr("running_cli.main.load_cached_strava_activities", lambda *args: strava_raw)
    
    # Test trail-runs command showing all
    result = runner.invoke(cli, ["trail-runs"])
    assert result.exit_code == 0
    assert "=== Cached Trail Runs ===" in result.output
    assert "Showing all 1 historical trail run(s):" in result.output
    assert "Mount Washington Trail Run" in result.output
    assert "Flat Run" not in result.output
    assert "Garmin-only Climb" not in result.output
    
    # Test invalid date formatting
    result_invalid = runner.invoke(cli, ["trail-runs", "--start-date", "invalid"])
    assert result_invalid.exit_code == 0
    assert "Error: Invalid --start-date format" in result_invalid.output









