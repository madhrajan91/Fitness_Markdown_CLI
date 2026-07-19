import json
import pytest
from datetime import date
from click.testing import CliRunner
from running_cli.main import cli
from running_cli.retrieval.weather import (
    degrees_to_cardinal,
    calculate_perceived_exertion,
    geocode_location,
    geolocate_by_ip,
    fetch_weather
)

def test_degrees_to_cardinal():
    assert degrees_to_cardinal(0) == "N"
    assert degrees_to_cardinal(360) == "N"
    assert degrees_to_cardinal(90) == "E"
    assert degrees_to_cardinal(180) == "S"
    assert degrees_to_cardinal(270) == "W"
    assert degrees_to_cardinal(45) == "NE"
    assert degrees_to_cardinal(22.5) == "NNE"
    assert degrees_to_cardinal(348.75) == "N"

def test_calculate_perceived_exertion():
    # Ideal: 50F, 40% humidity, 5 mph wind (Score: 0.0)
    score, label, desc = calculate_perceived_exertion(50.0, 40.0, 5.0)
    assert label == "Ideal"
    assert score == 0.0
    
    # Moderate: 68F, 50% humidity (dew point ~50F, score 0.5 for temp, 0.5 for dew point), 12 mph wind (1.0 wind) -> score 2.0
    score, label, desc = calculate_perceived_exertion(68.0, 50.0, 12.0)
    assert label == "Moderate"
    assert score == 2.0
    
    # Moderate: 75F, 65% humidity (dew point ~62F, score 1.5 for temp, 1.5 for dew point, 0 wind) -> score 3.0
    score, label, desc = calculate_perceived_exertion(75.0, 65.0, 5.0)
    assert label == "Moderate"
    assert score == 3.0
    
    # Hard: 82F, 75% humidity (dew point ~73F, score 3.0 temp, 4.5 dew point, 0 wind) -> score 7.5
    score, label, desc = calculate_perceived_exertion(82.0, 75.0, 0.0)
    assert label == "Extreme"
    
    # Very cold: 10F, 30% humidity, 5 mph wind -> score 3.0 (3.0 temp, 0 humidity, 0 wind)
    score, label, desc = calculate_perceived_exertion(10.0, 30.0, 5.0)
    assert label == "Moderate"
    assert score == 3.0

def test_geocode_location_success(monkeypatch):
    class MockResponse:
        status_code = 200
        def json(self):
            return {
                "results": [{
                    "latitude": 41.85,
                    "longitude": -87.65,
                    "name": "Chicago",
                    "admin1": "Illinois",
                    "country": "United States"
                }]
            }
        def raise_for_status(self):
            pass

    monkeypatch.setattr("requests.get", lambda url, timeout=10: MockResponse())
    res = geocode_location("Chicago")
    assert res is not None
    assert res["lat"] == 41.85
    assert res["lon"] == -87.65
    assert res["name"] == "Chicago, Illinois, United States"

def test_geocode_location_not_found(monkeypatch):
    class MockResponse:
        status_code = 200
        def json(self):
            return {"results": []}
        def raise_for_status(self):
            pass

    monkeypatch.setattr("requests.get", lambda url, timeout=10: MockResponse())
    res = geocode_location("NonExistentCityNameXYZ")
    assert res is None

def test_geolocate_by_ip_success(monkeypatch):
    class MockResponse:
        status_code = 200
        def json(self):
            return {
                "status": "success",
                "lat": 34.05,
                "lon": -118.24,
                "city": "Los Angeles",
                "regionName": "California",
                "country": "United States"
            }
        def raise_for_status(self):
            pass

    monkeypatch.setattr("requests.get", lambda url, timeout=5: MockResponse())
    res = geolocate_by_ip()
    assert res is not None
    assert res["lat"] == 34.05
    assert res["lon"] == -118.24
    assert "Los Angeles" in res["name"]

def test_geolocate_by_ip_fallback(monkeypatch):
    # Mock primary IP API to fail, fallback to succeed
    def mock_get(url, timeout=5):
        class MockResponse:
            status_code = 200
            def __init__(self, is_fallback=False):
                self.is_fallback = is_fallback
            def json(self):
                if self.is_fallback:
                    return {
                        "latitude": 42.36,
                        "longitude": -71.05,
                        "city": "Boston",
                        "region": "Massachusetts",
                        "country_name": "United States"
                    }
                raise RuntimeError("Main API failed")
            def raise_for_status(self):
                if not self.is_fallback:
                    raise RuntimeError("Main API failure")
        
        if "ip-api.com" in url:
            return MockResponse(is_fallback=False)
        else:
            return MockResponse(is_fallback=True)

    monkeypatch.setattr("requests.get", mock_get)
    res = geolocate_by_ip()
    assert res is not None
    assert res["lat"] == 42.36
    assert res["lon"] == -71.05
    assert "Boston" in res["name"]

def test_fetch_weather_success(monkeypatch):
    class MockResponse:
        status_code = 200
        def json(self):
            return {"hourly": {"time": ["2026-05-30T00:00"]}}
        def raise_for_status(self):
            pass

    monkeypatch.setattr("requests.get", lambda url, timeout=10: MockResponse())
    res = fetch_weather(40.0, -70.0, "2026-05-30", "miles")
    assert res is not None
    assert "hourly" in res

def test_weather_command_integration(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles",
        "weather_location": "Chicago, IL",
        "weather_lat": 41.85,
        "weather_lon": -87.65
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock Open-Meteo response
    weather_data = {
        "daily": {
            "sunrise": ["2026-05-30T05:22"],
            "sunset": ["2026-05-30T20:10"]
        },
        "hourly": {
            "time": [f"2026-05-30T{h:02d}:00" for h in range(24)],
            "temperature_2m": [70.0] * 24,
            "relative_humidity_2m": [60] * 24,
            "apparent_temperature": [70.0] * 24,
            "wind_speed_10m": [10.0] * 24,
            "wind_direction_10m": [180] * 24,
            "precipitation_probability": [0] * 24,
            "weather_code": [0] * 24
        }
    }
    
    monkeypatch.setattr("running_cli.retrieval.weather.fetch_weather", lambda *args, **kwargs: weather_data)
    
    # Run the click command
    result = runner.invoke(cli, ["weather", "-d", "2026-05-30"])
    
    assert result.exit_code == 0
    assert "=== Weather for Chicago, IL (2026-05-30) ===" in result.output
    assert "🌅 Sunrise: 05:22 AM  |  🌇 Sunset: 08:10 PM" in result.output
    assert "Hourly Forecast (5:00 AM - 7:00 PM):" in result.output
    # Check that hourly entries (e.g. 05:00 AM and 07:00 PM) are printed
    assert "05:00 AM" in result.output
    assert "07:00 PM" in result.output
    assert "08:00 PM" not in result.output # filtered out (since 8pm is 20:00)
    assert "☀️ Clear" in result.output
    assert "10 mph S" in result.output
    assert "Exertion Index Legend:" in result.output

def test_weather_command_invalid_date():
    runner = CliRunner()
    result = runner.invoke(cli, ["weather", "-d", "invalid-date-format"])
    assert "Error: Invalid date format" in result.output

def test_weather_command_geocode_failure(monkeypatch, tmp_path):
    runner = CliRunner()
    
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles"
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock geocode to return None
    monkeypatch.setattr("running_cli.retrieval.weather.geocode_location", lambda name: None)
    
    result = runner.invoke(cli, ["weather", "-l", "InvalidCityName"])
    assert "Error: Could not geocode location" in result.output

def test_weather_command_multi_day(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles",
        "weather_location": "Chicago, IL",
        "weather_lat": 41.85,
        "weather_lon": -87.65
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Create 3 days of hourly time
    hourly_times = []
    dates = ["2026-05-30", "2026-05-31", "2026-06-01"]
    for d in dates:
        for h in range(24):
            hourly_times.append(f"{d}T{h:02d}:00")
            
    # Mock Open-Meteo multi-day response
    weather_data = {
        "daily": {
            "time": dates,
            "weather_code": [0, 1, 3],
            "temperature_2m_max": [70.0, 72.0, 68.0],
            "temperature_2m_min": [50.0, 52.0, 48.0],
            "sunrise": [f"{d}T05:22" for d in dates],
            "sunset": [f"{d}T20:10" for d in dates],
            "wind_speed_10m_max": [10.0, 12.0, 15.0]
        },
        "hourly": {
            "time": hourly_times,
            "temperature_2m": [60.0] * len(hourly_times),
            "relative_humidity_2m": [60] * len(hourly_times),
            "wind_speed_10m": [8.0] * len(hourly_times)
        }
    }
    
    monkeypatch.setattr("running_cli.retrieval.weather.fetch_weather", lambda *args, **kwargs: weather_data)
    
    # Run the click command with --days 3
    result = runner.invoke(cli, ["weather", "-d", "2026-05-30", "--days", "3"])
    
    assert result.exit_code == 0
    assert "=== Daily Weather Outlook (3 Days) ===" in result.output
    assert "Location: Chicago, IL" in result.output
    assert "Sat, May 30" in result.output
    assert "Sun, May 31" in result.output
    assert "Mon, Jun 01" in result.output
    assert "☀️ Clear" in result.output
    assert "50°F - 70°F" in result.output
    assert "10 mph" in result.output
    assert "Exertion Index Legend:" in result.output

def test_parse_relative_date():
    from datetime import date, timedelta
    from running_cli.retrieval.weather import parse_relative_date
    
    today = date.today()
    
    # 1. Keywords
    assert parse_relative_date("today") == today
    assert parse_relative_date("tomorrow") == today + timedelta(days=1)
    assert parse_relative_date("day-after") == today + timedelta(days=2)
    assert parse_relative_date("day_after") == today + timedelta(days=2)
    
    # 2. Numeric offsets
    assert parse_relative_date("+3") == today + timedelta(days=3)
    assert parse_relative_date("4") == today + timedelta(days=4)
    
    # 3. Weekdays (check correct weekday calculation)
    mon_val = parse_relative_date("mon")
    assert mon_val is not None
    assert mon_val.weekday() == 0  # Monday is 0
    assert 1 <= (mon_val - today).days <= 7
    
    # 4. Standard format fallback
    assert parse_relative_date("2026-05-30") == date(2026, 5, 30)
    
    # 5. Invalid
    assert parse_relative_date("not-a-date") is None

def test_weather_command_relative_date(monkeypatch, tmp_path):
    runner = CliRunner()
    
    # Mock config location
    temp_config = tmp_path / "config.json"
    temp_config.write_text(json.dumps({
        "distance_unit": "miles",
        "weather_location": "Chicago, IL",
        "weather_lat": 41.85,
        "weather_lon": -87.65
    }))
    
    from running_cli import config
    monkeypatch.setattr(config, "CONFIG_PATH", str(temp_config))
    
    # Mock Open-Meteo response
    weather_data = {
        "daily": {
            "sunrise": ["2026-05-30T05:22"],
            "sunset": ["2026-05-30T20:10"]
        },
        "hourly": {
            "time": [f"2026-05-30T{h:02d}:00" for h in range(24)],
            "temperature_2m": [70.0] * 24,
            "relative_humidity_2m": [60] * 24,
            "apparent_temperature": [70.0] * 24,
            "wind_speed_10m": [10.0] * 24,
            "wind_direction_10m": [180] * 24,
            "precipitation_probability": [0] * 24,
            "weather_code": [0] * 24
        }
    }
    
    called_args = []
    def mock_fetch_weather(lat, lon, start_date, unit, end_date):
        called_args.append(start_date)
        return weather_data
        
    monkeypatch.setattr("running_cli.retrieval.weather.fetch_weather", mock_fetch_weather)
    
    # Invoke command with "tomorrow"
    result = runner.invoke(cli, ["weather", "-d", "tomorrow"])
    assert result.exit_code == 0
    from datetime import date, timedelta
    expected_tomorrow_str = (date.today() + timedelta(days=1)).isoformat()
    assert called_args[0] == expected_tomorrow_str


