# Project Requirements & Dependencies

This document outlines the dependencies, system requirements, API credentials, and development requirements for the `running-cli` application.

---

## 💻 System Prerequisites
- **Python**: version `3.8` or newer (Tested up to `3.14`).
- **OS**: macOS, Linux, or Windows (with bash/powershell).
- **Git**: For version control.

---

## 📦 Python Dependencies

The libraries required by this application are defined in [requirements.txt](file:///Users/madhavrajan/Documents/AIProjects/running_cli/requirements.txt):

1. **`garminconnect>=0.2.8`**
   - API client for Garmin Connect. Handles parsing responses, Garmin session configurations, and fetching activities.
2. **`requests>=2.31.0`**
   - HTTP client library used for executing Strava OAuth queries, fetching geocoding lookups, and fetching Open-Meteo forecasts.
3. **`click>=8.1.7`**
   - CLI utility package. Powers commands, arguments, setup wizard prompts, and terminal color outputs.
4. **`PyYAML>=6.0.1`**
   - Used for parsing and modifying Obsidian YAML frontmatter configurations without wiping custom keys.
5. **`pytest>=8.0.0`**
   - Python testing framework.
6. **`pytest-mock>=3.12.0`**
   - Pytest extension for mocking APIs and CLI prompts during testing.

---

## 🔑 External Accounts & API Credentials

To perform synchronizations, you must have accounts configured on the respective services:

### 1. Garmin Connect Account
- **Access**: Standard email and password.
- **MFA**: Multi-factor authentication is supported.
- **Session Tokens**: Handled passwordless via `garth` and stored inside `~/.running_cli/garmin_session/`.

### 2. Strava API Developer Account
- **Access**: A registered application on [Strava Developers](https://www.strava.com/settings/api).
- **Credentials**:
  - `Client ID`
  - `Client Secret`
  - `Refresh Token` (Obtained during interactive OAuth authorization redirect on `localhost:8000`).

### 3. Open-Meteo Weather API
- **Access**: Public endpoints (`geocoding-api.open-meteo.com` and `api.open-meteo.com`).
- **API Key**: No key or authentication token is required.

### 4. IP Geolocation API (Fallback)
- **Access**: Public endpoints (`ip-api.com` or `ipapi.co`).
- **API Key**: No key or authentication token is required.

---

## 📁 Storage Requirements
- **Local Cache**: Roughly `10 KB` per activity JSON in `~/.running_cli/cache/`.
- **Obsidian Vault**: Must have a local folder (e.g., `Areas/Running`) with write permissions for saving markdown files.
