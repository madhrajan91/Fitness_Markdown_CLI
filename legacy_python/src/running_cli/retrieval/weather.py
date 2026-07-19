import urllib.parse
import requests
from datetime import datetime, date, timedelta
from typing import Optional, Dict, Any, Tuple

WMO_CODES = {
    0: ("Clear", "☀️"),
    1: ("Mainly Clear", "🌤️"),
    2: ("Partly Cloudy", "⛅"),
    3: ("Overcast", "☁️"),
    45: ("Fog", "🌫️"),
    48: ("Depositing Rime Fog", "🌫️"),
    51: ("Light Drizzle", "🌧️"),
    53: ("Moderate Drizzle", "🌧️"),
    55: ("Dense Drizzle", "🌧️"),
    56: ("Light Freezing Drizzle", "🌧️"),
    57: ("Dense Freezing Drizzle", "🌧️"),
    61: ("Slight Rain", "🌧️"),
    63: ("Moderate Rain", "🌧️"),
    65: ("Heavy Rain", "🌧️"),
    66: ("Light Freezing Rain", "🌧️"),
    67: ("Heavy Freezing Rain", "🌧️"),
    71: ("Slight Snow", "❄️"),
    73: ("Moderate Snow", "❄️"),
    75: ("Heavy Snow", "❄️"),
    77: ("Snow Grains", "❄️"),
    80: ("Slight Rain Showers", "🌦️"),
    81: ("Moderate Rain Showers", "🌦️"),
    82: ("Violent Rain Showers", "🌦️"),
    85: ("Slight Snow Showers", "❄️"),
    86: ("Heavy Snow Showers", "❄️"),
    95: ("Thunderstorm", "⛈️"),
    96: ("Thunderstorm with Hail", "⛈️"),
    99: ("Severe Thunderstorm", "⛈️"),
}

def degrees_to_cardinal(degrees: float) -> str:
    """Convert wind direction in degrees to cardinal coordinates."""
    cardinals = ["N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"]
    idx = int((degrees + 11.25) / 22.5) % 16
    return cardinals[idx]

def geocode_location(location_name: str) -> Optional[Dict[str, Any]]:
    """
    Geocode a location name using the Open-Meteo Geocoding API.
    Returns a dict with 'lat', 'lon', and 'name' if found, else None.
    """
    if not location_name.strip():
        return None
    
    # State mapping for US abbreviations to full names
    STATE_MAP = {
        "AL": "Alabama", "AK": "Alaska", "AZ": "Arizona", "AR": "Arkansas", "CA": "California",
        "CO": "Colorado", "CT": "Connecticut", "DE": "Delaware", "FL": "Florida", "GA": "Georgia",
        "HI": "Hawaii", "ID": "Idaho", "IL": "Illinois", "IN": "Indiana", "IA": "Iowa",
        "KS": "Kansas", "KY": "Kentucky", "LA": "Louisiana", "ME": "Maine", "MD": "Maryland",
        "MA": "Massachusetts", "MI": "Michigan", "MN": "Minnesota", "MS": "Mississippi", "MO": "Missouri",
        "MT": "Montana", "NE": "Nebraska", "NV": "Nevada", "NH": "New Hampshire", "NJ": "New Jersey",
        "NM": "New Mexico", "NY": "New York", "NC": "North Carolina", "ND": "North Dakota", "OH": "Ohio",
        "OK": "Oklahoma", "OR": "Oregon", "PA": "Pennsylvania", "RI": "Rhode Island", "SC": "South Carolina",
        "SD": "South Dakota", "TN": "Tennessee", "TX": "Texas", "UT": "Utah", "VT": "Vermont",
        "VA": "Virginia", "WA": "Washington", "WV": "West Virginia", "WI": "Wisconsin", "WY": "Wyoming"
    }

    search_name = location_name.strip()
    filter_state = None
    if "," in location_name:
        parts = [p.strip() for p in location_name.split(",")]
        search_name = parts[0]
        if len(parts) > 1 and parts[1]:
            filter_state = parts[1]

    encoded_name = urllib.parse.quote(search_name)
    url = f"https://geocoding-api.open-meteo.com/v1/search?name={encoded_name}&count=50"
    
    try:
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        data = response.json()
        
        results = data.get("results")
        if not results:
            return None
        
        selected = None
        if filter_state:
            filter_state_lower = filter_state.lower()
            full_state_name = STATE_MAP.get(filter_state.upper(), "").lower()
            
            for res in results:
                admin1 = res.get("admin1", "")
                if admin1:
                    admin1_lower = admin1.lower()
                    if admin1_lower == filter_state_lower or (full_state_name and admin1_lower == full_state_name):
                        selected = res
                        break
        else:
            selected = results[0]
            
        if not selected:
            return None
        
        # Build a pretty formatted name
        parts = [selected.get("name")]
        
        admin1 = selected.get("admin1")
        if admin1:
            parts.append(admin1)
            
        country = selected.get("country")
        if country:
            parts.append(country)
            
        formatted_name = ", ".join([p for p in parts if p])
        
        return {
            "lat": selected["latitude"],
            "lon": selected["longitude"],
            "name": formatted_name
        }
    except Exception:
        return None

def geolocate_by_ip() -> Optional[Dict[str, Any]]:
    """
    Geolocate the user by their public IP address using a free IP API.
    Returns a dict with 'lat', 'lon', and 'name' if successful, else None.
    """
    # Use ip-api.com as it's free and requires no key
    url = "http://ip-api.com/json/"
    try:
        response = requests.get(url, timeout=5)
        response.raise_for_status()
        data = response.json()
        
        if data.get("status") == "success":
            city = data.get("city", "")
            region = data.get("regionName", "")
            country = data.get("country", "")
            
            parts = [city, region, country]
            formatted_name = ", ".join([p for p in parts if p])
            
            return {
                "lat": data["lat"],
                "lon": data["lon"],
                "name": formatted_name if formatted_name else "Detected Location"
            }
    except Exception:
        pass
    
    # Fallback to ipapi.co if ip-api.com is down
    url_fallback = "https://ipapi.co/json/"
    try:
        response = requests.get(url_fallback, timeout=5)
        response.raise_for_status()
        data = response.json()
        
        if not data.get("error"):
            city = data.get("city", "")
            region = data.get("region", "")
            country = data.get("country_name", "")
            
            parts = [city, region, country]
            formatted_name = ", ".join([p for p in parts if p])
            
            return {
                "lat": data["latitude"],
                "lon": data["longitude"],
                "name": formatted_name if formatted_name else "Detected Location"
            }
    except Exception:
        pass
        
    return None

def calculate_perceived_exertion(temp_f: float, humidity: float, wind_mph: float) -> Tuple[float, str, str]:
    """
    Estimate running perceived exertion impact (RPE) on a scale from 0 to 10+
    based on temperature (Fahrenheit), relative humidity (%), and wind speed (mph).
    
    Returns:
      (score: float, label: str, description: str)
    """
    # 1. Dew point impact calculation
    # Formula for dew point approximation in Celsius: Td = Tc - ((100 - RH) / 5)
    temp_c = (temp_f - 32) * 5 / 9
    dew_point_c = temp_c - ((100 - humidity) / 5)
    dew_point_f = (dew_point_c * 9 / 5) + 32
    
    humidity_points = 0.0
    if dew_point_f < 50:
        humidity_points = 0.0
    elif 50 <= dew_point_f < 60:
        humidity_points = 0.5
    elif 60 <= dew_point_f < 65:
        humidity_points = 1.5
    elif 65 <= dew_point_f < 70:
        humidity_points = 3.0
    elif 70 <= dew_point_f < 75:
        humidity_points = 4.5
    else:  # >= 75
        humidity_points = 6.0
        
    # 2. Temperature impact calculation
    temp_points = 0.0
    if temp_f < 20:
        temp_points = 3.0  # Extremely cold
    elif 20 <= temp_f < 32:
        temp_points = 1.5  # Very cold
    elif 32 <= temp_f <= 60:
        temp_points = 0.0  # Ideal range
    elif 60 < temp_f <= 70:
        temp_points = 0.5  # Warm
    elif 70 < temp_f <= 80:
        temp_points = 1.5  # Moderately warm
    elif 80 < temp_f <= 90:
        temp_points = 3.0  # Hot
    else:  # > 90
        temp_points = 5.0  # Extremely hot
        
    # 3. Wind impact calculation
    wind_points = 0.0
    if wind_mph < 10:
        wind_points = 0.0  # Light breeze, negligible or good cooling
    elif 10 <= wind_mph <= 18:
        wind_points = 1.0  # Moderate resistance
    elif 18 < wind_mph <= 25:
        wind_points = 2.0  # Strong headwind resistance
    else:  # > 25
        wind_points = 3.5  # Heavy wind resistance
        
    score = humidity_points + temp_points + wind_points
    
    if score <= 1.5:
        label = "Ideal"
        desc = "Perfect running conditions. RPE is baseline."
    elif 1.5 < score <= 3.0:
        label = "Moderate"
        desc = "Slightly increased effort. Keep steady pace."
    elif 3.0 < score <= 5.0:
        label = "Hard"
        desc = "Noticeable RPE increase. Slow pace slightly."
    elif 5.0 < score <= 7.0:
        label = "Very Hard"
        desc = "High cardiovascular strain. Hydrate and run by feel."
    else:
        label = "Extreme"
        desc = "Severe strain. Avoid peak hours or run indoors."
        
    return round(score, 1), label, desc

def fetch_weather(lat: float, lon: float, date_str: str, unit: str, end_date_str: Optional[str] = None) -> Optional[Dict[str, Any]]:
    """
    Fetch weather forecast data from the Open-Meteo Forecast API.
    Handles unit conversion parameter adjustments.
    
    unit: 'miles' or 'km'
    """
    temp_unit = "fahrenheit" if unit == "miles" else "celsius"
    wind_unit = "mph" if unit == "miles" else "kmh"
    
    end_date = end_date_str or date_str
    
    # Construct API Request
    url = (
        f"https://api.open-meteo.com/v1/forecast?"
        f"latitude={lat}&longitude={lon}&"
        f"hourly=temperature_2m,relative_humidity_2m,apparent_temperature,wind_speed_10m,wind_direction_10m,precipitation_probability,weather_code&"
        f"daily=sunrise,sunset,temperature_2m_max,temperature_2m_min,wind_speed_10m_max,weather_code&"
        f"timezone=auto&"
        f"start_date={date_str}&end_date={end_date}&"
        f"temperature_unit={temp_unit}&"
        f"wind_speed_unit={wind_unit}"
    )
    
    try:
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception:
        return None

def parse_relative_date(date_str: str) -> Optional[date]:
    """
    Parse natural language relative dates or weekdays up to 7 days.
    Returns a date object if successful, else None.
    """
    clean_str = date_str.strip().lower()
    today = datetime.now().date()
    
    # 1. Standard YYYY-MM-DD
    try:
        return datetime.strptime(clean_str, "%Y-%m-%d").date()
    except ValueError:
        pass
        
    # 2. Keywords
    if clean_str in ("today", "0"):
        return today
    if clean_str in ("tomorrow", "1", "+1"):
        return today + timedelta(days=1)
    if clean_str in ("day-after", "day_after", "2", "+2"):
        return today + timedelta(days=2)
        
    # 3. Numeric offsets (+3 to +7)
    if clean_str.startswith("+"):
        clean_value = clean_str[1:]
    else:
        clean_value = clean_str
        
    if clean_value.isdigit():
        val = int(clean_value)
        if 0 <= val <= 7:
            return today + timedelta(days=val)
            
    # 4. Weekdays
    weekdays = {
        "monday": 0, "mon": 0,
        "tuesday": 1, "tue": 1,
        "wednesday": 2, "wed": 2,
        "thursday": 3, "thu": 3,
        "friday": 4, "fri": 4,
        "saturday": 5, "sat": 5,
        "sunday": 6, "sun": 6
    }
    
    if clean_str in weekdays:
        target_wd = weekdays[clean_str]
        current_wd = today.weekday()
        days_ahead = (target_wd - current_wd) % 7
        if days_ahead == 0:
            days_ahead = 7
        return today + timedelta(days=days_ahead)
        
    return None

