import os
import click
import urllib.parse
from datetime import date, timedelta, datetime
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Optional

from running_cli.config import (
    load_config,
    save_config,
    get_vault_path,
    is_garmin_configured,
    is_strava_configured,
    DEFAULT_CONFIG
)
from running_cli.retrieval.garmin import (
    fetch_and_cache_garmin_activities,
    load_cached_garmin_activities,
    get_garmin_client
)
from running_cli.retrieval.strava import (
    fetch_and_cache_strava_activities,
    load_cached_strava_activities,
    refresh_strava_token
)
from running_cli.core.merger import merge_activities
from running_cli.storage.obsidian import (
    write_weekly_note,
    write_race_notes,
    write_trail_run_notes,
    get_monday_of_week,
    SPORT_EMOJIS,
    format_distance,
    format_duration_compact,
    format_pace_or_speed,
    format_elevation
)

# Temporary HTTP Server to catch Strava Redirect
class StravaAuthHandler(BaseHTTPRequestHandler):
    auth_code = None

    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        if parsed.path == "/auth":
            query = urllib.parse.parse_qs(parsed.query)
            if "code" in query:
                StravaAuthHandler.auth_code = query["code"][0]
                self.send_response(200)
                self.send_header("Content-type", "text/html")
                self.end_headers()
                self.wfile.write(b"<html><body><h1 style='color:#008000;'>Authorization Successful!</h1><p>You can close this browser tab and return to the terminal.</p></body></html>")
            else:
                self.send_response(400)
                self.send_header("Content-type", "text/html")
                self.end_headers()
                self.wfile.write(b"<html><body><h1 style='color:#FF0000;'>Error</h1><p>No authorization code found in URL redirect.</p></body></html>")
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        # Silence HTTP logs in terminal
        pass

def run_auth_server() -> str:
    """Run local server until authentication code is captured."""
    server = HTTPServer(("localhost", 8000), StravaAuthHandler)
    click.echo("Waiting for authorization redirect on http://localhost:8000/auth ...")
    while StravaAuthHandler.auth_code is None:
        server.handle_request()
    server.server_close()
    code = StravaAuthHandler.auth_code
    StravaAuthHandler.auth_code = None  # Reset for future setups
    return code


@click.group()
def cli():
    """CLI tool to sync Garmin and Strava running/sports activity summaries to Obsidian weekly notes."""
    pass


@cli.command()
def setup():
    """Interactive wizard to configure the sync credentials and settings."""
    click.echo("=== running-cli setup wizard ===\n")
    
    config = load_config()
    
    # 1. Obsidian Configuration
    click.echo("--- Obsidian Settings ---")
    vault = click.prompt("Obsidian Vault absolute path", default=config.get("obsidian_vault_path", ""))
    vault = os.path.expanduser(vault.strip())
    
    # Simple validation
    if not os.path.exists(vault):
        click.echo(click.style(f"Warning: Path '{vault}' does not exist. We'll still save it.", fg="yellow"))
        
    folder = click.prompt("Subfolder inside Vault for summaries", default=config.get("obsidian_folder", "Running/Weekly"))
    unit = click.prompt("Distance unit preference (miles / km)", default=config.get("distance_unit", "miles"), type=click.Choice(["miles", "km"]))
    
    config["obsidian_vault_path"] = vault
    config["obsidian_folder"] = folder.strip("/")
    config["distance_unit"] = unit
    
    # 2. Garmin Configuration
    click.echo("\n--- Garmin Connect Settings ---")
    setup_garmin = click.confirm("Configure Garmin Connect credentials?", default=True)
    if setup_garmin:
        garmin_email = click.prompt("Garmin Email", default=config["garmin"].get("email", ""))
        garmin_pass = click.prompt("Garmin Password", default="", hide_input=True)
        
        click.echo("Verifying Garmin credentials...")
        try:
            get_garmin_client(garmin_email.strip(), garmin_pass)
            config["garmin"]["email"] = garmin_email.strip()
            if "password" in config["garmin"]:
                del config["garmin"]["password"]
            click.echo(click.style("Garmin Authentication Verified & Session Token Saved!", fg="green"))
        except Exception as e:
            click.echo(click.style(f"Garmin verification failed: {e}", fg="red"))
            if click.confirm("Keep this email address anyway?", default=False):
                config["garmin"]["email"] = garmin_email.strip()
                if "password" in config["garmin"]:
                    del config["garmin"]["password"]
                
    # 3. Strava Configuration
    click.echo("\n--- Strava API Settings ---")
    click.echo("To configure Strava, you need to create a developer application at https://www.strava.com/settings/api")
    setup_strava = click.confirm("Configure Strava API credentials?", default=True)
    if setup_strava:
        client_id = click.prompt("Strava Client ID", default=config["strava"].get("client_id", ""))
        client_secret = click.prompt("Strava Client Secret", default=config["strava"].get("client_secret", ""), hide_input=True)
        
        # Guide user to browser
        auth_url = (
            f"https://www.strava.com/oauth/authorize?"
            f"client_id={client_id.strip()}&"
            f"redirect_uri=http://localhost:8000/auth&"
            f"response_type=code&"
            f"scope=activity:read_all"
        )
        click.echo("\nPlease open the following URL in your browser to authorize your CLI client:")
        click.echo(click.style(auth_url, fg="cyan", underline=True))
        
        # Start local redirect catcher
        try:
            auth_code = run_auth_server()
            
            # Exchange code for tokens
            click.echo("Exchanging auth code for tokens...")
            exchange_url = "https://www.strava.com/oauth/token"
            payload = {
                "client_id": client_id.strip(),
                "client_secret": client_secret.strip(),
                "code": auth_code,
                "grant_type": "authorization_code"
            }
            res = requests_post_with_catch(exchange_url, payload)
            if res:
                refresh_token = res.get("refresh_token")
                config["strava"]["client_id"] = client_id.strip()
                config["strava"]["client_secret"] = client_secret.strip()
                config["strava"]["refresh_token"] = refresh_token
                click.echo(click.style("Strava Authentication Verified & Saved!", fg="green"))
            else:
                click.echo(click.style("Failed to exchange authentication code.", fg="red"))
        except Exception as e:
            click.echo(click.style(f"Strava setup error: {e}", fg="red"))
            
    # 4. Weather Configuration
    click.echo("\n--- Weather Settings ---")
    setup_weather = click.confirm("Configure default location for weather forecast?", default=True)
    if setup_weather:
        weather_loc = click.prompt("Default Location (e.g. City name, 'Boston, MA')", default=config.get("weather_location", ""))
        if weather_loc.strip():
            click.echo("Verifying location via Open-Meteo...")
            from running_cli.retrieval.weather import geocode_location
            resolved = geocode_location(weather_loc)
            if resolved:
                config["weather_location"] = resolved["name"]
                config["weather_lat"] = resolved["lat"]
                config["weather_lon"] = resolved["lon"]
                click.echo(click.style(f"Location Verified: {resolved['name']} ({resolved['lat']:.4f}, {resolved['lon']:.4f})", fg="green"))
            else:
                click.echo(click.style("Location could not be geocoded.", fg="red"))
                if click.confirm("Keep the raw location string anyway?", default=False):
                    config["weather_location"] = weather_loc.strip()
                    config["weather_lat"] = None
                    config["weather_lon"] = None
        else:
            config["weather_location"] = ""
            config["weather_lat"] = None
            config["weather_lon"] = None

    # Save config
    save_config(config)
    click.echo("\n" + click.style("Setup completed successfully!", fg="green", bold=True))


def requests_post_with_catch(url, payload):
    import requests
    try:
        response = requests.post(url, data=payload)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        click.echo(click.style(f"HTTP Post request failed: {e}", fg="red"))
        return None


@cli.command()
def status():
    """Verify setup configuration and test connection to Garmin and Strava."""
    config = load_config()
    
    vault_path = get_vault_path(config)
    click.echo("=== Configuration Status ===")
    click.echo(f"Obsidian Vault: {vault_path if vault_path else 'Not configured'}")
    click.echo(f"Obsidian Folder: {config.get('obsidian_folder')}")
    click.echo(f"Preferred Unit: {config.get('distance_unit')}")
    
    # Garmin status
    if is_garmin_configured(config):
        click.echo("Garmin Connect: Configured. Testing connection...")
        try:
            get_garmin_client(email=config["garmin"]["email"])
            click.echo(click.style("  [OK] Garmin Connected (Session active)", fg="green"))
        except Exception as e:
            click.echo(click.style(f"  [ERROR] Garmin Login failed: {e}", fg="red"))
    else:
        click.echo("Garmin Connect: Not configured.")
        
    # Strava status
    if is_strava_configured(config):
        click.echo("Strava API: Configured. Testing connection...")
        try:
            refresh_strava_token(
                config["strava"]["client_id"],
                config["strava"]["client_secret"],
                config["strava"]["refresh_token"]
            )
            click.echo(click.style("  [OK] Strava Connected & Token Refreshed", fg="green"))
        except Exception as e:
            click.echo(click.style(f"  [ERROR] Strava token refresh failed: {e}", fg="red"))
    else:
        click.echo("Strava API: Not configured.")


@cli.command()
@click.option("--days", default=7, help="Number of days to sync from today. (Default is 7)")
@click.option("--start-date", help="Sync start date YYYY-MM-DD. Overrides --days.")
@click.option("--end-date", help="Sync end date YYYY-MM-DD. (Defaults to today)")
@click.option("--races-only", is_flag=True, help="Only sync races to the Races folder and skip generating weekly summaries.")
def sync(days: int, start_date: Optional[str], end_date: Optional[str], races_only: bool):
    """Fetch new activities from Garmin & Strava, merge them, and sync Obsidian weekly summaries."""
    config = load_config()
    
    vault_path = get_vault_path(config)
    if not vault_path:
        click.echo(click.style("Error: Obsidian Vault path is not configured. Run 'running-cli setup' first.", fg="red"))
        return
        
    if not is_garmin_configured(config) and not is_strava_configured(config):
        click.echo(click.style("Error: Neither Garmin nor Strava is configured. Run 'running-cli setup' first.", fg="red"))
        return

    # Parse end date
    if end_date:
        try:
            sync_end = date.fromisoformat(end_date)
        except ValueError:
            click.echo(click.style("Error: Invalid --end-date format. Use YYYY-MM-DD.", fg="red"))
            return
    else:
        sync_end = date.today()

    # Parse start date
    if start_date:
        try:
            sync_start = date.fromisoformat(start_date)
        except ValueError:
            click.echo(click.style("Error: Invalid --start-date format. Use YYYY-MM-DD.", fg="red"))
            return
    else:
        sync_start = sync_end - timedelta(days=days)

    click.echo(f"Syncing activities between {sync_start.isoformat()} and {sync_end.isoformat()}...")
    
    # 1. Fetch Garmin Activities
    if is_garmin_configured(config):
        click.echo("Fetching Garmin activities...")
        try:
            fetched = fetch_and_cache_garmin_activities(
                start_date=sync_start,
                end_date=sync_end,
                email=config["garmin"]["email"]
            )
            click.echo(click.style(f"  Successfully cached {len(fetched)} Garmin activities.", fg="green"))
        except RuntimeError as re:
            click.echo(click.style(f"  Garmin Session Error: {re}", fg="red"))
            click.echo(click.style("  Please run 'running-cli setup' to re-authenticate and restore your session.", fg="yellow"))
        except Exception as e:
            click.echo(click.style(f"  Error fetching Garmin activities: {e}", fg="red"))
    else:
        click.echo("Garmin is not configured, skipping.")
        
    # 2. Fetch Strava Activities
    if is_strava_configured(config):
        click.echo("Fetching Strava activities...")
        try:
            fetched = fetch_and_cache_strava_activities(
                config["strava"]["client_id"],
                config["strava"]["client_secret"],
                config["strava"]["refresh_token"],
                sync_start,
                sync_end
            )
            click.echo(click.style(f"  Successfully cached {len(fetched)} Strava activities.", fg="green"))
        except Exception as e:
            click.echo(click.style(f"  Error fetching Strava activities: {e}", fg="red"))
    else:
        click.echo("Strava is not configured, skipping.")

    # 3. Identify all weeks affected by this sync window
    start_monday = get_monday_of_week(sync_start)
    end_monday = get_monday_of_week(sync_end)
    
    weeks_to_sync = []
    curr = start_monday
    while curr <= end_monday:
        weeks_to_sync.append(curr)
        curr += timedelta(weeks=1)

    # 4. Generate/Update Weekly notes for all affected weeks
    # Load all cached activities for each week (so we compile complete weeks)
    unit = config.get("distance_unit", "miles")
    folder = config.get("obsidian_folder", "Running/Weekly")
    
    if races_only:
        click.echo("\nSkipping weekly notes generation (--races-only).")
        weeks_to_sync = []
    else:
        click.echo(f"\nProcessing weekly note exports for {len(weeks_to_sync)} week(s)...")
        
    for monday in weeks_to_sync:
        sunday = monday + timedelta(days=6)
        
        # Load cached activities for this specific week from both providers
        garmin_raw = load_cached_garmin_activities(monday, sunday)
        strava_raw = load_cached_strava_activities(monday, sunday)
        
        # Merge Garmin and Strava activities
        merged = merge_activities(garmin_raw, strava_raw)
        
        if not merged:
            click.echo(f"  No activities found for Week of {monday.isoformat()}, skipping note update.")
            continue
            
        # Write to Obsidian
        try:
            filepath = write_weekly_note(vault_path, folder, monday, merged, unit)
            click.echo(click.style(f"  [OK] Updated Weekly Note: {os.path.basename(filepath)} with {len(merged)} activities.", fg="green"))
        except Exception as e:
            click.echo(click.style(f"  [ERROR] Failed to write weekly note for {monday.isoformat()}: {e}", fg="red"))
            
    # 5. Process and update races
    click.echo("\nProcessing race activities...")
    try:
        # Load ALL cached activities from the entire local cache
        all_garmin_raw = load_cached_garmin_activities()
        all_strava_raw = load_cached_strava_activities()
        all_merged = merge_activities(all_garmin_raw, all_strava_raw)
        
        written_races = write_race_notes(vault_path, folder, all_merged, unit)
        
        # Filter for output message
        race_count = len([a for a in all_merged if a.is_race])
        click.echo(click.style(f"  [OK] Synced {race_count} race(s). Updated index at Races/summary.md", fg="green"))
    except Exception as e:
        click.echo(click.style(f"  [ERROR] Failed to sync race activities: {e}", fg="red"))

    # 6. Process and update trail runs (elevation gain >= 2000 ft / 609.6 m from Strava)
    click.echo("\nProcessing trail runs...")
    try:
        # We can reuse all_merged if it was successfully loaded, otherwise load it
        try:
            merged_for_trails = all_merged
        except NameError:
            all_garmin_raw = load_cached_garmin_activities()
            all_strava_raw = load_cached_strava_activities()
            merged_for_trails = merge_activities(all_garmin_raw, all_strava_raw)
            
        written_trails = write_trail_run_notes(vault_path, folder, merged_for_trails, unit)
        
        # Filter for output message
        trail_count = len([
            a for a in merged_for_trails 
            if a.sport == "Run" and a.elevation_gain_meters >= 609.6 and "strava" in a.sources
        ])
        click.echo(click.style(f"  [OK] Synced {trail_count} trail run(s). Updated index at TrailRuns/summary.md", fg="green"))
    except Exception as e:
        click.echo(click.style(f"  [ERROR] Failed to sync trail runs: {e}", fg="red"))
        
    # 7. Update Weekly Weather Summary
    click.echo("\nUpdating Weekly Weather Summary...")
    try:
        from running_cli.storage.obsidian import write_weekly_weather_note
        weather_path = write_weekly_weather_note(vault_path, folder, config, unit)
        click.echo(click.style(f"  [OK] Updated Weekly Weather Summary: {os.path.basename(weather_path)}", fg="green"))
    except Exception as e:
        click.echo(click.style(f"  [ERROR] Failed to update weekly weather note: {e}", fg="red"))
        
    click.echo("\n" + click.style("Sync complete!", fg="green", bold=True))


@cli.command()
@click.option("--week", help="Date in the target week (YYYY-MM-DD). Defaults to today.")
def view(week: Optional[str]):
    """View a summary of merged activities for a specific week from local cache."""
    config = load_config()
    unit = config.get("distance_unit", "miles")
    
    if week:
        try:
            target_date = date.fromisoformat(week)
        except ValueError:
            click.echo(click.style("Error: Invalid --week format. Use YYYY-MM-DD.", fg="red"))
            return
    else:
        target_date = date.today()
        
    monday = get_monday_of_week(target_date)
    sunday = monday + timedelta(days=6)
    
    click.echo(f"Retrieving cached activities for Week of {monday.isoformat()} ({monday.isoformat()} to {sunday.isoformat()})...\n")
    
    garmin_raw = load_cached_garmin_activities(monday, sunday)
    strava_raw = load_cached_strava_activities(monday, sunday)
    
    merged = merge_activities(garmin_raw, strava_raw)
    
    if not merged:
        click.echo("No cached activities found for this week.")
        return
        
    # Sort chronologically
    merged = sorted(merged, key=lambda a: a.datetime)
    
    # Print a summary table
    click.echo(click.style(f"=== Week of {monday.isoformat()} ===", bold=True))
    
    # Calculate stats
    totals_by_sport = {}
    for a in merged:
        sport = a.sport
        if sport not in totals_by_sport:
            totals_by_sport[sport] = {"distance": 0.0, "duration": 0.0, "count": 0}
        totals_by_sport[sport]["distance"] += a.distance_meters
        totals_by_sport[sport]["duration"] += a.duration_seconds
        totals_by_sport[sport]["count"] += 1
        
    click.echo("\nWeekly Breakdown:")
    for sport, stats in sorted(totals_by_sport.items()):
        emoji = SPORT_EMOJIS.get(sport, SPORT_EMOJIS.get("Other", ""))
        dist_str = format_distance(sport, stats["distance"], unit)
        dur_str = format_duration_compact(stats["duration"])
        cnt = stats["count"]
        act_label = "activity" if cnt == 1 else "activities"
        click.echo(f"  {emoji} {sport}: {dist_str} | {dur_str} | {cnt} {act_label}")
        
    # Activity table
    click.echo("\nActivity Log:")
    header = f"{'Date':<12} | {'Sport':<6} | {'Title':<30} | {'Distance':<10} | {'Time':<8} | {'Pace/Speed':<10} | {'HR':<7} | {'Location':<12} | {'Source':<15}"
    click.echo(click.style(header, bold=True))
    click.echo("-" * len(header))
    
    for a in merged:
        date_str = a.date.strftime("%a, %b %d")
        emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", ""))
        sport_str = f"{emoji} {a.sport}"
        dist_str = format_distance(a.sport, a.distance_meters, unit)
        time_str = format_duration_compact(a.duration_seconds)
        pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
        
        hr_str = "-"
        if a.avg_hr is not None:
            max_hr_val = f"/{int(a.max_hr)}" if a.max_hr is not None else ""
            hr_str = f"{int(a.avg_hr)}{max_hr_val}"
            
        loc_name = a.location_name if a.location_name else "-"
        if len(loc_name) > 12:
            loc_name = loc_name[:9] + "..."

        src_str = ", ".join(sorted(a.sources))
        
        title = a.title if len(a.title) <= 30 else a.title[:27] + "..."
        
        row = f"{date_str:<12} | {sport_str:<6} | {title:<30} | {dist_str:<10} | {time_str:<8} | {pace_str:<10} | {hr_str:<7} | {loc_name:<12} | {src_str:<15}"
        click.echo(row)
        
    # Activity Links
    has_links = any((a.garmin_id or a.strava_id) for a in merged)
    if has_links:
        click.echo("\nActivity Links:")
        for a in merged:
            links = []
            if "garmin" in a.sources and a.garmin_id:
                url = f"https://connect.garmin.com/modern/activity/{a.garmin_id}"
                links.append(f"\033]8;;{url}\033\\Garmin\033]8;;\033\\ ({url})")
            if "strava" in a.sources and a.strava_id:
                url = f"https://www.strava.com/activities/{a.strava_id}"
                links.append(f"\033]8;;{url}\033\\Strava\033]8;;\033\\ ({url})")
            if links:
                date_str = a.date.strftime("%a, %b %d")
                emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", ""))
                click.echo(f"  * {date_str} - {emoji} {a.title}: {' | '.join(links)}")


@cli.command()
@click.option("--week", help="Date in the target week (YYYY-MM-DD). Defaults to today.")
def open(week: Optional[str]):
    """Select a merged activity from a specific week and open it in the default web browser."""
    config = load_config()
    
    if week:
        try:
            target_date = date.fromisoformat(week)
        except ValueError:
            click.echo(click.style("Error: Invalid --week format. Use YYYY-MM-DD.", fg="red"))
            return
    else:
        target_date = date.today()
        
    monday = get_monday_of_week(target_date)
    sunday = monday + timedelta(days=6)
    
    click.echo(f"Retrieving cached activities for Week of {monday.isoformat()} ({monday.isoformat()} to {sunday.isoformat()})...\n")
    
    garmin_raw = load_cached_garmin_activities(monday, sunday)
    strava_raw = load_cached_strava_activities(monday, sunday)
    
    merged = merge_activities(garmin_raw, strava_raw)
    
    if not merged:
        click.echo("No cached activities found for this week.")
        return
        
    # Sort chronologically
    merged = sorted(merged, key=lambda a: a.datetime)
    
    click.echo("Cached Activities:")
    for idx, a in enumerate(merged, 1):
        date_str = a.date.strftime("%a, %b %d")
        emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", ""))
        click.echo(f"  {idx}. {date_str} - {emoji} {a.sport}: {a.title} ({', '.join(sorted(a.sources))})")
        
    val = click.prompt(
        f"\nSelect an activity to open (1-{len(merged)})",
        type=click.IntRange(1, len(merged))
    )
    
    selected = merged[val - 1]
    
    choices = []
    if "garmin" in selected.sources and selected.garmin_id:
        choices.append("garmin")
    if "strava" in selected.sources and selected.strava_id:
        choices.append("strava")
        
    if not choices:
        click.echo(click.style("No activity URL available for the selected activity.", fg="red"))
        return
        
    provider = choices[0]
    if len(choices) > 1:
        provider = click.prompt(
            "Multiple sources available. Open which provider?",
            type=click.Choice(choices),
            default=choices[0]
        )
        
    url = ""
    if provider == "garmin":
        url = f"https://connect.garmin.com/modern/activity/{selected.garmin_id}"
    elif provider == "strava":
        url = f"https://www.strava.com/activities/{selected.strava_id}"
        
    click.echo(f"Launching {provider.capitalize()} activity in browser: {url}")
    click.launch(url)


@cli.command()
@click.option("--start-date", help="Filter races starting from this date (YYYY-MM-DD).")
def races(start_date: Optional[str]):
    """View all cached races in a terminal table, optionally starting from a specific date."""
    config = load_config()
    unit = config.get("distance_unit", "miles")
    
    filter_date = None
    if start_date:
        try:
            filter_date = date.fromisoformat(start_date)
        except ValueError:
            click.echo(click.style("Error: Invalid --start-date format. Use YYYY-MM-DD.", fg="red"))
            return
            
    # Load all cached activities
    all_garmin_raw = load_cached_garmin_activities()
    all_strava_raw = load_cached_strava_activities()
    all_merged = merge_activities(all_garmin_raw, all_strava_raw)
    
    # Filter for races and optional start date
    race_activities = [a for a in all_merged if a.is_race]
    if filter_date:
        race_activities = [a for a in race_activities if a.date >= filter_date]
        
    if not race_activities:
        click.echo("No cached races found.")
        return
        
    # Sort chronologically
    race_activities.sort(key=lambda a: a.datetime)
    
    click.echo(click.style("=== Cached Races ===", bold=True))
    if filter_date:
        click.echo(f"Filtering starting from: {filter_date.isoformat()}\n")
    else:
        click.echo(f"Showing all {len(race_activities)} historical race(s):\n")
        
    header = f"{'Date':<12} | {'Sport':<6} | {'Title':<30} | {'Distance':<10} | {'Time':<8} | {'Pace/Speed':<10} | {'HR':<7} | {'Location':<12} | {'Source':<15}"
    click.echo(click.style(header, bold=True))
    click.echo("-" * len(header))
    
    for a in race_activities:
        date_str = a.date.strftime("%b %d, %Y")
        emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", ""))
        sport_str = f"{emoji} {a.sport}"
        dist_str = format_distance(a.sport, a.distance_meters, unit)
        time_str = format_duration_compact(a.duration_seconds)
        pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
        
        hr_str = "-"
        if a.avg_hr is not None:
            max_hr_val = f"/{int(a.max_hr)}" if a.max_hr is not None else ""
            hr_str = f"{int(a.avg_hr)}{max_hr_val}"
            
        loc_name = a.location_name if a.location_name else "-"
        if len(loc_name) > 12:
            loc_name = loc_name[:9] + "..."

        src_str = ", ".join(sorted(a.sources))
        title = a.title if len(a.title) <= 30 else a.title[:27] + "..."
        
        row = f"{date_str:<12} | {sport_str:<6} | {title:<30} | {dist_str:<10} | {time_str:<8} | {pace_str:<10} | {hr_str:<7} | {loc_name:<12} | {src_str:<15}"
        click.echo(row)
        
    # Add clickable links
    has_links = any((a.garmin_id or a.strava_id) for a in race_activities)
    if has_links:
        click.echo("\nRace Links:")
        for a in race_activities:
            links = []
            if "garmin" in a.sources and a.garmin_id:
                url = f"https://connect.garmin.com/modern/activity/{a.garmin_id}"
                links.append(f"\033]8;;{url}\033\\Garmin\033]8;;\033\\ ({url})")
            if "strava" in a.sources and a.strava_id:
                url = f"https://www.strava.com/activities/{a.strava_id}"
                links.append(f"\033]8;;{url}\033\\Strava\033]8;;\033\\ ({url})")
            if links:
                date_str = a.date.strftime("%b %d, %Y")
                emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", ""))
                click.echo(f"  * {date_str} - {emoji} {a.title}: {' | '.join(links)}")


@cli.command(name="trail-runs")
@click.option("--start-date", help="Filter trail runs starting from this date (YYYY-MM-DD).")
def trail_runs(start_date: Optional[str]):
    """View all cached trail runs in a terminal table, optionally starting from a specific date."""
    config = load_config()
    unit = config.get("distance_unit", "miles")
    
    filter_date = None
    if start_date:
        try:
            filter_date = date.fromisoformat(start_date)
        except ValueError:
            click.echo(click.style("Error: Invalid --start-date format. Use YYYY-MM-DD.", fg="red"))
            return
            
    # Load all cached activities
    all_garmin_raw = load_cached_garmin_activities()
    all_strava_raw = load_cached_strava_activities()
    all_merged = merge_activities(all_garmin_raw, all_strava_raw)
    
    # Filter for trail runs: Runs from Strava with elevation gain >= 2000 ft (609.6 m)
    trail_activities = [
        a for a in all_merged 
        if a.sport == "Run" and a.elevation_gain_meters >= 609.6 and "strava" in a.sources
    ]
    
    if filter_date:
        trail_activities = [a for a in trail_activities if a.date >= filter_date]
        
    if not trail_activities:
        click.echo("No cached trail runs found.")
        return
        
    # Sort chronologically
    trail_activities.sort(key=lambda a: a.datetime)
    
    click.echo(click.style("=== Cached Trail Runs ===", bold=True))
    if filter_date:
        click.echo(f"Filtering starting from: {filter_date.isoformat()}\n")
    else:
        click.echo(f"Showing all {len(trail_activities)} historical trail run(s):\n")
        
    header = f"{'Date':<12} | {'Sport':<6} | {'Title':<30} | {'Distance':<10} | {'Time':<8} | {'Pace/Speed':<10} | {'Elev':<8} | {'Location':<12} | {'Source':<15}"
    click.echo(click.style(header, bold=True))
    click.echo("-" * len(header))
    
    for a in trail_activities:
        date_str = a.date.strftime("%b %d, %Y")
        emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", ""))
        sport_str = f"{emoji} {a.sport}"
        dist_str = format_distance(a.sport, a.distance_meters, unit)
        time_str = format_duration_compact(a.duration_seconds)
        pace_str = format_pace_or_speed(a.sport, a.distance_meters, a.duration_seconds, unit)
        elev_str = format_elevation(a.elevation_gain_meters, unit)
        
        loc_name = a.location_name if a.location_name else "-"
        if len(loc_name) > 12:
            loc_name = loc_name[:9] + "..."

        src_str = ", ".join(sorted(a.sources))
        title = a.title if len(a.title) <= 30 else a.title[:27] + "..."
        
        row = f"{date_str:<12} | {sport_str:<6} | {title:<30} | {dist_str:<10} | {time_str:<8} | {pace_str:<10} | {elev_str:<8} | {loc_name:<12} | {src_str:<15}"
        click.echo(row)
        
    # Add clickable links
    has_links = any((a.garmin_id or a.strava_id) for a in trail_activities)
    if has_links:
        click.echo("\nTrail Run Links:")
        for a in trail_activities:
            links = []
            if "garmin" in a.sources and a.garmin_id:
                url = f"https://connect.garmin.com/modern/activity/{a.garmin_id}"
                links.append(f"\033]8;;{url}\033\\Garmin\033]8;;\033\\ ({url})")
            if "strava" in a.sources and a.strava_id:
                url = f"https://www.strava.com/activities/{a.strava_id}"
                links.append(f"\033]8;;{url}\033\\Strava\033]8;;\033\\ ({url})")
            if links:
                date_str = a.date.strftime("%b %d, %Y")
                emoji = SPORT_EMOJIS.get(a.sport, SPORT_EMOJIS.get("Other", ""))
                click.echo(f"  * {date_str} - {emoji} {a.title}: {' | '.join(links)}")


@cli.command()
@click.option("--location", "-l", help="Location name (city, state, or zip) to fetch weather for.")
@click.option("--lat", type=float, help="Latitude override.")
@click.option("--lon", type=float, help="Longitude override.")
@click.option("--date", "-d", help="Target date in YYYY-MM-DD format (defaults to today).")
@click.option("--days", type=click.IntRange(1, 16), default=1, help="Number of days to forecast (1-16). Shows hourly for 1 day, daily summary for >1 days.")
def weather(location: Optional[str], lat: Optional[float], lon: Optional[float], date: Optional[str], days: int):
    """Fetch weather forecast (hourly or multi-day summary) and perceived running exertion (RPE)."""
    config = load_config()
    unit = config.get("distance_unit", "miles")
    
    # Parse date
    if date:
        from running_cli.retrieval.weather import parse_relative_date
        parsed = parse_relative_date(date)
        if parsed:
            target_date = parsed
        else:
            click.echo(click.style("Error: Invalid date format. Use YYYY-MM-DD, keywords (today/tomorrow/day-after), offsets (+N), or weekday names (mon/tue/etc.).", fg="red"))
            return
    else:
        target_date = datetime.now().date()
    
    date_str = target_date.isoformat()
    end_date_str = (target_date + timedelta(days=days - 1)).isoformat()
    
    # Resolve coordinates
    resolved_name = ""
    target_lat = None
    target_lon = None
    
    if lat is not None and lon is not None:
        target_lat = lat
        target_lon = lon
        resolved_name = f"Coordinates ({lat:.4f}, {lon:.4f})"
    elif location:
        click.echo(f"Geocoding '{location}'...")
        from running_cli.retrieval.weather import geocode_location
        res = geocode_location(location)
        if res:
            target_lat = res["lat"]
            target_lon = res["lon"]
            resolved_name = res["name"]
        else:
            click.echo(click.style(f"Error: Could not geocode location '{location}'.", fg="red"))
            return
    else:
        # Check config
        conf_lat = config.get("weather_lat")
        conf_lon = config.get("weather_lon")
        conf_name = config.get("weather_location")
        if conf_lat is not None and conf_lon is not None:
            target_lat = conf_lat
            target_lon = conf_lon
            resolved_name = conf_name
        else:
            # Geolocate by IP
            click.echo("No location configured. Geolocating via IP...")
            from running_cli.retrieval.weather import geolocate_by_ip
            res = geolocate_by_ip()
            if res:
                target_lat = res["lat"]
                target_lon = res["lon"]
                resolved_name = res["name"]
                click.echo(f"Detected Location: {resolved_name}")
            else:
                click.echo(click.style("Error: Could not geolocate by IP and no default location is configured.", fg="red"))
                return
                
    # Fetch weather
    if days == 1:
        click.echo(f"Fetching weather forecast for {resolved_name} on {date_str}...")
    else:
        click.echo(f"Fetching {days}-day weather forecast for {resolved_name} from {date_str} to {end_date_str}...")
        
    from running_cli.retrieval.weather import fetch_weather, calculate_perceived_exertion, degrees_to_cardinal, WMO_CODES
    data = fetch_weather(target_lat, target_lon, date_str, unit, end_date_str)
    if not data or "hourly" not in data:
        click.echo(click.style("Error: Failed to retrieve weather forecast from Open-Meteo.", fg="red"))
        return
        
    if days == 1:
        # Get daily variables (sunrise/sunset)
        daily = data.get("daily", {})
        sunrise_list = daily.get("sunrise", [])
        sunset_list = daily.get("sunset", [])
        
        sunrise_str = "-"
        sunset_str = "-"
        if sunrise_list and sunset_list:
            try:
                sr_dt = datetime.fromisoformat(sunrise_list[0])
                ss_dt = datetime.fromisoformat(sunset_list[0])
                sunrise_str = sr_dt.strftime("%I:%M %p")
                sunset_str = ss_dt.strftime("%I:%M %p")
            except Exception:
                pass
                
        # Print summary header
        click.echo(click.style(f"\n=== Weather for {resolved_name} ({date_str}) ===", bold=True))
        click.echo(f"🌅 Sunrise: {sunrise_str}  |  🌇 Sunset: {sunset_str}")
        
        # Process hourly data
        hourly = data.get("hourly", {})
        times = hourly.get("time", [])
        temps = hourly.get("temperature_2m", [])
        humidities = hourly.get("relative_humidity_2m", [])
        apparent_temps = hourly.get("apparent_temperature", [])
        wind_speeds = hourly.get("wind_speed_10m", [])
        wind_dirs = hourly.get("wind_direction_10m", [])
        precip_probs = hourly.get("precipitation_probability", [])
        weather_codes = hourly.get("weather_code", [])
        
        click.echo(click.style(f"\nHourly Forecast (5:00 AM - 7:00 PM):", bold=True))
        header = f"{'Hour':<10} | {'Condition':<20} | {'Temp (Feels)':<14} | {'Humidity':<8} | {'Wind':<16} | {'Precip':<6} | {'Perceived Exertion (RPE)'}"
        click.echo(click.style(header, bold=True))
        click.echo("-" * 105)
        
        for i in range(len(times)):
            try:
                dt = datetime.fromisoformat(times[i])
            except Exception:
                continue
                
            # Filter for hours between 5:00 AM and 7:00 PM inclusive
            if not (5 <= dt.hour <= 19):
                continue
                
            hour_str = dt.strftime("%I:%M %p")
            
            # Values safely
            t_val = temps[i] if i < len(temps) else 0.0
            app_t_val = apparent_temps[i] if i < len(apparent_temps) else 0.0
            rh_val = humidities[i] if i < len(humidities) else 0
            w_speed = wind_speeds[i] if i < len(wind_speeds) else 0.0
            w_dir_deg = wind_dirs[i] if i < len(wind_dirs) else 0.0
            precip = precip_probs[i] if i < len(precip_probs) else 0
            w_code = weather_codes[i] if i < len(weather_codes) else 0
            
            temp_suffix = "°F" if unit == "miles" else "°C"
            wind_suffix = "mph" if unit == "miles" else "kmh"
            
            # Exertion math conversion
            if unit == "km":
                t_f = t_val * 9 / 5 + 32
                w_mph = w_speed * 0.621371
            else:
                t_f = t_val
                w_mph = w_speed
                
            rpe_score, rpe_label, rpe_desc = calculate_perceived_exertion(t_f, rh_val, w_mph)
            
            # Color codes
            if rpe_label == "Ideal":
                colored_rpe = click.style(f"{rpe_label} (Score: {rpe_score})", fg="green", bold=True)
            elif rpe_label == "Moderate":
                colored_rpe = click.style(f"{rpe_label} (Score: {rpe_score})", fg="yellow", bold=True)
            elif rpe_label == "Hard":
                colored_rpe = click.style(f"{rpe_label} (Score: {rpe_score})", fg="magenta", bold=True)
            elif rpe_label == "Very Hard":
                colored_rpe = click.style(f"{rpe_label} (Score: {rpe_score})", fg="red", bold=True)
            else:
                colored_rpe = click.style(f"{rpe_label} (Score: {rpe_score})", fg="red", bold=True, underline=True)
                
            cond_name, cond_emoji = WMO_CODES.get(w_code, ("Unknown", "❓"))
            cond_str = f"{cond_emoji} {cond_name}"
            temp_str = f"{int(round(t_val))}{temp_suffix} ({int(round(app_t_val))}{temp_suffix})"
            hum_str = f"{rh_val}%"
            
            cardinal_dir = degrees_to_cardinal(w_dir_deg)
            wind_str = f"{int(round(w_speed))} {wind_suffix} {cardinal_dir}"
            precip_str = f"{precip}%"
            
            row = f"{hour_str:<10} | {cond_str:<20} | {temp_str:<14} | {hum_str:<8} | {wind_str:<16} | {precip_str:<6} | {colored_rpe}"
            click.echo(row)
            
        click.echo("-" * 105)
    else:
        # Process multi-day summary
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
        hourly_humidities = hourly.get("relative_humidity_2m", [])
        hourly_wind_speeds = hourly.get("wind_speed_10m", [])
        
        hourly_by_date = {}
        for i in range(len(hourly_times)):
            t_str = hourly_times[i]
            try:
                dt = datetime.fromisoformat(t_str)
            except Exception:
                continue
            d_str = dt.date().isoformat()
            if d_str not in hourly_by_date:
                hourly_by_date[d_str] = []
            hourly_by_date[d_str].append({
                "hour": dt.hour,
                "dt": dt,
                "temp": hourly_temps[i] if i < len(hourly_temps) else 0.0,
                "humidity": hourly_humidities[i] if i < len(hourly_humidities) else 0,
                "wind_speed": hourly_wind_speeds[i] if i < len(hourly_wind_speeds) else 0.0,
            })
            
        click.echo(click.style(f"\n=== Daily Weather Outlook ({days} Days) ===", bold=True))
        click.echo(f"Location: {resolved_name}")
        
        header = f"{'Date':<15} | {'Condition':<20} | {'Temp Range':<14} | {'Max Wind':<12} | {'Avg Hum':<8} | {'Sun Times':<19} | {'Best Run Window'}"
        click.echo(click.style(header, bold=True))
        click.echo("-" * 111)
        
        for i in range(len(daily_times)):
            d_str = daily_times[i]
            try:
                d_val = datetime.strptime(d_str.strip(), "%Y-%m-%d")
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
            
            sunrise_str = "-"
            sunset_str = "-"
            if i < len(sunrises) and i < len(sunsets):
                try:
                    sr_dt = datetime.fromisoformat(sunrises[i])
                    ss_dt = datetime.fromisoformat(sunsets[i])
                    sunrise_str = sr_dt.strftime("%I:%M%p").lstrip('0')
                    sunset_str = ss_dt.strftime("%I:%M%p").lstrip('0')
                except Exception:
                    pass
            sun_times_str = f"{sunrise_str} - {sunset_str}"
            
            # Running hours (5 AM - 7 PM) hourly analytics
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
                        
            # Highlight best run window
            if best_rpe_label == "Ideal":
                colored_best = click.style(best_hour_str, fg="green", bold=True)
            elif best_rpe_label == "Moderate":
                colored_best = click.style(best_hour_str, fg="yellow", bold=True)
            elif best_rpe_label == "Hard":
                colored_best = click.style(best_hour_str, fg="magenta", bold=True)
            elif best_rpe_label == "Very Hard":
                colored_best = click.style(best_hour_str, fg="red", bold=True)
            elif best_rpe_label == "Extreme":
                colored_best = click.style(best_hour_str, fg="red", bold=True, underline=True)
            else:
                colored_best = best_hour_str
                
            hum_str = f"{avg_hum_val}%"
            row = f"{date_display:<15} | {cond_str:<20} | {temp_str:<14} | {wind_str:<12} | {hum_str:<8} | {sun_times_str:<19} | {colored_best}"
            click.echo(row)
            
        click.echo("-" * 111)
        
    click.echo(click.style("Exertion Index Legend:", bold=True))
    click.echo(f"  * {click.style('Ideal', fg='green', bold=True)} (0-1.5): Perfect conditions. RPE is baseline.")
    click.echo(f"  * {click.style('Moderate', fg='yellow', bold=True)} (1.6-3.0): Slight increase. Keep steady pace.")
    click.echo(f"  * {click.style('Hard', fg='magenta', bold=True)} (3.1-5.0): Noticeable RPE increase. Slow pace slightly.")
    click.echo(f"  * {click.style('Very Hard', fg='red', bold=True)} (5.1-7.0): High cardiovascular strain. Hydrate and run by feel.")
    click.echo(f"  * {click.style('Extreme', fg='red', bold=True, underline=True)} (>7.0): Severe strain. Avoid peak hours / run indoors.")
    # Automatically update Obsidian WeeklyWeather.md note if vault is configured
    from running_cli.config import get_vault_path
    try:
        vault_path = get_vault_path(config)
        if vault_path:
            folder = config.get("obsidian_folder", "Running/Weekly")
            from running_cli.storage.obsidian import write_weekly_weather_note
            write_weekly_weather_note(vault_path, folder, config, unit)
    except Exception:
        pass
    
    click.echo("")


if __name__ == "__main__":
    cli()


