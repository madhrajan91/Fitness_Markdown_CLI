package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	ConfigDirName  = ".running_cli"
	ConfigFileName = "config.json"
	DBFileName     = "running_cli.db"
)

type GarminConfig struct {
	Email string `json:"email"`
}

type StravaConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
}

type Config struct {
	ObsidianVaultPath string       `json:"obsidian_vault_path"`
	ObsidianFolder    string       `json:"obsidian_folder"`
	DistanceUnit      string       `json:"distance_unit"`
	WeatherLocation   string       `json:"weather_location"`
	WeatherLat        *float64     `json:"weather_lat"`
	WeatherLon        *float64     `json:"weather_lon"`
	Garmin            GarminConfig `json:"garmin"`
	Strava            StravaConfig `json:"strava"`
}

// GetConfigDir returns the absolute path to the configuration directory
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ConfigDirName), nil
}

// GetConfigPath returns the absolute path to the config.json file
func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// GetDBPath returns the absolute path to the SQLite database file
func GetDBPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DBFileName), nil
}

// LoadConfig loads configuration from disk, returning defaults if missing
func LoadConfig() (*Config, error) {
	cfgPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Default values
	cfg := &Config{
		ObsidianFolder: "Running/Weekly",
		DistanceUnit:   "miles",
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return cfg, err
	}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

// SaveConfig saves the configuration struct to disk
func SaveConfig(cfg *Config) error {
	cfgPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cfgPath, data, 0644)
}

// GetVaultPath returns the absolute expanded path to the Obsidian Vault
func (cfg *Config) GetVaultPath() (string, error) {
	if cfg.ObsidianVaultPath == "" {
		return "", errors.New("obsidian vault path is not configured")
	}

	// Expand home directory if starts with ~
	path := cfg.ObsidianVaultPath
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}

	return filepath.Abs(path)
}

// IsGarminConfigured checks if Garmin email is present
func (cfg *Config) IsGarminConfigured() bool {
	return cfg.Garmin.Email != ""
}

// IsStravaConfigured checks if Strava credentials and refresh token are present
func (cfg *Config) IsStravaConfigured() bool {
	return cfg.Strava.ClientID != "" && cfg.Strava.ClientSecret != "" && cfg.Strava.RefreshToken != ""
}
