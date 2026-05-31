import os
import re
import yaml
from datetime import date, timedelta
from typing import List, Dict, Any
from running_cli.core.models import MergedActivity

SPORT_EMOJIS = {
    "Run": "🏃",
    "Ride": "🚴",
    "Swim": "🏊",
    "Walk": "🚶",
    "Hike": "🥾",
    "Other": "🏋️"
}

def get_monday_of_week(d: date) -> date:
    """Return the Monday of the week for the given date."""
    return d - timedelta(days=d.weekday())

def format_duration(seconds: float) -> str:
    """Format duration into a friendly string like '2h 14m' or '45m 12s'."""
    hrs = int(seconds // 3600)
    mins = int((seconds % 3600) // 60)
    secs = int(seconds % 60)
    if hrs > 0:
        return f"{hrs}h {mins:02d}m"
    return f"{mins}m {secs:02d}s"

def format_duration_compact(seconds: float) -> str:
    """Format duration into compact 'H:MM:SS' or 'MM:SS'."""
    hrs = int(seconds // 3600)
    mins = int((seconds % 3600) // 60)
    secs = int(seconds % 60)
    if hrs > 0:
        return f"{hrs}:{mins:02d}:{secs:02d}"
    return f"{mins:02d}:{secs:02d}"

def format_distance(sport: str, meters: float, unit: str) -> str:
    """Format distance according to sport type and preference unit."""
    sport_lower = sport.lower()
    if sport_lower == "swim":
        if unit == "miles":
            # Display yards
            yards = meters * 1.09361
            return f"{int(round(yards))} yd"
        else:
            return f"{int(round(meters))} m"
    else:
        if unit == "miles":
            miles = meters * 0.000621371
            return f"{miles:.2f} mi"
        else:
            km = meters * 0.001
            return f"{km:.2f} km"

def format_pace_or_speed(sport: str, distance_meters: float, duration_seconds: float, unit: str) -> str:
    """Calculate and format pace or speed depending on sport type."""
    if distance_meters <= 0 or duration_seconds <= 0:
        return "-"
        
    sport_lower = sport.lower()
    
    if sport_lower in ("run", "walk", "hike"):
        if unit == "miles":
            dist = distance_meters * 0.000621371
            label = "/mi"
        else:
            dist = distance_meters * 0.001
            label = "/km"
            
        if dist <= 0:
            return "-"
        
        total_minutes = duration_seconds / 60.0
        pace_dec = total_minutes / dist
        pace_min = int(pace_dec)
        pace_sec = int((pace_dec - pace_min) * 60)
        return f"{pace_min}:{pace_sec:02d}{label}"
        
    elif sport_lower in ("ride", "cycling"):
        if unit == "miles":
            dist = distance_meters * 0.000621371
            label = " mph"
        else:
            dist = distance_meters * 0.001
            label = " km/h"
            
        hours = duration_seconds / 3600.0
        if hours <= 0:
            return "-"
        speed = dist / hours
        return f"{speed:.1f}{label}"
        
    elif sport_lower == "swim":
        if unit == "miles":
            yards = distance_meters * 1.09361
            if yards <= 0:
                return "-"
            hundred_yards = yards / 100.0
            total_minutes = duration_seconds / 60.0
            pace_dec = total_minutes / hundred_yards
            pace_min = int(pace_dec)
            pace_sec = int((pace_dec - pace_min) * 60)
            return f"{pace_min}:{pace_sec:02d}/100yd"
        else:
            if distance_meters <= 0:
                return "-"
            hundred_meters = distance_meters / 100.0
            total_minutes = duration_seconds / 60.0
            pace_dec = total_minutes / hundred_meters
            pace_min = int(pace_dec)
            pace_sec = int((pace_dec - pace_min) * 60)
            return f"{pace_min}:{pace_sec:02d}/100m"
            
    else:
        if unit == "miles":
            dist = distance_meters * 0.000621371
            label = " mph"
        else:
            dist = distance_meters * 0.001
            label = " km/h"
        hours = duration_seconds / 3600.0
        if hours <= 0:
            return "-"
        speed = dist / hours
        return f"{speed:.1f}{label}"

def format_elevation(meters: float, unit: str) -> str:
    """Format elevation gain."""
    if meters <= 0:
        return "-"
    if unit == "miles":
        feet = meters * 3.28084
        return f"+{int(round(feet))} ft"
    else:
        return f"+{int(round(meters))} m"

def safe_update_markdown(
    file_content: str,
    new_frontmatter: Dict[str, Any],
    new_summary: str,
    start_tag: str = "%% START_WEEKLY_SUMMARY %%",
    end_tag: str = "%% END_WEEKLY_SUMMARY %%",
    missing_tags_template: str = "\n# Weekly Summary\n\n{replacement}\n\n{body}"
) -> str:
    """Update YAML frontmatter and summary section inside comments, leaving rest intact."""
    # Find frontmatter
    frontmatter_match = re.match(r"^---\s*\n(.*?)\n---\s*\n(.*)$", file_content, re.DOTALL | re.MULTILINE)
    
    existing_yaml = {}
    body = file_content
    
    if frontmatter_match:
        yaml_content = frontmatter_match.group(1)
        body = frontmatter_match.group(2)
        try:
            existing_yaml = yaml.safe_load(yaml_content) or {}
        except Exception:
            existing_yaml = {}
            
    # Merge frontmatter keys
    for k, v in new_frontmatter.items():
        existing_yaml[k] = v
        
    try:
        updated_yaml_str = yaml.safe_dump(existing_yaml, sort_keys=False).strip()
    except Exception:
        # Fallback manual string representation
        updated_yaml_str = "\n".join(f"{k}: {v}" for k, v in existing_yaml.items())
        
    pattern = re.compile(rf"{re.escape(start_tag)}.*?{re.escape(end_tag)}", re.DOTALL)
    replacement = f"{start_tag}\n\n{new_summary}\n\n{end_tag}"
    
    if pattern.search(body):
        updated_body = pattern.sub(replacement, body)
    else:
        # Append tags to top if missing
        updated_body = missing_tags_template.format(replacement=replacement, body=body.lstrip())
        
    return f"---\n{updated_yaml_str}\n---\n\n{updated_body.lstrip()}"

def write_weekly_note(
    vault_path: str,
    folder: str,
    monday_date: date,
    activities: List[MergedActivity],
    unit: str
) -> str:
    """Generate or update weekly Markdown note in Obsidian vault."""
    # Sort activities chronologically
    activities = sorted(activities, key=lambda a: a.datetime)
    
    # 1. Compute summary stats
    totals_by_sport = {}
    total_duration = 0.0
    sources_set = set()
    
    for a in activities:
        total_duration += a.duration_seconds
        sources_set.update(a.sources)
        
        sport = a.sport
        if sport not in totals_by_sport:
            totals_by_sport[sport] = {"distance": 0.0, "duration": 0.0, "count": 0}
        totals_by_sport[sport]["distance"] += a.distance_meters
        totals_by_sport[sport]["duration"] += a.duration_seconds
        totals_by_sport[sport]["count"] += 1

    # 2. Build new frontmatter
    # Format total distances for frontmatter (as readable floats)
    frontmatter_distances = {}
    for sport, stats in totals_by_sport.items():
        if sport.lower() == "swim":
            # yards or meters
            val = stats["distance"] * (1.09361 if unit == "miles" else 1.0)
        else:
            val = stats["distance"] * (0.000621371 if unit == "miles" else 0.001)
        frontmatter_distances[sport.lower()] = round(val, 2)

    new_frontmatter = {
        "type": "weekly-activities",
        "week_of": monday_date.isoformat(),
        "activity_count": len(activities),
        "total_duration_seconds": int(total_duration),
        "total_duration_friendly": format_duration(total_duration),
        "distances": frontmatter_distances,
        "distance_unit": unit,
        "sources": sorted(list(sources_set))
    }
    
    # 3. Build Markdown Summary Block
    breakdown_lines = []
    for sport, stats in sorted(totals_by_sport.items()):
        emoji = SPORT_EMOJIS.get(sport, SPORT_EMOJIS["Other"])
        dist_str = format_distance(sport, stats["distance"], unit)
        dur_str = format_duration(stats["duration"])
        cnt = stats["count"]
        act_label = "activity" if cnt == 1 else "activities"
        breakdown_lines.append(f"- **{emoji} {sport}**: {dist_str} | {dur_str} | {cnt} {act_label}")
        
    breakdown_section = "\n".join(breakdown_lines)
    
    # Build Table
    table_headers = [
        "Date", "Sport", "Title", "Distance", "Time", "Pace/Speed", "HR (Avg/Max)", "Elev", "Location", "Source"
    ]
    table_rows = [
        "| " + " | ".join(table_headers) + " |",
        "| " + " | ".join(["---"] * len(table_headers)) + " |"
    ]
    
    for a in activities:
        date_str = a.date.strftime("%a, %b %d")
        emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS["Other"])
        sport_str = f"{emoji} {a.sport}"
        dist_str = format_distance(a.sport, a.distance_meters, unit)
        time_str = format_duration_compact(a.duration_seconds)
        pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
        
        hr_str = "-"
        if a.avg_hr is not None:
            max_hr_val = f"/{int(a.max_hr)}" if a.max_hr is not None else ""
            hr_str = f"{int(a.avg_hr)}{max_hr_val}"
            
        elev_str = format_elevation(a.elevation_gain_meters, unit)
        
        loc_str = "-"
        if a.latitude is not None and a.longitude is not None:
            map_url = f"https://www.google.com/maps/search/?api=1&query={a.latitude},{a.longitude}"
            loc_name = a.location_name if a.location_name else "Map"
            loc_str = f"📍 [{loc_name}]({map_url})"

        src_links = []
        if "garmin" in a.sources and a.garmin_id:
            src_links.append(f"[Garmin](https://connect.garmin.com/modern/activity/{a.garmin_id})")
        elif "garmin" in a.sources:
            src_links.append("Garmin")
            
        if "strava" in a.sources and a.strava_id:
            src_links.append(f"[Strava](https://www.strava.com/activities/{a.strava_id})")
        elif "strava" in a.sources:
            src_links.append("Strava")
            
        src_str = ", ".join(src_links)
        
        cleaned_t = clean_filename(a.title)
        date_iso = a.date.isoformat()
        is_trail = (a.sport == "Run" and a.elevation_gain_meters >= 609.6 and "strava" in a.sources)
        
        if a.is_race and is_trail:
            title_str = f"[[Races/{date_iso} - {cleaned_t}|{a.title}]] (also [[TrailRuns/{date_iso} - {cleaned_t}|Trail Run]])"
        elif a.is_race:
            title_str = f"[[Races/{date_iso} - {cleaned_t}|{a.title}]]"
        elif is_trail:
            title_str = f"[[TrailRuns/{date_iso} - {cleaned_t}|{a.title}]]"
        else:
            title_str = a.title
            
        row_fields = [
            date_str, sport_str, title_str, dist_str, time_str, pace_str, hr_str, elev_str, loc_str, src_str
        ]
        table_rows.append("| " + " | ".join(row_fields) + " |")
        
    table_section = "\n".join(table_rows)
    
    # Build Collapsible Notes Section (if any activity has a description)
    notes_section = ""
    activities_with_notes = [a for a in activities if a.description and a.description.strip()]
    if activities_with_notes:
        notes_lines = ["", "### Activity Notes"]
        for a in activities_with_notes:
            date_str = a.date.strftime("%a, %b %d")
            emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS["Other"])
            notes_lines.append("<details>")
            notes_lines.append(f"<summary><b>{date_str} - {emoji} {a.title}</b></summary>")
            notes_lines.append("")
            notes_lines.append(a.description.strip())
            notes_lines.append("</details>")
        notes_section = "\n".join(notes_lines)
    
    new_summary = f"### Weekly Breakdown\n{breakdown_section}\n\n### Activity Log\n{table_section}{notes_section}"
    
    # 4. Check if file exists, read it, and perform safe update
    dest_dir = os.path.join(vault_path, folder)
    os.makedirs(dest_dir, exist_ok=True)
    
    filename = f"Week of {monday_date.isoformat()}.md"
    file_path = os.path.join(dest_dir, filename)
    
    existing_content = ""
    if os.path.exists(file_path):
        with open(file_path, 'r', encoding='utf-8') as f:
            existing_content = f.read()
            
    updated_content = safe_update_markdown(existing_content, new_frontmatter, new_summary)
    
    with open(file_path, 'w', encoding='utf-8') as f:
        f.write(updated_content)
        
    return file_path

def clean_filename(title: str) -> str:
    """Sanitize the title for filename use."""
    return re.sub(r'[\\/*?:"<>|]', "", title).strip()

def write_race_notes(
    vault_path: str,
    folder: str,
    activities: List[MergedActivity],
    unit: str
) -> List[str]:
    """
    Generate or update individual race notes and the summary.md file under <vault_path>/<folder>/Races/.
    Returns a list of created/updated markdown file paths.
    """
    races = [a for a in activities if a.is_race]
    
    # Sort races chronologically
    races = sorted(races, key=lambda a: a.datetime)

    races_dir = os.path.join(vault_path, folder, "Races")
    os.makedirs(races_dir, exist_ok=True)

    written_paths = []

    # 1. Write/Update individual race files
    for a in races:
        filename = f"{a.date.isoformat()} - {clean_filename(a.title)}.md"
        file_path = os.path.join(races_dir, filename)

        # Format stats
        emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", "🏋️"))
        dist_str = format_distance(a.sport, a.distance_meters, unit)
        time_str = format_duration_compact(a.duration_seconds)
        pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
        
        hr_str = "-"
        if a.avg_hr is not None:
            max_hr_val = f"/{int(a.max_hr)}" if a.max_hr is not None else ""
            hr_str = f"{int(a.avg_hr)}{max_hr_val}"
            
        elev_str = format_elevation(a.elevation_gain_meters, unit)
        
        loc_str = "-"
        if a.latitude is not None and a.longitude is not None:
            map_url = f"https://www.google.com/maps/search/?api=1&query={a.latitude},{a.longitude}"
            loc_name = a.location_name if a.location_name else "Map"
            loc_str = f"📍 [{loc_name}]({map_url})"

        src_links = []
        if "garmin" in a.sources and a.garmin_id:
            src_links.append(f"[Garmin](https://connect.garmin.com/modern/activity/{a.garmin_id})")
        elif "garmin" in a.sources:
            src_links.append("Garmin")
            
        if "strava" in a.sources and a.strava_id:
            src_links.append(f"[Strava](https://www.strava.com/activities/{a.strava_id})")
        elif "strava" in a.sources:
            src_links.append("Strava")
            
        src_str = ", ".join(src_links)

        # Build new summary section
        cleaned_t = clean_filename(a.title)
        is_trail = (a.sport == "Run" and a.elevation_gain_meters >= 609.6 and "strava" in a.sources)
        monday_date = get_monday_of_week(a.date)
        
        summary_lines = [
            "### Race Stats",
            f"- **Sport**: {emoji} {a.sport}",
            f"- **Distance**: {dist_str}",
            f"- **Duration**: {time_str}",
            f"- **Pace/Speed**: {pace_str}",
            f"- **Elevation**: {elev_str}",
            f"- **Heart Rate**: {hr_str}",
            f"- **Location**: {loc_str}",
            f"- **Source**: {src_str}",
            f"- **Weekly Log**: [[../Week of {monday_date.isoformat()}|Week of {monday_date.isoformat()}]]",
        ]
        if is_trail:
            summary_lines.append(f"- **Trail Run Note**: [[../TrailRuns/{a.date.isoformat()} - {cleaned_t}|Trail Run Note]]")
        
        if a.description and a.description.strip():
            summary_lines.append("")
            summary_lines.append("### Description")
            summary_lines.append(a.description.strip())
            
        new_summary = "\n".join(summary_lines)

        # Frontmatter
        frontmatter = {
            "type": "race-activity",
            "date": a.date.isoformat(),
            "sport": a.sport,
            "title": a.title,
            "distance": round(a.distance_meters * (0.000621371 if unit == "miles" else 0.001), 2),
            "distance_unit": unit,
            "duration_seconds": int(a.duration_seconds),
            "location": a.location_name,
            "sources": sorted(a.sources)
        }

        existing_content = ""
        if os.path.exists(file_path):
            with open(file_path, 'r', encoding='utf-8') as f:
                existing_content = f.read()

        missing_template = (
            f"\n# {a.title}\n\n"
            f"{{replacement}}\n\n"
            f"## Personal Notes\n"
            f"*Write your personal race report or notes here...*\n\n"
            f"{{body}}"
        )
        
        updated_content = safe_update_markdown(
            existing_content,
            frontmatter,
            new_summary,
            start_tag="%% START_RACE_SUMMARY %%",
            end_tag="%% END_RACE_SUMMARY %%",
            missing_tags_template=missing_template
        )

        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(updated_content)
        written_paths.append(file_path)

    # 2. Write/Update summary.md
    summary_path = os.path.join(races_dir, "summary.md")
    
    # Build Table Headers
    table_headers = ["Date", "Sport", "Race", "Distance", "Time", "Pace", "Location", "Source"]
    table_rows = [
        "| " + " | ".join(table_headers) + " |",
        "| " + " | ".join(["---"] * len(table_headers)) + " |"
    ]
    
    if not races:
        table_rows.append("| - | - | No races recorded yet. | - | - | - | - | - |")
    else:
        for a in races:
            date_str = a.date.strftime("%a, %b %d, %Y")
            emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", "🏋️"))
            sport_str = f"{emoji} {a.sport}"
            dist_str = format_distance(a.sport, a.distance_meters, unit)
            time_str = format_duration_compact(a.duration_seconds)
            pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
            
            loc_str = "-"
            if a.latitude is not None and a.longitude is not None:
                map_url = f"https://www.google.com/maps/search/?api=1&query={a.latitude},{a.longitude}"
                loc_name = a.location_name if a.location_name else "Map"
                loc_str = f"📍 [{loc_name}]({map_url})"

            src_links = []
            if "garmin" in a.sources and a.garmin_id:
                src_links.append(f"[Garmin](https://connect.garmin.com/modern/activity/{a.garmin_id})")
            elif "garmin" in a.sources:
                src_links.append("Garmin")
                
            if "strava" in a.sources and a.strava_id:
                src_links.append(f"[Strava](https://www.strava.com/activities/{a.strava_id})")
            elif "strava" in a.sources:
                src_links.append("Strava")
                
            src_str = ", ".join(src_links)
            
            # Wikilink to individual race file (note name is filename without .md)
            race_file_link_name = f"{a.date.isoformat()} - {clean_filename(a.title)}"
            race_link = f"[[{race_file_link_name}|{a.title}]]"
            
            is_trail = (a.sport == "Run" and a.elevation_gain_meters >= 609.6 and "strava" in a.sources)
            if is_trail:
                race_link += f" (also [[../TrailRuns/{race_file_link_name}|Trail Run]])"
            
            row = [
                date_str, sport_str, race_link, dist_str, time_str, pace_str, loc_str, src_str
            ]
            table_rows.append("| " + " | ".join(row) + " |")

    table_content = "\n".join(table_rows)

    summary_frontmatter = {
        "type": "race-summary",
        "total_races": len(races)
    }

    existing_summary_content = ""
    if os.path.exists(summary_path):
        with open(summary_path, 'r', encoding='utf-8') as f:
            existing_summary_content = f.read()

    missing_summary_template = (
        f"\n# Races Index\n\n"
        f"{{replacement}}\n\n"
        f"{{body}}"
    )

    updated_summary_content = safe_update_markdown(
        existing_summary_content,
        summary_frontmatter,
        table_content,
        start_tag="%% START_RACES_LIST %%",
        end_tag="%% END_RACES_LIST %%",
        missing_tags_template=missing_summary_template
    )

    with open(summary_path, 'w', encoding='utf-8') as f:
        f.write(updated_summary_content)
    written_paths.append(summary_path)

    return written_paths


def write_trail_run_notes(
    vault_path: str,
    folder: str,
    activities: List[MergedActivity],
    unit: str
) -> List[str]:
    """
    Generate or update individual trail run notes and the summary.md file under <vault_path>/<folder>/TrailRuns/.
    Returns a list of created/updated markdown file paths.
    """
    # Filter for trail runs: runs from Strava with elevation gain >= 2000 ft (609.6 m)
    trail_runs = [
        a for a in activities 
        if a.sport == "Run" and a.elevation_gain_meters >= 609.6 and "strava" in a.sources
    ]
    
    # Sort chronologically
    trail_runs = sorted(trail_runs, key=lambda a: a.datetime)

    trail_runs_dir = os.path.join(vault_path, folder, "TrailRuns")
    os.makedirs(trail_runs_dir, exist_ok=True)

    written_paths = []

    # 1. Write/Update individual trail run files
    for a in trail_runs:
        filename = f"{a.date.isoformat()} - {clean_filename(a.title)}.md"
        file_path = os.path.join(trail_runs_dir, filename)

        # Format stats
        emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", "🏋️"))
        dist_str = format_distance(a.sport, a.distance_meters, unit)
        time_str = format_duration_compact(a.duration_seconds)
        pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
        
        hr_str = "-"
        if a.avg_hr is not None:
            max_hr_val = f"/{int(a.max_hr)}" if a.max_hr is not None else ""
            hr_str = f"{int(a.avg_hr)}{max_hr_val}"
            
        elev_str = format_elevation(a.elevation_gain_meters, unit)
        
        loc_str = "-"
        if a.latitude is not None and a.longitude is not None:
            map_url = f"https://www.google.com/maps/search/?api=1&query={a.latitude},{a.longitude}"
            loc_name = a.location_name if a.location_name else "Map"
            loc_str = f"📍 [{loc_name}]({map_url})"

        src_links = []
        if "garmin" in a.sources and a.garmin_id:
            src_links.append(f"[Garmin](https://connect.garmin.com/modern/activity/{a.garmin_id})")
        elif "garmin" in a.sources:
            src_links.append("Garmin")
            
        if "strava" in a.sources and a.strava_id:
            src_links.append(f"[Strava](https://www.strava.com/activities/{a.strava_id})")
        elif "strava" in a.sources:
            src_links.append("Strava")
            
        src_str = ", ".join(src_links)

        # Build new summary section
        cleaned_t = clean_filename(a.title)
        monday_date = get_monday_of_week(a.date)
        
        summary_lines = [
            "### Trail Run Stats",
            f"- **Sport**: {emoji} {a.sport}",
            f"- **Distance**: {dist_str}",
            f"- **Duration**: {time_str}",
            f"- **Pace/Speed**: {pace_str}",
            f"- **Elevation**: {elev_str}",
            f"- **Heart Rate**: {hr_str}",
            f"- **Location**: {loc_str}",
            f"- **Source**: {src_str}",
            f"- **Weekly Log**: [[../Week of {monday_date.isoformat()}|Week of {monday_date.isoformat()}]]",
        ]
        if a.is_race:
            summary_lines.append(f"- **Race Note**: [[../Races/{a.date.isoformat()} - {cleaned_t}|Race Note]]")
        
        if a.description and a.description.strip():
            summary_lines.append("")
            summary_lines.append("### Description")
            summary_lines.append(a.description.strip())
            
        new_summary = "\n".join(summary_lines)

        # Frontmatter
        frontmatter = {
            "type": "trail-run-activity",
            "date": a.date.isoformat(),
            "sport": a.sport,
            "title": a.title,
            "distance": round(a.distance_meters * (0.000621371 if unit == "miles" else 0.001), 2),
            "distance_unit": unit,
            "duration_seconds": int(a.duration_seconds),
            "elevation_gain_meters": float(a.elevation_gain_meters),
            "location": a.location_name,
            "sources": sorted(a.sources)
        }

        existing_content = ""
        if os.path.exists(file_path):
            with open(file_path, 'r', encoding='utf-8') as f:
                existing_content = f.read()

        missing_template = (
            f"\n# {a.title}\n\n"
            f"{{replacement}}\n\n"
            f"## Personal Notes\n"
            f"*Write your personal trail run report or notes here...*\n\n"
            f"{{body}}"
        )
        
        updated_content = safe_update_markdown(
            existing_content,
            frontmatter,
            new_summary,
            start_tag="%% START_TRAIL_RUN_SUMMARY %%",
            end_tag="%% END_TRAIL_RUN_SUMMARY %%",
            missing_tags_template=missing_template
        )

        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(updated_content)
        written_paths.append(file_path)

    # 2. Write/Update summary.md
    summary_path = os.path.join(trail_runs_dir, "summary.md")
    
    # Build Table Headers
    table_headers = ["Date", "Sport", "Trail Run", "Distance", "Time", "Pace", "Elevation", "Location", "Source"]
    table_rows = [
        "| " + " | ".join(table_headers) + " |",
        "| " + " | ".join(["---"] * len(table_headers)) + " |"
    ]
    
    if not trail_runs:
        table_rows.append("| - | - | No trail runs recorded yet. | - | - | - | - | - | - |")
    else:
        for a in trail_runs:
            date_str = a.date.strftime("%a, %b %d, %Y")
            emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", "🏋️"))
            sport_str = f"{emoji} {a.sport}"
            dist_str = format_distance(a.sport, a.distance_meters, unit)
            time_str = format_duration_compact(a.duration_seconds)
            pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
            elev_str = format_elevation(a.elevation_gain_meters, unit)
            
            loc_str = "-"
            if a.latitude is not None and a.longitude is not None:
                map_url = f"https://www.google.com/maps/search/?api=1&query={a.latitude},{a.longitude}"
                loc_name = a.location_name if a.location_name else "Map"
                loc_str = f"📍 [{loc_name}]({map_url})"

            src_links = []
            if "garmin" in a.sources and a.garmin_id:
                src_links.append(f"[Garmin](https://connect.garmin.com/modern/activity/{a.garmin_id})")
            elif "garmin" in a.sources:
                src_links.append("Garmin")
                
            if "strava" in a.sources and a.strava_id:
                src_links.append(f"[Strava](https://www.strava.com/activities/{a.strava_id})")
            elif "strava" in a.sources:
                src_links.append("Strava")
                
            src_str = ", ".join(src_links)
            
            # Wikilink to individual trail run file
            trail_file_link_name = f"{a.date.isoformat()} - {clean_filename(a.title)}"
            trail_link = f"[[{trail_file_link_name}|{a.title}]]"
            
            if a.is_race:
                trail_link += f" (also [[../Races/{trail_file_link_name}|Race]])"
            
            row = [
                date_str, sport_str, trail_link, dist_str, time_str, pace_str, elev_str, loc_str, src_str
            ]
            table_rows.append("| " + " | ".join(row) + " |")

    table_content = "\n".join(table_rows)

    summary_frontmatter = {
        "type": "trail-run-summary",
        "total_trail_runs": len(trail_runs)
    }

    existing_summary_content = ""
    if os.path.exists(summary_path):
        with open(summary_path, 'r', encoding='utf-8') as f:
            existing_summary_content = f.read()

    missing_summary_template = (
        f"\n# Trail Runs Index\n\n"
        f"{{replacement}}\n\n"
        f"{{body}}"
    )

    updated_summary_content = safe_update_markdown(
        existing_summary_content,
        summary_frontmatter,
        table_content,
        start_tag="%% START_TRAIL_RUNS_LIST %%",
        end_tag="%% END_TRAIL_RUNS_LIST %%",
        missing_tags_template=missing_summary_template
    )

    with open(summary_path, 'w', encoding='utf-8') as f:
        f.write(updated_summary_content)
    written_paths.append(summary_path)

    return written_paths


def write_weekly_weather_note(
    vault_path: str,
    folder: str,
    config: Dict[str, Any],
    unit: str
) -> str:
    """
    Fetch weather forecast for the next 7 days and write exactly one
    WeeklyWeather.md summary note in the Obsidian folder.
    """
    import datetime
    from running_cli.retrieval.weather import (
        geolocate_by_ip,
        fetch_weather,
        calculate_perceived_exertion,
        degrees_to_cardinal,
        WMO_CODES
    )
    
    # 1. Resolve location
    lat = config.get("weather_lat")
    lon = config.get("weather_lon")
    loc_name = config.get("weather_location")
    
    if lat is None or lon is None:
        res = geolocate_by_ip()
        if res:
            lat = res["lat"]
            lon = res["lon"]
            loc_name = res["name"]
        else:
            lat = 42.3709
            lon = -71.1828
            loc_name = "Watertown, Massachusetts, United States"
            
    today = datetime.date.today()
    start_date_str = today.isoformat()
    end_date_str = (today + datetime.timedelta(days=6)).isoformat()
    
    # 2. Fetch weather
    data = fetch_weather(lat, lon, start_date_str, unit, end_date_str)
    if not data or "hourly" not in data or "daily" not in data:
        raise RuntimeError("Failed to fetch weather data for WeeklyWeather summary.")
        
    daily = data.get("daily", {})
    daily_times = daily.get("time", [])
    weather_codes = daily.get("weather_code", [])
    temps_max = daily.get("temperature_2m_max", [])
    temps_min = daily.get("temperature_2m_min", [])
    sunrises = daily.get("sunrise", [])
    sunsets = daily.get("sunset", [])
    winds_max = daily.get("wind_speed_10m_max", [])
    
    # Process hourly data to group by date
    hourly = data.get("hourly", {})
    hourly_times = hourly.get("time", [])
    hourly_temps = hourly.get("temperature_2m", [])
    hourly_apparent_temps = hourly.get("apparent_temperature", [])
    hourly_humidities = hourly.get("relative_humidity_2m", [])
    hourly_wind_speeds = hourly.get("wind_speed_10m", [])
    hourly_wind_dirs = hourly.get("wind_direction_10m", [])
    hourly_precip_probs = hourly.get("precipitation_probability", [])
    hourly_weather_codes = hourly.get("weather_code", [])
    
    hourly_by_date = {}
    for i in range(len(hourly_times)):
        t_str = hourly_times[i]
        try:
            dt = datetime.datetime.fromisoformat(t_str)
        except Exception:
            continue
        d_str = dt.date().isoformat()
        if d_str not in hourly_by_date:
            hourly_by_date[d_str] = []
        hourly_by_date[d_str].append({
            "hour": dt.hour,
            "dt": dt,
            "temp": hourly_temps[i] if i < len(hourly_temps) else 0.0,
            "apparent_temp": hourly_apparent_temps[i] if i < len(hourly_apparent_temps) else 0.0,
            "humidity": hourly_humidities[i] if i < len(hourly_humidities) else 0,
            "wind_speed": hourly_wind_speeds[i] if i < len(hourly_wind_speeds) else 0.0,
            "wind_dir": hourly_wind_dirs[i] if i < len(hourly_wind_dirs) else 0.0,
            "precip": hourly_precip_probs[i] if i < len(hourly_precip_probs) else 0,
            "weather_code": hourly_weather_codes[i] if i < len(hourly_weather_codes) else 0,
        })
        
    # 3. Build Markdown content
    frontmatter = {
        "type": "weather-summary",
        "location": loc_name,
        "latitude": round(lat, 4),
        "longitude": round(lon, 4),
        "last_updated": datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "distance_unit": unit
    }
    
    table_headers = ["Date", "Condition", "Temp Range", "Max Wind", "Avg Hum", "Sun Times", "Best Run Window"]
    table_rows = [
        "| " + " | ".join(table_headers) + " |",
        "| " + " | ".join(["---"] * len(table_headers)) + " |"
    ]
    
    details_sections = []
    
    for i in range(len(daily_times)):
        d_str = daily_times[i]
        try:
            d_val = datetime.date.fromisoformat(d_str)
            date_display = d_val.strftime("%a, %b %d")
        except Exception:
            date_display = d_str
            
        w_code = weather_codes[i] if i < len(weather_codes) else 0
        cond_name, cond_emoji = WMO_CODES.get(w_code, ("Unknown", "❓"))
        cond_str = f"{cond_emoji} {cond_name}"
        
        t_max = temps_max[i] if i < len(temps_max) else 0.0
        t_min = temps_min[i] if i < len(temps_min) else 0.0
        temp_suffix = "°F" if unit == "miles" else "°C"
        temp_str = f"{int(round(t_min))}{temp_suffix} - {int(round(t_max))}{temp_suffix}"
        
        w_max = winds_max[i] if i < len(winds_max) else 0.0
        wind_suffix = "mph" if unit == "miles" else "kmh"
        wind_str = f"{int(round(w_max))} {wind_suffix}"
        
        # Sun Times
        sunrise_str = "-"
        sunset_str = "-"
        if i < len(sunrises) and i < len(sunsets):
            try:
                sr_dt = datetime.datetime.fromisoformat(sunrises[i])
                ss_dt = datetime.datetime.fromisoformat(sunsets[i])
                sunrise_str = sr_dt.strftime("%I:%M%p").lstrip('0')
                sunset_str = ss_dt.strftime("%I:%M%p").lstrip('0')
            except Exception:
                pass
        sun_times_str = f"{sunrise_str} - {sunset_str}"
        
        run_hours = [h for h in hourly_by_date.get(d_str, []) if 5 <= h["hour"] <= 19]
        
        avg_hum_val = 0
        best_hour_str = "No window"
        best_rpe_score = float('inf')
        best_rpe_label = "N/A"
        
        if run_hours:
            hums = [h["humidity"] for h in run_hours]
            avg_hum_val = int(round(sum(hums) / len(hums)))
            
            for h in run_hours:
                t_val = h["temp"]
                w_val = h["wind_speed"]
                rh_val = h["humidity"]
                
                if unit == "km":
                    t_f = t_val * 9 / 5 + 32
                    w_mph = w_val * 0.621371
                else:
                    t_f = t_val
                    w_mph = w_val
                    
                rpe_score, rpe_label, _ = calculate_perceived_exertion(t_f, rh_val, w_mph)
                
                if rpe_score < best_rpe_score:
                    best_rpe_score = rpe_score
                    best_rpe_label = rpe_label
                    best_hour_str = f"{h['dt'].strftime('%I:%M %p').lstrip('0')} ({rpe_label})"
                    
        table_rows.append(f"| {date_display} | {cond_str} | {temp_str} | {wind_str} | {avg_hum_val}% | {sun_times_str} | {best_hour_str} |")
        
        # Build hourly breakdown table
        day_hourly_lines = [
            f"### {date_display}",
            f"",
            f"| Hour | Condition | Temp (Feels) | Humidity | Wind | Precip | Perceived Exertion (RPE) |",
            f"| --- | --- | --- | --- | --- | --- | --- |"
        ]
        
        for h in hourly_by_date.get(d_str, []):
            if not (5 <= h["hour"] <= 19):
                continue
            
            hr_disp = h["dt"].strftime("%I:%M %p").lstrip('0')
            h_w_code = h["weather_code"]
            h_cond_name, h_cond_emoji = WMO_CODES.get(h_w_code, ("Unknown", "❓"))
            h_cond_str = f"{h_cond_emoji} {h_cond_name}"
            h_temp_str = f"{int(round(h['temp']))}{temp_suffix} ({int(round(h['apparent_temp']))}{temp_suffix})"
            h_hum_str = f"{h['humidity']}%"
            
            h_cardinal_dir = degrees_to_cardinal(h["wind_dir"])
            h_wind_str = f"{int(round(h['wind_speed']))} {wind_suffix} {h_cardinal_dir}"
            h_precip_str = f"{h['precip']}%"
            
            t_val = h["temp"]
            w_val = h["wind_speed"]
            rh_val = h["humidity"]
            if unit == "km":
                t_f = t_val * 9 / 5 + 32
                w_mph = w_val * 0.621371
            else:
                t_f = t_val
                w_mph = w_val
                
            rpe_score, rpe_label, _ = calculate_perceived_exertion(t_f, rh_val, w_mph)
            h_rpe_str = f"**{rpe_label}** (Score: {rpe_score})"
            
            day_hourly_lines.append(f"| {hr_disp} | {h_cond_str} | {h_temp_str} | {h_hum_str} | {h_wind_str} | {h_precip_str} | {h_rpe_str} |")
            
        day_hourly_lines.append("")
        details_sections.append("\n".join(day_hourly_lines))
        
    new_table_summary = "\n".join(table_rows)
    new_hourly_summary = "\n".join(details_sections)
    
    # 4. Save to Obsidian
    dest_dir = os.path.join(vault_path, folder)
    os.makedirs(dest_dir, exist_ok=True)
    file_path = os.path.join(dest_dir, "WeeklyWeather.md")
    
    existing_content = ""
    if os.path.exists(file_path):
        with open(file_path, 'r', encoding='utf-8') as f:
            existing_content = f.read()
            
    if not existing_content:
        existing_content = (
            f"---\n"
            f"type: weather-summary\n"
            f"---\n\n"
            f"# Weekly Weather & Running Exertion\n\n"
            f"**Location**: {loc_name}\n"
            f"**Last Updated**: {frontmatter['last_updated']}\n\n"
            f"%% START_WEATHER_OUTLOOK %%\n"
            f"%% END_WEATHER_OUTLOOK %%\n\n"
            f"## 📋 Exertion Index Legend\n"
            f"- **Ideal** (0-1.5): Perfect running conditions. RPE is baseline.\n"
            f"- **Moderate** (1.6-3.0): Slightly increased effort.\n"
            f"- **Hard** (3.1-5.0): Noticeable exertion increase.\n"
            f"- **Very Hard** (5.1-7.0): High cardiovascular strain.\n"
            f"- **Extreme** (>7.0): Severe strain.\n\n"
            f"## 📅 Detailed Hourly Outlooks (5:00 AM - 7:00 PM)\n\n"
            f"%% START_HOURLY_OUTLOOK %%\n"
            f"%% END_HOURLY_OUTLOOK %%\n\n"
            f"## 📝 Training & Running Notes\n"
            f"*Write your training notes or plans for the week here...*\n"
        )
        
    content_1 = safe_update_markdown(
        existing_content,
        frontmatter,
        new_table_summary,
        start_tag="%% START_WEATHER_OUTLOOK %%",
        end_tag="%% END_WEATHER_OUTLOOK %%",
        missing_tags_template="\n# Weather Outlook\n\n{replacement}\n\n{body}"
    )
    
    final_content = safe_update_markdown(
        content_1,
        {},
        new_hourly_summary,
        start_tag="%% START_HOURLY_OUTLOOK %%",
        end_tag="%% END_HOURLY_OUTLOOK %%",
        missing_tags_template="\n# Hourly Outlooks\n\n{replacement}\n\n{body}"
    )
    
    with open(file_path, 'w', encoding='utf-8') as f:
        f.write(final_content)
        
    return file_path

