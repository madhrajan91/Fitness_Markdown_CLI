import os
import json
from typing import Dict, Any, Optional

CONFIG_DIR = os.path.expanduser("~/.running_cli")
CONFIG_PATH = os.path.join(CONFIG_DIR, "config.json")

DEFAULT_CONFIG = {
    "obsidian_vault_path": "",
    "obsidian_folder": "Running/Weekly",
    "distance_unit": "miles",
    "weather_location": "",
    "weather_lat": None,
    "weather_lon": None,
    "garmin": {
        "email": ""
    },
    "strava": {
        "client_id": "",
        "client_secret": "",
        "refresh_token": ""
    }
}

def load_config() -> Dict[str, Any]:
    """Load configuration from the standard config file location, falling back to defaults."""
    if not os.path.exists(CONFIG_PATH):
        return DEFAULT_CONFIG.copy()
    try:
        with open(CONFIG_PATH, 'r') as f:
            config = json.load(f)
        
        # Merge loaded config with DEFAULT_CONFIG structure to ensure all keys exist
        merged_config = DEFAULT_CONFIG.copy()
        for k, v in config.items():
            if isinstance(v, dict) and k in merged_config and isinstance(merged_config[k], dict):
                merged_config[k] = {**merged_config[k], **v}
            else:
                merged_config[k] = v
        return merged_config
    except Exception:
        return DEFAULT_CONFIG.copy()

def save_config(config: Dict[str, Any]) -> None:
    """Save the configuration dictionary to disk."""
    os.makedirs(CONFIG_DIR, exist_ok=True)
    with open(CONFIG_PATH, 'w') as f:
        json.dump(config, f, indent=2)

def get_vault_path(config: Dict[str, Any]) -> Optional[str]:
    """Get the absolute, expanded path to the Obsidian Vault."""
    path = config.get("obsidian_vault_path", "")
    if not path:
        return None
    expanded = os.path.expanduser(path)
    return os.path.abspath(expanded)

def is_garmin_configured(config: Dict[str, Any]) -> bool:
    """Check if Garmin credentials are present."""
    garmin = config.get("garmin", {})
    return bool(garmin.get("email"))

def is_strava_configured(config: Dict[str, Any]) -> bool:
    """Check if Strava developer credentials and refresh token are present."""
    strava = config.get("strava", {})
    return bool(strava.get("client_id") and strava.get("client_secret") and strava.get("refresh_token"))
