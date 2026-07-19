import os
import json
import pytest
from running_cli import config

def test_load_config_default(monkeypatch, tmp_path):
    # Patch CONFIG_PATH to point to a nonexistent file in a temp dir
    temp_config = tmp_path / "config.json"
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    cfg = config.load_config()
    assert cfg == config.DEFAULT_CONFIG
    assert cfg["distance_unit"] == "miles"

def test_save_and_load_config(monkeypatch, tmp_path):
    temp_config = tmp_path / "config.json"
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    monkeypatch.setattr(config, "CONFIG_DIR", str(tmp_path))
    
    cfg = config.DEFAULT_CONFIG.copy()
    cfg["obsidian_vault_path"] = "/tmp/my_vault"
    cfg["distance_unit"] = "km"
    cfg["garmin"]["email"] = "test@garmin.com"
    
    config.save_config(cfg)
    
    loaded = config.load_config()
    assert loaded["obsidian_vault_path"] == "/tmp/my_vault"
    assert loaded["distance_unit"] == "km"
    assert loaded["garmin"]["email"] == "test@garmin.com"

def test_is_configured(monkeypatch, tmp_path):
    cfg = {
        "garmin": {"email": "test@email.com"},
        "strava": {"client_id": "123", "client_secret": "sec", "refresh_token": "ref"},
        "obsidian_vault_path": "~/my_vault"
    }
    
    assert config.is_garmin_configured(cfg) is True
    assert config.is_strava_configured(cfg) is True
    
    cfg_incomplete = {
        "garmin": {"email": ""},
        "strava": {"client_id": "123", "client_secret": "", "refresh_token": "ref"}
    }
    assert config.is_garmin_configured(cfg_incomplete) is False
    assert config.is_strava_configured(cfg_incomplete) is False

def test_get_vault_path(monkeypatch):
    cfg = {"obsidian_vault_path": "~/my_vault"}
    
    # Mock expanduser
    monkeypatch.setattr(os.path, "expanduser", lambda p: p.replace("~", "/Users/testuser"))
    # Mock abspath
    monkeypatch.setattr(os.path, "abspath", lambda p: p)
    
    val = config.get_vault_path(cfg)
    assert val == "/Users/testuser/my_vault"
