from datetime import datetime, date, time
from running_cli.core.merger import merge_activities
from running_cli.core.models import Activity, MergedActivity

def test_merge_garmin_only():
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Boston Running",
        "activityType": {"typeKey": "running"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10000.0, # 10 km
        "duration": 3000.0, # 50 mins
        "averageHR": 150.0,
        "maxHR": 170.0,
        "elevationGain": 150.0
    }]
    strava_raw = []
    
    merged = merge_activities(garmin_raw, strava_raw)
    assert len(merged) == 1
    m = merged[0]
    assert m.garmin_id == 1111
    assert m.strava_id is None
    assert m.title == "Boston Running"
    assert m.sport == "Run"
    assert m.distance_meters == 10000.0
    assert m.duration_seconds == 3000.0
    assert m.avg_hr == 150.0
    assert m.sources == ["garmin"]

def test_merge_strava_only():
    garmin_raw = []
    strava_raw = [{
        "id": 2222,
        "name": "Morning Ride",
        "sport_type": "Ride",
        "start_date_local": "2026-05-22T09:00:00Z",
        "distance": 25000.0,
        "moving_time": 3600.0,
        "average_heartrate": 130.0,
        "total_elevation_gain": 200.0
    }]
    
    merged = merge_activities(garmin_raw, strava_raw)
    assert len(merged) == 1
    m = merged[0]
    assert m.garmin_id is None
    assert m.strava_id == 2222
    assert m.title == "Morning Ride"
    assert m.sport == "Ride"
    assert m.distance_meters == 25000.0
    assert m.duration_seconds == 3600.0
    assert m.avg_hr == 130.0
    assert m.sources == ["strava"]

def test_merge_overlapping_activities():
    # Activity recorded on both:
    # Garmin start: 08:00:00. Strava start: 08:02:00 (within 10 mins)
    # Garmin distance: 10050m. Strava distance: 10010m (similar distance)
    # Prefer Strava title over Garmin title. Prefer Garmin distance/duration.
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Boston Running",
        "activityType": {"typeKey": "running"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10050.0,
        "duration": 3000.0,
        "averageHR": 150.0,
        "maxHR": 170.0,
        "elevationGain": 120.0
    }]
    strava_raw = [{
        "id": 2222,
        "name": "Boston 10k race (PR!)",
        "type": "Run",
        "start_date_local": "2026-05-22T08:02:00Z",
        "distance": 10010.0,
        "moving_time": 2990.0,
        "average_heartrate": 148.0,
        "max_heartrate": 168.0,
        "total_elevation_gain": 118.0
    }]
    
    merged = merge_activities(garmin_raw, strava_raw)
    assert len(merged) == 1
    m = merged[0]
    assert m.garmin_id == 1111
    assert m.strava_id == 2222
    assert m.title == "Boston 10k race (PR!)" # Preferred Strava title!
    assert m.sport == "Run"
    assert m.distance_meters == 10050.0 # Garmin distance preferred
    assert m.duration_seconds == 3000.0 # Garmin duration preferred
    assert m.avg_hr == 150.0
    assert m.max_hr == 170.0
    assert m.sources == ["garmin", "strava"]

def test_no_merge_different_times():
    # Same sport, but start times differ by 30 mins
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Morning Run",
        "activityType": {"typeKey": "running"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 5000.0,
        "duration": 1500.0
    }]
    strava_raw = [{
        "id": 2222,
        "name": "Lunch Run",
        "type": "Run",
        "start_date_local": "2026-05-22T08:35:00Z",
        "distance": 5000.0,
        "moving_time": 1500.0
    }]
    
    merged = merge_activities(garmin_raw, strava_raw)
    assert len(merged) == 2
    assert merged[0].garmin_id == 1111
    assert merged[1].strava_id == 2222

def test_no_merge_different_distances():
    # Same start time, but distances are 5k vs 20k
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Short Run",
        "activityType": {"typeKey": "running"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 5000.0,
        "duration": 1500.0
    }]
    strava_raw = [{
        "id": 2222,
        "name": "Long Run",
        "type": "Run",
        "start_date_local": "2026-05-22T08:00:00Z",
        "distance": 20000.0,
        "moving_time": 6000.0
    }]
    
    merged = merge_activities(garmin_raw, strava_raw)
    assert len(merged) == 2

def test_merge_race_activities():
    # 1. Garmin-only race
    garmin_raw = [{
        "activityId": 1111,
        "activityName": "Garmin Race",
        "activityType": {"typeKey": "running"},
        "eventType": {"typeKey": "race"},
        "startTimeLocal": "2026-05-22 08:00:00",
        "distance": 10000.0,
        "duration": 3000.0
    }]
    
    # 2. Strava-only race
    strava_raw = [{
        "id": 2222,
        "name": "Strava Race",
        "type": "Run",
        "workout_type": 1,
        "start_date_local": "2026-05-23T08:00:00Z",
        "distance": 10000.0,
        "moving_time": 3000.0
    }]
    
    merged = merge_activities(garmin_raw, strava_raw)
    assert len(merged) == 2
    assert merged[0].is_race is True
    assert merged[0].title == "Garmin Race"
    assert merged[1].is_race is True
    assert merged[1].title == "Strava Race"
    
    # 3. Merged race (Garmin uncategorized + Strava race)
    garmin_raw_2 = [{
        "activityId": 3333,
        "activityName": "Garmin Activity",
        "activityType": {"typeKey": "running"},
        "eventType": {"typeKey": "uncategorized"},
        "startTimeLocal": "2026-05-24 08:00:00",
        "distance": 10000.0,
        "duration": 3000.0
    }]
    strava_raw_2 = [{
        "id": 4444,
        "name": "Strava Race Activity",
        "type": "Run",
        "workout_type": 1,
        "start_date_local": "2026-05-24T08:01:00Z",
        "distance": 10000.0,
        "moving_time": 3000.0
    }]
    merged_2 = merge_activities(garmin_raw_2, strava_raw_2)
    assert len(merged_2) == 1
    assert merged_2[0].is_race is True
    assert merged_2[0].title == "Strava Race Activity"

