import os
import json
import requests
from datetime import date, datetime, time
from typing import List, Dict, Any, Optional
from running_cli.config import load_config, save_config

CACHE_DIR = os.path.expanduser("~/.running_cli/cache/strava")

def refresh_strava_token(client_id: str, client_secret: str, refresh_token: str) -> str:
    """Refresh the Strava OAuth2 access token and update the config with the new refresh token if changed."""
    url = "https://www.strava.com/oauth/token"
    payload = {
        "client_id": client_id,
        "client_secret": client_secret,
        "grant_type": "refresh_token",
        "refresh_token": refresh_token
    }
    
    response = requests.post(url, data=payload)
    response.raise_for_status()
    data = response.json()
    
    new_access_token = data["access_token"]
    new_refresh_token = data.get("refresh_token")
    
    # If the refresh token was updated, save it back to our configuration
    if new_refresh_token and new_refresh_token != refresh_token:
        config = load_config()
        config["strava"]["refresh_token"] = new_refresh_token
        save_config(config)
        
    return new_access_token

def fetch_and_cache_strava_activities(
    client_id: str,
    client_secret: str,
    refresh_token: str,
    start_date: date,
    end_date: date
) -> List[Dict[str, Any]]:
    """Fetch activities from Strava API for the date range and cache them locally."""
    os.makedirs(CACHE_DIR, exist_ok=True)
    
    access_token = refresh_strava_token(client_id, client_secret, refresh_token)
    
    # Strava parameters for filtering by time
    # 'after' corresponds to start of start_date
    after_epoch = int(datetime.combine(start_date, time.min).timestamp())
    # 'before' corresponds to end of end_date
    before_epoch = int(datetime.combine(end_date, time.max).timestamp())
    
    url = "https://www.strava.com/api/v3/athlete/activities"
    headers = {"Authorization": f"Bearer {access_token}"}
    
    all_activities = []
    page = 1
    
    while True:
        params = {
            "before": before_epoch,
            "after": after_epoch,
            "page": page,
            "per_page": 100
        }
        
        response = requests.get(url, headers=headers, params=params)
        response.raise_for_status()
        activities = response.json()
        
        if not activities:
            break
            
        all_activities.extend(activities)
        
        # If we got less than 100, we've reached the end
        if len(activities) < 100:
            break
            
        page += 1
        
    # Cache each activity to disk
    for activity in all_activities:
        activity_id = activity.get("id")
        if activity_id is None:
            continue
            
        cache_file = os.path.join(CACHE_DIR, f"{activity_id}.json")
        with open(cache_file, "w", encoding="utf-8") as f:
            json.dump(activity, f, indent=2)
            
    return all_activities

def load_cached_strava_activities(
    start_date: Optional[date] = None,
    end_date: Optional[date] = None
) -> List[Dict[str, Any]]:
    """Load all cached Strava activities, optionally filtering by date range."""
    activities = []
    if not os.path.exists(CACHE_DIR):
        return activities
        
    for filename in os.listdir(CACHE_DIR):
        if not filename.endswith(".json"):
            continue
            
        filepath = os.path.join(CACHE_DIR, filename)
        try:
            with open(filepath, "r", encoding="utf-8") as f:
                data = json.load(f)
                
            # Filter by date range if specified (e.g. "2026-05-22T07:16:33Z")
            start_time_str = data.get("start_date_local", "")
            if start_time_str and (start_date or end_date):
                act_date = date.fromisoformat(start_time_str[:10])
                if start_date and act_date < start_date:
                    continue
                if end_date and act_date > end_date:
                    continue
                    
            activities.append(data)
        except Exception:
            continue
            
    # Sort chronologically by start_date_local
    activities.sort(key=lambda x: x.get("start_date_local", ""))
    return activities
