from datetime import timedelta
from typing import List, Dict, Any, Tuple
from running_cli.core.models import Activity, MergedActivity

def merge_activities(
    garmin_raw: List[Dict[str, Any]], 
    strava_raw: List[Dict[str, Any]]
) -> List[MergedActivity]:
    """Merge Garmin and Strava raw activities, deduplicating overlaps and prioritizing Strava titles."""
    garmin_activities: List[Activity] = []
    for g in garmin_raw:
        try:
            garmin_activities.append(Activity.from_garmin(g))
        except Exception:
            # Silently skip parse errors or log them (for robustness)
            continue

    strava_activities: List[Activity] = []
    for s in strava_raw:
        try:
            strava_activities.append(Activity.from_strava(s))
        except Exception:
            continue

    merged_results: List[MergedActivity] = []
    matched_strava_ids = set()

    for g_act in garmin_activities:
        # Try to find a matching Strava activity
        match_s_act = None
        for s_act in strava_activities:
            if s_act.id in matched_strava_ids:
                continue
            
            # Check matching criteria:
            # 1. Sport is similar or same
            if g_act.sport != s_act.sport:
                # Allow minor mismatched names if times are identical, but generally must match
                if g_act.sport == "Other" or s_act.sport == "Other":
                    pass
                else:
                    continue
            
            # 2. Start time within 10 minutes
            time_diff = abs((g_act.start_time - s_act.start_time).total_seconds())
            if time_diff > 600:  # 10 minutes
                continue
            
            # 3. Distance within 10% or within 500 meters
            dist_diff = abs(g_act.distance_meters - s_act.distance_meters)
            avg_dist = (g_act.distance_meters + s_act.distance_meters) / 2.0
            
            is_dist_match = False
            if avg_dist > 0:
                is_dist_match = (dist_diff / avg_dist) <= 0.10
            else:
                is_dist_match = dist_diff <= 500
            
            if is_dist_match or dist_diff <= 500:
                match_s_act = s_act
                break

        if match_s_act:
            matched_strava_ids.add(match_s_act.id)
            # Create a merged activity
            # Decision: Prefer Strava title over Garmin auto-generated title
            title = match_s_act.title if match_s_act.title else g_act.title
            
            description = match_s_act.description if match_s_act.description else g_act.description
            location_name = g_act.location_name if g_act.location_name else match_s_act.location_name
            latitude = g_act.latitude if g_act.latitude is not None else match_s_act.latitude
            longitude = g_act.longitude if g_act.longitude is not None else match_s_act.longitude
            merged_results.append(
                MergedActivity(
                    date=g_act.start_time.date(),
                    start_time=g_act.start_time.time(),
                    sport=g_act.sport,
                    title=title,
                    distance_meters=g_act.distance_meters,  # Prefer Garmin raw device GPS distance
                    duration_seconds=g_act.duration_seconds,  # Prefer Garmin device elapsed time
                    avg_hr=g_act.avg_hr if g_act.avg_hr is not None else match_s_act.avg_hr,
                    max_hr=g_act.max_hr if g_act.max_hr is not None else match_s_act.max_hr,
                    elevation_gain_meters=g_act.elevation_gain_meters,
                    garmin_id=g_act.id,
                    strava_id=match_s_act.id,
                    description=description,
                    location_name=location_name,
                    latitude=latitude,
                    longitude=longitude,
                    is_race=g_act.is_race or match_s_act.is_race,
                    sources=["garmin", "strava"]
                )
            )
        else:
            # Garmin-only activity
            merged_results.append(
                MergedActivity(
                    date=g_act.start_time.date(),
                    start_time=g_act.start_time.time(),
                    sport=g_act.sport,
                    title=g_act.title,
                    distance_meters=g_act.distance_meters,
                    duration_seconds=g_act.duration_seconds,
                    avg_hr=g_act.avg_hr,
                    max_hr=g_act.max_hr,
                    elevation_gain_meters=g_act.elevation_gain_meters,
                    garmin_id=g_act.id,
                    strava_id=None,
                    description=g_act.description,
                    location_name=g_act.location_name,
                    latitude=g_act.latitude,
                    longitude=g_act.longitude,
                    is_race=g_act.is_race,
                    sources=["garmin"]
                )
            )

    # Process remaining Strava activities (Strava-only)
    for s_act in strava_activities:
        if s_act.id in matched_strava_ids:
            continue
        
        merged_results.append(
            MergedActivity(
                date=s_act.start_time.date(),
                start_time=s_act.start_time.time(),
                sport=s_act.sport,
                title=s_act.title,
                distance_meters=s_act.distance_meters,
                duration_seconds=s_act.duration_seconds,
                avg_hr=s_act.avg_hr,
                max_hr=s_act.max_hr,
                elevation_gain_meters=s_act.elevation_gain_meters,
                garmin_id=None,
                strava_id=s_act.id,
                description=s_act.description,
                location_name=s_act.location_name,
                latitude=s_act.latitude,
                longitude=s_act.longitude,
                is_race=s_act.is_race,
                sources=["strava"]
            )
        )

    # Sort merged activities chronologically
    merged_results.sort(key=lambda x: x.datetime)
    return merged_results
