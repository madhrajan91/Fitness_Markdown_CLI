import re
from datetime import datetime, date, time
from typing import Optional, List, Dict, Any

class Activity:
    """Represents a normalized activity from a specific provider (Garmin or Strava)."""
    def __init__(
        self,
        id: int,
        provider: str,  # 'garmin' or 'strava'
        sport: str,     # 'Run', 'Ride', 'Swim', 'Walk', 'Hike', 'Other'
        title: str,
        start_time: datetime,
        distance_meters: float,
        duration_seconds: float,
        avg_hr: Optional[float] = None,
        max_hr: Optional[float] = None,
        elevation_gain_meters: float = 0.0,
        description: Optional[str] = None,
        location_name: Optional[str] = None,
        latitude: Optional[float] = None,
        longitude: Optional[float] = None,
        is_race: bool = False,
        raw_data: Optional[Dict[str, Any]] = None
    ):
        self.id = id
        self.provider = provider
        self.sport = sport
        self.title = title
        self.start_time = start_time
        self.distance_meters = distance_meters
        self.duration_seconds = duration_seconds
        self.avg_hr = avg_hr
        self.max_hr = max_hr
        self.elevation_gain_meters = elevation_gain_meters
        self.description = description
        self.location_name = location_name
        self.latitude = latitude
        self.longitude = longitude
        self.is_race = is_race
        self.raw_data = raw_data or {}

    @classmethod
    def from_garmin(cls, data: Dict[str, Any]) -> "Activity":
        activity_id = data.get("activityId")
        if activity_id is None:
            raise ValueError("Garmin activity missing activityId")
            
        title = data.get("activityName") or "Garmin Activity"
        
        # Parse sport type
        type_key = ""
        activity_type = data.get("activityType")
        if isinstance(activity_type, dict):
            type_key = activity_type.get("typeKey", "").lower()
        
        sport = "Other"
        if "running" in type_key or "run" in type_key:
            sport = "Run"
        elif "cycling" in type_key or "biking" in type_key or "ride" in type_key:
            sport = "Ride"
        elif "swimming" in type_key or "swim" in type_key:
            sport = "Swim"
        elif "walking" in type_key or "walk" in type_key:
            sport = "Walk"
        elif "hiking" in type_key or "hike" in type_key:
            sport = "Hike"
        else:
            sport = type_key.capitalize() if type_key else "Other"

        # Parse start time
        start_time_str = data.get("startTimeLocal", "")
        start_time = cls._parse_time(start_time_str, "Garmin")

        # Distance is typically in meters in the Garmin API response
        distance = float(data.get("distance", 0.0))
        duration = float(data.get("duration", 0.0))
        
        avg_hr = data.get("averageHR")
        if avg_hr is not None:
            avg_hr = float(avg_hr)
        max_hr = data.get("maxHR")
        if max_hr is not None:
            max_hr = float(max_hr)

        elevation_gain = float(data.get("elevationGain", 0.0))
        description = data.get("description")
        location_name = data.get("locationName")
        latitude = data.get("startLatitude")
        longitude = data.get("startLongitude")

        # Parse event type for race status
        event_type = data.get("eventType")
        is_race = False
        if isinstance(event_type, dict):
            is_race = event_type.get("typeKey", "").lower() == "race"

        return cls(
            id=activity_id,
            provider="garmin",
            sport=sport,
            title=title,
            start_time=start_time,
            distance_meters=distance,
            duration_seconds=duration,
            avg_hr=avg_hr,
            max_hr=max_hr,
            elevation_gain_meters=elevation_gain,
            description=description,
            location_name=location_name,
            latitude=latitude,
            longitude=longitude,
            is_race=is_race,
            raw_data=data
        )

    @classmethod
    def from_strava(cls, data: Dict[str, Any]) -> "Activity":
        activity_id = data.get("id")
        if activity_id is None:
            raise ValueError("Strava activity missing id")
            
        title = data.get("name") or "Strava Activity"
        
        # Parse sport type
        sport_type = (data.get("sport_type") or data.get("type") or "").lower()
        sport = "Other"
        if "run" in sport_type:
            sport = "Run"
        elif "ride" in sport_type or "cycling" in sport_type:
            sport = "Ride"
        elif "swim" in sport_type:
            sport = "Swim"
        elif "walk" in sport_type:
            sport = "Walk"
        elif "hike" in sport_type:
            sport = "Hike"
        else:
            sport = sport_type.capitalize() if sport_type else "Other"

        # Parse start time (e.g. "2026-05-22T07:16:33Z")
        start_time_str = data.get("start_date_local", "")
        start_time = cls._parse_time(start_time_str, "Strava")

        # Strava uses SI units: distance in meters, duration in seconds
        distance = float(data.get("distance", 0.0))
        # Use moving time or elapsed time depending on availability
        duration = float(data.get("moving_time") or data.get("elapsed_time") or 0.0)
        
        avg_hr = data.get("average_heartrate")
        if avg_hr is not None:
            avg_hr = float(avg_hr)
        max_hr = data.get("max_heartrate")
        if max_hr is not None:
            max_hr = float(max_hr)

        elevation_gain = float(data.get("total_elevation_gain", 0.0))
        description = data.get("description")
        
        location_name = None
        city = data.get("location_city")
        state = data.get("location_state")
        if city and state:
            location_name = f"{city}, {state}"
        elif city:
            location_name = city
        elif state:
            location_name = state

        latitude = None
        longitude = None
        start_latlng = data.get("start_latlng")
        if isinstance(start_latlng, list) and len(start_latlng) >= 2:
            latitude = float(start_latlng[0])
            longitude = float(start_latlng[1])

        # Parse workout type for race status
        workout_type = data.get("workout_type")
        is_race = (workout_type == 1)

        return cls(
            id=activity_id,
            provider="strava",
            sport=sport,
            title=title,
            start_time=start_time,
            distance_meters=distance,
            duration_seconds=duration,
            avg_hr=avg_hr,
            max_hr=max_hr,
            elevation_gain_meters=elevation_gain,
            description=description,
            location_name=location_name,
            latitude=latitude,
            longitude=longitude,
            is_race=is_race,
            raw_data=data
        )

    @staticmethod
    def _parse_time(time_str: str, provider_name: str) -> datetime:
        if not time_str:
            return datetime.now()
        for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%dT%H:%M:%SZ", "%Y-%m-%dT%H:%M:%S.%f"):
            try:
                return datetime.strptime(time_str[:19], fmt[:19] if "." not in time_str else fmt)
            except ValueError:
                continue
        # Fallback regex
        m = re.match(r"(\d{4})-(\d{2})-(\d{2})[T ](\d{2}):(\d{2}):(\d{2})", time_str)
        if m:
            return datetime(int(m[1]), int(m[2]), int(m[3]), int(m[4]), int(m[5]), int(m[6]))
        raise ValueError(f"Could not parse {provider_name} time: {time_str}")


class MergedActivity:
    """Represents a unified activity that can combine Garmin and Strava source activities."""
    def __init__(
        self,
        date: date,
        start_time: time,
        sport: str,
        title: str,
        distance_meters: float,
        duration_seconds: float,
        avg_hr: Optional[float] = None,
        max_hr: Optional[float] = None,
        elevation_gain_meters: float = 0.0,
        garmin_id: Optional[int] = None,
        strava_id: Optional[int] = None,
        description: Optional[str] = None,
        location_name: Optional[str] = None,
        latitude: Optional[float] = None,
        longitude: Optional[float] = None,
        is_race: bool = False,
        sources: Optional[List[str]] = None
    ):
        self.date = date
        self.start_time = start_time
        self.sport = sport
        self.title = title
        self.distance_meters = distance_meters
        self.duration_seconds = duration_seconds
        self.avg_hr = avg_hr
        self.max_hr = max_hr
        self.elevation_gain_meters = elevation_gain_meters
        self.garmin_id = garmin_id
        self.strava_id = strava_id
        self.description = description
        self.location_name = location_name
        self.latitude = latitude
        self.longitude = longitude
        self.is_race = is_race
        self.sources = sources or []

    @property
    def datetime(self) -> datetime:
        return datetime.combine(self.date, self.start_time)
