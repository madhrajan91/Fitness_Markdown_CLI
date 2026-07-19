import os
import json
from datetime import date, datetime
from typing import List, Dict, Any, Optional
from garminconnect import Garmin

CACHE_DIR = os.path.expanduser("~/.running_cli/cache/garmin")
TOKENSTORE = os.path.expanduser("~/.running_cli/garmin_session")

def get_garmin_client(email: Optional[str] = None, password: Optional[str] = None) -> Garmin:
    """Initialize Garmin client with session tokenstore. 
    If tokenstore is valid, email and password are not required.
    """
    os.makedirs(TOKENSTORE, exist_ok=True)
    
    # Try using the existing tokenstore first
    client = Garmin()
    try:
        # Check if we have files in tokenstore
        if os.listdir(TOKENSTORE):
            client.login(tokenstore=TOKENSTORE)
            return client
    except Exception:
        pass
        
    # If tokenstore login failed or was empty, perform fresh credentials login
    if email and password:
        def prompt_mfa():
            return input("\nGarmin Multi-Factor Authentication Code: ").strip()

        client = Garmin(
            email=email,
            password=password,
            prompt_mfa=prompt_mfa
        )
        client.login(tokenstore=TOKENSTORE)
        return client
    else:
        raise RuntimeError("Garmin session expired or tokenstore not initialized. Please run 'running-cli setup' to login.")

def fetch_and_cache_garmin_activities(
    start_date: date, 
    end_date: date,
    email: Optional[str] = None,
    password: Optional[str] = None
) -> List[Dict[str, Any]]:
    """Fetch activities from Garmin Connect API and save them to the local cache directory."""
    os.makedirs(CACHE_DIR, exist_ok=True)
    
    client = get_garmin_client(email, password)
    
    # Fetch from Garmin API. Date strings must be in ISO format: YYYY-MM-DD
    activities = client.get_activities_by_date(
        start_date.isoformat(), 
        end_date.isoformat()
    )
    
    saved_activities = []
    for activity in activities:
        activity_id = activity.get("activityId")
        if activity_id is None:
            continue
            
        # Cache file path
        cache_file = os.path.join(CACHE_DIR, f"{activity_id}.json")
        with open(cache_file, "w", encoding="utf-8") as f:
            json.dump(activity, f, indent=2)
            
        saved_activities.append(activity)
        
    return saved_activities

def load_cached_garmin_activities(
    start_date: Optional[date] = None, 
    end_date: Optional[date] = None
) -> List[Dict[str, Any]]:
    """Load all cached Garmin activities, optionally filtering by date range."""
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
                
            # Filter by date range if specified
            start_time_str = data.get("startTimeLocal", "")
            if start_time_str and (start_date or end_date):
                # Extract YYYY-MM-DD
                act_date = date.fromisoformat(start_time_str[:10])
                if start_date and act_date < start_date:
                    continue
                if end_date and act_date > end_date:
                    continue
                    
            activities.append(data)
        except Exception:
            # Skip corrupted cache files
            continue
            
    # Sort chronologically by startTimeLocal
    activities.sort(key=lambda x: x.get("startTimeLocal", ""))
    return activities
