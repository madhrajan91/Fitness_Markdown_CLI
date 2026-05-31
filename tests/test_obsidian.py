import os
from datetime import date, time, datetime
from running_cli.storage import obsidian
from running_cli.core.models import MergedActivity

def test_get_monday_of_week():
    # 2026-05-22 is a Friday. The Monday of that week is 2026-05-18.
    assert obsidian.get_monday_of_week(date(2026, 5, 22)) == date(2026, 5, 18)
    # 2026-05-18 is a Monday. The Monday of that week is 2026-05-18.
    assert obsidian.get_monday_of_week(date(2026, 5, 18)) == date(2026, 5, 18)
    # 2026-05-24 is a Sunday. The Monday of that week is 2026-05-18.
    assert obsidian.get_monday_of_week(date(2026, 5, 24)) == date(2026, 5, 18)

def test_format_duration():
    assert obsidian.format_duration(45) == "0m 45s"
    assert obsidian.format_duration(2700) == "45m 00s"
    assert obsidian.format_duration(3672) == "1h 01m"
    assert obsidian.format_duration(7200) == "2h 00m"

def test_format_duration_compact():
    assert obsidian.format_duration_compact(45) == "00:45"
    assert obsidian.format_duration_compact(3672) == "1:01:12"

def test_format_distance():
    # unit = miles
    assert obsidian.format_distance("Run", 1609.34, "miles") == "1.00 mi"
    assert obsidian.format_distance("Swim", 1000, "miles") == "1094 yd"
    
    # unit = km
    assert obsidian.format_distance("Run", 1000, "km") == "1.00 km"
    assert obsidian.format_distance("Swim", 1000, "km") == "1000 m"

def test_format_pace_or_speed():
    # Running pace: min/mi or min/km
    # 10km in 50 mins (3000 seconds)
    # unit = km -> 5:00 / km
    assert obsidian.format_pace_or_speed("Run", 10000, 3000, "km") == "5:00/km"
    # unit = miles -> 10000m = 6.21371 mi. 50 mins -> 8:02 / mi
    assert obsidian.format_pace_or_speed("Run", 10000, 3000, "miles") == "8:02/mi"

    # Cycling speed: mph or km/h
    # 20km in 1 hour (3600 seconds)
    assert obsidian.format_pace_or_speed("Ride", 20000, 3600, "km") == "20.0 km/h"
    # 20km = 12.4274 miles in 1 hour -> 12.4 mph
    assert obsidian.format_pace_or_speed("Ride", 20000, 3600, "miles") == "12.4 mph"

    # Swim pace: min/100m or min/100yd
    # 1000m in 20 mins (1200 seconds) -> 2:00/100m
    assert obsidian.format_pace_or_speed("Swim", 1000, 1200, "km") == "2:00/100m"
    # 1000m = 1093.61 yards. 20 mins -> 1:49/100yd
    assert obsidian.format_pace_or_speed("Swim", 1000, 1200, "miles") == "1:49/100yd"

def test_safe_update_new_content():
    content = ""
    new_fm = {"type": "test", "val": 42}
    new_summary = "Generated summary goes here"
    
    updated = obsidian.safe_update_markdown(content, new_fm, new_summary)
    
    assert "type: test" in updated
    assert "val: 42" in updated
    assert "%% START_WEEKLY_SUMMARY %%" in updated
    assert "Generated summary goes here" in updated
    assert "%% END_WEEKLY_SUMMARY %%" in updated

def test_safe_update_existing_content():
    existing = """---
type: weekly-activities
custom_user_key: "important metadata"
activity_count: 1
---
# Weekly Log

Some user intro notes here.

%% START_WEEKLY_SUMMARY %%
Old summary that should be replaced.
%% END_WEEKLY_SUMMARY %%

## User Notes
- I felt great this week.
- Met my training plan goals!
"""

    new_fm = {
        "activity_count": 3,
        "new_metadata": "new"
    }
    new_summary = "This is the newly generated weekly table and breakdown."
    
    updated = obsidian.safe_update_markdown(existing, new_fm, new_summary)
    
    # Verify updated frontmatter
    assert "custom_user_key: important metadata" in updated or "custom_user_key: \"important metadata\"" in updated
    assert "activity_count: 3" in updated
    assert "new_metadata: new" in updated
    
    # Verify updated body
    assert "Some user intro notes here." in updated
    assert "%% START_WEEKLY_SUMMARY %%" in updated
    assert "This is the newly generated weekly table and breakdown." in updated
    assert "%% END_WEEKLY_SUMMARY %%" in updated
    assert "Old summary that should be replaced" not in updated
    
    # Verify user notes are preserved
    assert "## User Notes" in updated
    assert "- I felt great this week." in updated
    assert "- Met my training plan goals!" in updated

def test_write_weekly_note(tmp_path):
    # Set up dummy activities
    act1 = MergedActivity(
        date=date(2026, 5, 18),
        start_time=time(8, 0, 0),
        sport="Run",
        title="Monday Morning Recovery",
        distance_meters=8000.0,  # ~5 miles
        duration_seconds=2400.0, # 40 mins
        avg_hr=145,
        max_hr=160,
        elevation_gain_meters=50,
        garmin_id=101,
        strava_id=201,
        description="Felt great during recovery run!",
        location_name="Belmont",
        latitude=42.381727,
        longitude=-71.168501,
        sources=["garmin", "strava"]
    )
    
    act2 = MergedActivity(
        date=date(2026, 5, 20),
        start_time=time(18, 30, 0),
        sport="Swim",
        title="Pool Laps",
        distance_meters=1500.0,
        duration_seconds=2700.0, # 45 mins
        avg_hr=None,
        max_hr=None,
        elevation_gain_meters=0,
        garmin_id=102,
        strava_id=None,
        sources=["garmin"]
    )
    
    vault_dir = tmp_path / "vault"
    obsidian_folder = "Training/Weekly"
    
    monday_date = date(2026, 5, 18)
    
    # Run the exporter
    filepath = obsidian.write_weekly_note(
        vault_path=str(vault_dir),
        folder=obsidian_folder,
        monday_date=monday_date,
        activities=[act1, act2],
        unit="miles"
    )
    
    assert os.path.exists(filepath)
    
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
        
    assert "week_of: '2026-05-18'" in content or "week_of: 2026-05-18" in content
    assert "activity_count: 2" in content
    assert "total_duration_friendly: 1h 25m" in content
    
    # Verify sport distance conversions in frontmatter
    # Run: 8000m * 0.000621371 = 4.97 mi
    # Swim: 1500m * 1.09361 = 1640 yd
    assert "run: 4.97" in content
    assert "swim: 1640" in content
    
    # Verify body contents
    assert "Monday Morning Recovery" in content
    assert "Pool Laps" in content
    assert "[Garmin](https://connect.garmin.com/modern/activity/101)" in content
    assert "[Strava](https://www.strava.com/activities/201)" in content
    assert "[Garmin](https://connect.garmin.com/modern/activity/102)" in content
    assert "📍 [Belmont](https://www.google.com/maps/search/?api=1&query=42.381727,-71.168501)" in content
    
    # Verify activity notes section
    assert "### Activity Notes" in content
    assert "<details>" in content
    assert "<summary><b>Mon, May 18 - 🏃 Monday Morning Recovery</b></summary>" in content
    assert "Felt great during recovery run!" in content

def test_write_race_notes(tmp_path):
    # Set up dummy activities
    act1 = MergedActivity(
        date=date(2026, 5, 18),
        start_time=time(8, 0, 0),
        sport="Run",
        title="Boston Marathon",
        distance_meters=42195.0,  # ~26.2 miles
        duration_seconds=10800.0, # 3 hours
        avg_hr=165,
        max_hr=180,
        elevation_gain_meters=150,
        garmin_id=101,
        strava_id=201,
        description="A great race!",
        location_name="Boston",
        latitude=42.3584,
        longitude=-71.0598,
        is_race=True,
        sources=["garmin", "strava"]
    )
    
    act2 = MergedActivity(
        date=date(2026, 5, 20),
        start_time=time(18, 30, 0),
        sport="Run",
        title="Local 5k",
        distance_meters=5000.0,
        duration_seconds=1200.0,
        avg_hr=170,
        max_hr=185,
        elevation_gain_meters=20,
        garmin_id=None,
        strava_id=202,
        description=None,
        is_race=True,
        sources=["strava"]
    )

    # Act3 is a non-race run which should NOT be included in races folder
    act3 = MergedActivity(
        date=date(2026, 5, 21),
        start_time=time(7, 0, 0),
        sport="Run",
        title="Easy Recovery Run",
        distance_meters=8000.0,
        duration_seconds=2400.0,
        is_race=False,
        sources=["garmin"]
    )

    vault_dir = tmp_path / "vault"
    obsidian_folder = "Areas/Running"
    
    # 1. Run the exporter for the first time
    paths = obsidian.write_race_notes(
        vault_path=str(vault_dir),
        folder=obsidian_folder,
        activities=[act1, act2, act3],
        unit="miles"
    )
    
    # Verify expected files are returned
    expected_race1_path = os.path.join(str(vault_dir), obsidian_folder, "Races", "2026-05-18 - Boston Marathon.md")
    expected_race2_path = os.path.join(str(vault_dir), obsidian_folder, "Races", "2026-05-20 - Local 5k.md")
    expected_summary_path = os.path.join(str(vault_dir), obsidian_folder, "Races", "summary.md")
    
    assert expected_race1_path in paths
    assert expected_race2_path in paths
    assert expected_summary_path in paths
    assert len(paths) == 3
    
    # Verify non-race was NOT created
    non_race_path = os.path.join(str(vault_dir), obsidian_folder, "Races", "2026-05-21 - Easy Recovery Run.md")
    assert not os.path.exists(non_race_path)
    
    # Verify contents of race 1
    with open(expected_race1_path, 'r', encoding='utf-8') as f:
        content_race1 = f.read()
    assert "type: race-activity" in content_race1
    assert "sport: Run" in content_race1
    assert "distance: 26.22" in content_race1  # 42195m in miles
    assert "duration_seconds: 10800" in content_race1
    assert "A great race!" in content_race1
    assert "📍 [Boston](https://www.google.com/maps/search/?api=1&query=42.3584,-71.0598)" in content_race1
    assert "## Personal Notes" in content_race1
    assert "*Write your personal race report or notes here...*" in content_race1

    # Verify summary contents
    with open(expected_summary_path, 'r', encoding='utf-8') as f:
        summary_content = f.read()
    assert "type: race-summary" in summary_content
    assert "total_races: 2" in summary_content
    assert "[[2026-05-18 - Boston Marathon|Boston Marathon]]" in summary_content
    assert "[[2026-05-20 - Local 5k|Local 5k]]" in summary_content
    
    # 2. Add some personal notes to Race 1 and edit summary.md
    with open(expected_race1_path, 'w', encoding='utf-8') as f:
        f.write(content_race1 + "\nMy custom race report details.\n")
        
    with open(expected_summary_path, 'w', encoding='utf-8') as f:
        f.write(summary_content + "\nMy custom general race goals.\n")
        
    # 3. Run exporter again to check if user edits are preserved
    paths_re = obsidian.write_race_notes(
        vault_path=str(vault_dir),
        folder=obsidian_folder,
        activities=[act1, act2, act3],
        unit="miles"
    )
    
    # Check Race 1 updated content
    with open(expected_race1_path, 'r', encoding='utf-8') as f:
        content_race1_re = f.read()
    assert "My custom race report details." in content_race1_re
    assert "A great race!" in content_race1_re
    
    # Check Summary updated content
    with open(expected_summary_path, 'r', encoding='utf-8') as f:
        summary_content_re = f.read()
    assert "My custom general race goals." in summary_content_re
    assert "[[2026-05-18 - Boston Marathon|Boston Marathon]]" in summary_content_re

def test_write_trail_run_notes(tmp_path):
    # Set up dummy activities
    # act1 is a trail run (Run, from strava, elevation >= 609.6m)
    act1 = MergedActivity(
        date=date(2026, 5, 18),
        start_time=time(8, 0, 0),
        sport="Run",
        title="Mount Washington trail run",
        distance_meters=15000.0,
        duration_seconds=7200.0,
        elevation_gain_meters=1200.0, # over 2000 ft (609.6 m)
        garmin_id=101,
        strava_id=201,
        description="Epic climb!",
        sources=["garmin", "strava"]
    )
    
    # act2 is a run with high elevation but garmin-only (not from strava), so it should NOT be in TrailRuns
    act2 = MergedActivity(
        date=date(2026, 5, 20),
        start_time=time(18, 30, 0),
        sport="Run",
        title="Garmin-Only Mountain Climb",
        distance_meters=12000.0,
        duration_seconds=5400.0,
        elevation_gain_meters=800.0,
        garmin_id=102,
        strava_id=None,
        sources=["garmin"]
    )

    # act3 is a run from strava but low elevation (< 609.6m), so it should NOT be in TrailRuns
    act3 = MergedActivity(
        date=date(2026, 5, 21),
        start_time=time(7, 0, 0),
        sport="Run",
        title="Flat Road Run",
        distance_meters=10000.0,
        duration_seconds=3000.0,
        elevation_gain_meters=10.0,
        garmin_id=None,
        strava_id=203,
        sources=["strava"]
    )

    vault_dir = tmp_path / "vault"
    obsidian_folder = "Areas/Running"
    
    # 1. Run the exporter for the first time
    paths = obsidian.write_trail_run_notes(
        vault_path=str(vault_dir),
        folder=obsidian_folder,
        activities=[act1, act2, act3],
        unit="miles"
    )
    
    expected_trail1_path = os.path.join(str(vault_dir), obsidian_folder, "TrailRuns", "2026-05-18 - Mount Washington trail run.md")
    expected_summary_path = os.path.join(str(vault_dir), obsidian_folder, "TrailRuns", "summary.md")
    
    assert expected_trail1_path in paths
    assert expected_summary_path in paths
    assert len(paths) == 2
    
    # Verify non-trail runs were not created
    assert not os.path.exists(os.path.join(str(vault_dir), obsidian_folder, "TrailRuns", "2026-05-20 - Garmin-Only Mountain Climb.md"))
    assert not os.path.exists(os.path.join(str(vault_dir), obsidian_folder, "TrailRuns", "2026-05-21 - Flat Road Run.md"))
    
    # Verify content of trail run file
    with open(expected_trail1_path, 'r', encoding='utf-8') as f:
        content_trail1 = f.read()
    assert "type: trail-run-activity" in content_trail1
    assert "elevation_gain_meters: 1200.0" in content_trail1
    assert "Epic climb!" in content_trail1
    assert "## Personal Notes" in content_trail1
    
    # Verify summary contents
    with open(expected_summary_path, 'r', encoding='utf-8') as f:
        summary_content = f.read()
    assert "type: trail-run-summary" in summary_content
    assert "total_trail_runs: 1" in summary_content
    assert "[[2026-05-18 - Mount Washington trail run|Mount Washington trail run]]" in summary_content
    
    # 2. Add custom personal notes
    with open(expected_trail1_path, 'w', encoding='utf-8') as f:
        f.write(content_trail1 + "\nMy custom trail run report details.\n")
        
    # 3. Run exporter again to check if user edits are preserved
    obsidian.write_trail_run_notes(
        vault_path=str(vault_dir),
        folder=obsidian_folder,
        activities=[act1, act2, act3],
        unit="miles"
    )
    
    # Check updated content
    with open(expected_trail1_path, 'r', encoding='utf-8') as f:
        content_trail1_re = f.read()
    assert "My custom trail run report details." in content_trail1_re
    assert "Epic climb!" in content_trail1_re


def test_cross_linking(tmp_path):
    from datetime import date, time
    from running_cli.core.models import MergedActivity
    import running_cli.storage.obsidian as obsidian

    # 1. Activity that is both a race and a trail run
    act_both = MergedActivity(
        date=date(2026, 5, 18),
        start_time=time(8, 0, 0),
        sport="Run",
        title="Epic Ridge Race",
        distance_meters=21000.0,
        duration_seconds=7200.0,
        elevation_gain_meters=800.0,  # > 609.6m
        garmin_id=101,
        strava_id=201,
        is_race=True,
        sources=["garmin", "strava"]
    )

    # 2. Activity that is a race only
    act_race = MergedActivity(
        date=date(2026, 5, 19),
        start_time=time(9, 0, 0),
        sport="Run",
        title="Road 10k Race",
        distance_meters=10000.0,
        duration_seconds=2400.0,
        elevation_gain_meters=10.0,
        garmin_id=102,
        strava_id=202,
        is_race=True,
        sources=["garmin", "strava"]
    )

    # 3. Activity that is a trail run only
    act_trail = MergedActivity(
        date=date(2026, 5, 20),
        start_time=time(10, 0, 0),
        sport="Run",
        title="Mount Wilson Climb",
        distance_meters=15000.0,
        duration_seconds=6000.0,
        elevation_gain_meters=700.0,  # > 609.6m
        garmin_id=103,
        strava_id=203,
        is_race=False,
        sources=["garmin", "strava"]
    )

    vault_dir = tmp_path / "vault"
    folder = "Weekly"

    # Write weekly note
    obsidian.write_weekly_note(str(vault_dir), folder, date(2026, 5, 18), [act_both, act_race, act_trail], "miles")
    # Write race notes
    obsidian.write_race_notes(str(vault_dir), folder, [act_both, act_race, act_trail], "miles")
    # Write trail run notes
    obsidian.write_trail_run_notes(str(vault_dir), folder, [act_both, act_race, act_trail], "miles")

    # Paths
    weekly_path = os.path.join(str(vault_dir), folder, "Week of 2026-05-18.md")
    race_both_path = os.path.join(str(vault_dir), folder, "Races", "2026-05-18 - Epic Ridge Race.md")
    race_only_path = os.path.join(str(vault_dir), folder, "Races", "2026-05-19 - Road 10k Race.md")
    trail_both_path = os.path.join(str(vault_dir), folder, "TrailRuns", "2026-05-18 - Epic Ridge Race.md")
    trail_only_path = os.path.join(str(vault_dir), folder, "TrailRuns", "2026-05-20 - Mount Wilson Climb.md")
    
    races_summary_path = os.path.join(str(vault_dir), folder, "Races", "summary.md")
    trail_summary_path = os.path.join(str(vault_dir), folder, "TrailRuns", "summary.md")

    # A. Check weekly note for cross-links
    with open(weekly_path, 'r', encoding='utf-8') as f:
        weekly_content = f.read()
    assert "[[Races/2026-05-18 - Epic Ridge Race|Epic Ridge Race]] (also [[TrailRuns/2026-05-18 - Epic Ridge Race|Trail Run]])" in weekly_content
    assert "[[Races/2026-05-19 - Road 10k Race|Road 10k Race]]" in weekly_content
    assert "[[TrailRuns/2026-05-20 - Mount Wilson Climb|Mount Wilson Climb]]" in weekly_content

    # B. Check individual Race note cross-links
    with open(race_both_path, 'r', encoding='utf-8') as f:
        race_both_content = f.read()
    assert "- **Weekly Log**: [[../Week of 2026-05-18|Week of 2026-05-18]]" in race_both_content
    assert "- **Trail Run Note**: [[../TrailRuns/2026-05-18 - Epic Ridge Race|Trail Run Note]]" in race_both_content

    with open(race_only_path, 'r', encoding='utf-8') as f:
        race_only_content = f.read()
    assert "- **Weekly Log**: [[../Week of 2026-05-18|Week of 2026-05-18]]" in race_only_content
    assert "Trail Run Note" not in race_only_content

    # C. Check individual Trail Run note cross-links
    with open(trail_both_path, 'r', encoding='utf-8') as f:
        trail_both_content = f.read()
    assert "- **Weekly Log**: [[../Week of 2026-05-18|Week of 2026-05-18]]" in trail_both_content
    assert "- **Race Note**: [[../Races/2026-05-18 - Epic Ridge Race|Race Note]]" in trail_both_content

    with open(trail_only_path, 'r', encoding='utf-8') as f:
        trail_only_content = f.read()
    assert "- **Weekly Log**: [[../Week of 2026-05-18|Week of 2026-05-18]]" in trail_only_content
    assert "Race Note" not in trail_only_content

    # D. Check Races index cross-links
    with open(races_summary_path, 'r', encoding='utf-8') as f:
        races_sum_content = f.read()
    assert "[[2026-05-18 - Epic Ridge Race|Epic Ridge Race]] (also [[../TrailRuns/2026-05-18 - Epic Ridge Race|Trail Run]])" in races_sum_content
    assert "[[2026-05-19 - Road 10k Race|Road 10k Race]]" in races_sum_content
    assert "[[../TrailRuns/2026-05-19 - Road 10k Race|Trail Run]]" not in races_sum_content

    # E. Check TrailRuns index cross-links
    with open(trail_summary_path, 'r', encoding='utf-8') as f:
        trail_sum_content = f.read()
    assert "[[2026-05-18 - Epic Ridge Race|Epic Ridge Race]] (also [[../Races/2026-05-18 - Epic Ridge Race|Race]])" in trail_sum_content
    assert "[[2026-05-20 - Mount Wilson Climb|Mount Wilson Climb]]" in trail_sum_content
    assert "[[../Races/2026-05-20 - Mount Wilson Climb|Race]]" not in trail_sum_content


def test_write_weekly_weather_note(monkeypatch, tmp_path):
    vault_dir = tmp_path / "vault"
    obsidian_folder = "Weekly"
    
    # Configuration
    config = {
        "weather_location": "Chicago, IL",
        "weather_lat": 41.85,
        "weather_lon": -87.65
    }
    
    # 7 Days dates
    dates = ["2026-05-30", "2026-05-31", "2026-06-01", "2026-06-02", "2026-06-03", "2026-06-04", "2026-06-05"]
    hourly_times = []
    for d in dates:
        for h in range(24):
            hourly_times.append(f"{d}T{h:02d}:00")
            
    # Mock weather data response
    weather_data = {
        "daily": {
            "time": dates,
            "weather_code": [0] * 7,
            "temperature_2m_max": [70.0] * 7,
            "temperature_2m_min": [50.0] * 7,
            "sunrise": [f"{d}T05:22" for d in dates],
            "sunset": [f"{d}T20:10" for d in dates],
            "wind_speed_10m_max": [10.0] * 7
        },
        "hourly": {
            "time": hourly_times,
            "temperature_2m": [60.0] * len(hourly_times),
            "apparent_temperature": [58.0] * len(hourly_times),
            "relative_humidity_2m": [50] * len(hourly_times),
            "wind_speed_10m": [8.0] * len(hourly_times),
            "wind_direction_10m": [180] * len(hourly_times),
            "precipitation_probability": [0] * len(hourly_times),
            "weather_code": [0] * len(hourly_times)
        }
    }
    
    monkeypatch.setattr("running_cli.retrieval.weather.fetch_weather", lambda *args, **kwargs: weather_data)
    
    # Execute write
    filepath = obsidian.write_weekly_weather_note(
        vault_path=str(vault_dir),
        folder=obsidian_folder,
        config=config,
        unit="miles"
    )
    
    assert os.path.exists(filepath)
    assert os.path.basename(filepath) == "WeeklyWeather.md"
    
    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()
        
    assert "type: weather-summary" in content
    assert "location: Chicago, IL" in content
    assert "latitude: 41.85" in content
    assert "longitude: -87.65" in content
    assert "Weekly Weather & Running Exertion" in content
    
    # Outlook table check
    assert "Date | Condition | Temp Range | Max Wind | Avg Hum | Sun Times | Best Run Window" in content
    assert "Sat, May 30 | ☀️ Clear | 50°F - 70°F | 10 mph | 50% | 5:22AM - 8:10PM" in content
    
    # Hourly tables check
    assert "### Sat, May 30" in content
    assert "| 5:00 AM | ☀️ Clear | 60°F (58°F) | 50% | 8 mph S | 0% | **Ideal** (Score: 0.0) |" in content




