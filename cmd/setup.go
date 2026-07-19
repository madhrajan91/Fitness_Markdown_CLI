package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"running_cli/pkg/client"
	"running_cli/pkg/config"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Run the interactive configuration wizard",
	Run:   runSetup,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check connection status and credentials validity",
	Run:   runStatus,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(statusCmd)
}

func runSetup(cmd *cobra.Command, args []string) {
	fmt.Println("=== running-cli setup wizard ===")
	fmt.Println()

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	reader := bufio.NewReader(os.Stdin)

	// 1. Obsidian Vault Settings
	fmt.Println("--- Obsidian Settings ---")
	fmt.Printf("Obsidian Vault absolute path [%s]: ", cfg.ObsidianVaultPath)
	vaultInput, _ := reader.ReadString('\n')
	vaultInput = strings.TrimSpace(vaultInput)
	if vaultInput != "" {
		cfg.ObsidianVaultPath = vaultInput
	}

	// Expand ~ for validation
	expPath := cfg.ObsidianVaultPath
	if len(expPath) > 0 && expPath[0] == '~' {
		home, _ := os.UserHomeDir()
		expPath = strings.Replace(expPath, "~", home, 1)
	}
	if _, err := os.Stat(expPath); os.IsNotExist(err) {
		fmt.Printf("Warning: Path '%s' does not exist on disk. We'll still save it.\n", cfg.ObsidianVaultPath)
	}

	fmt.Printf("Subfolder inside Vault for summaries [%s]: ", cfg.ObsidianFolder)
	folderInput, _ := reader.ReadString('\n')
	folderInput = strings.TrimSpace(folderInput)
	if folderInput != "" {
		cfg.ObsidianFolder = strings.Trim(folderInput, "/")
	}

	fmt.Printf("Distance unit preference (miles / km) [%s]: ", cfg.DistanceUnit)
	unitInput, _ := reader.ReadString('\n')
	unitInput = strings.TrimSpace(strings.ToLower(unitInput))
	if unitInput == "miles" || unitInput == "km" {
		cfg.DistanceUnit = unitInput
	}

	// 2. Garmin Settings
	fmt.Println("\n--- Garmin Connect Settings ---")
	fmt.Printf("Configure Garmin Connect credentials? (y/n) [y]: ")
	garminPrompt, _ := reader.ReadString('\n')
	garminPrompt = strings.TrimSpace(strings.ToLower(garminPrompt))
	if garminPrompt == "" || garminPrompt == "y" || garminPrompt == "yes" {
		fmt.Printf("Garmin Email [%s]: ", cfg.Garmin.Email)
		emailInput, _ := reader.ReadString('\n')
		emailInput = strings.TrimSpace(emailInput)
		if emailInput == "" {
			emailInput = cfg.Garmin.Email
		}

		fmt.Print("Garmin Password: ")
		passwordInput := readPassword()
		fmt.Println()

		fmt.Println("Verifying Garmin credentials...")
		// Test login by querying today's activities
		today := time.Now()
		_, err := client.FetchGarminActivities(emailInput, passwordInput, today, today)
		if err != nil {
			fmt.Printf("Garmin login verification failed: %v\n", err)
			fmt.Print("Keep this email address anyway? (y/n) [n]: ")
			keep, _ := reader.ReadString('\n')
			keep = strings.TrimSpace(strings.ToLower(keep))
			if keep == "y" || keep == "yes" {
				cfg.Garmin.Email = emailInput
			}
		} else {
			cfg.Garmin.Email = emailInput
			fmt.Println("Garmin Authentication Verified & Session Token Saved!")
		}
	}

	// 3. Strava Settings
	fmt.Println("\n--- Strava API Settings ---")
	fmt.Println("To configure Strava, you need to create a developer application at https://www.strava.com/settings/api")
	fmt.Printf("Configure Strava API credentials? (y/n) [y]: ")
	stravaPrompt, _ := reader.ReadString('\n')
	stravaPrompt = strings.TrimSpace(strings.ToLower(stravaPrompt))
	if stravaPrompt == "" || stravaPrompt == "y" || stravaPrompt == "yes" {
		fmt.Printf("Strava Client ID [%s]: ", cfg.Strava.ClientID)
		clientIDInput, _ := reader.ReadString('\n')
		clientIDInput = strings.TrimSpace(clientIDInput)
		if clientIDInput == "" {
			clientIDInput = cfg.Strava.ClientID
		}

		fmt.Print("Strava Client Secret: ")
		clientSecretInput := readPassword()
		fmt.Println()
		if clientSecretInput == "" {
			clientSecretInput = cfg.Strava.ClientSecret
		}

		authURL := fmt.Sprintf(
			"https://www.strava.com/oauth/authorize?client_id=%s&redirect_uri=http://localhost:8000/auth&response_type=code&scope=activity:read_all",
			clientIDInput,
		)

		fmt.Println("\nPlease open the following URL in your browser to authorize your CLI client:")
		fmt.Printf("\033[4;36m%s\033[0m\n\n", authURL)

		fmt.Println("Waiting for authorization redirect on http://localhost:8000/auth ...")
		authCode, err := client.StartStravaAuthServer()
		if err != nil {
			fmt.Printf("Strava authorization failed: %v\n", err)
		} else {
			fmt.Println("Exchanging auth code for tokens...")
			tokens, err := client.ExchangeStravaAuthCode(clientIDInput, clientSecretInput, authCode)
			if err != nil {
				fmt.Printf("Token exchange failed: %v\n", err)
			} else {
				cfg.Strava.ClientID = clientIDInput
				cfg.Strava.ClientSecret = clientSecretInput
				cfg.Strava.RefreshToken = tokens.RefreshToken
				fmt.Println("Strava Authentication Verified & Saved!")
			}
		}
	}

	// 4. Weather Settings
	fmt.Println("\n--- Weather Settings ---")
	fmt.Printf("Configure default location for weather forecast? (y/n) [y]: ")
	weatherPrompt, _ := reader.ReadString('\n')
	weatherPrompt = strings.TrimSpace(strings.ToLower(weatherPrompt))
	if weatherPrompt == "" || weatherPrompt == "y" || weatherPrompt == "yes" {
		fmt.Printf("Default Location (e.g. City name, 'Boston, MA') [%s]: ", cfg.WeatherLocation)
		locInput, _ := reader.ReadString('\n')
		locInput = strings.TrimSpace(locInput)
		if locInput == "" {
			locInput = cfg.WeatherLocation
		}

		if locInput != "" {
			fmt.Println("Verifying location via Open-Meteo...")
			resolved, err := client.GeocodeLocation(locInput)
			if err != nil {
				fmt.Printf("Location geocoding failed: %v\n", err)
				fmt.Print("Keep the raw location string anyway? (y/n) [n]: ")
				keep, _ := reader.ReadString('\n')
				keep = strings.TrimSpace(strings.ToLower(keep))
				if keep == "y" || keep == "yes" {
					cfg.WeatherLocation = locInput
					cfg.WeatherLat = nil
					cfg.WeatherLon = nil
				}
			} else {
				name := resolved["name"].(string)
				lat := resolved["lat"].(float64)
				lon := resolved["lon"].(float64)

				cfg.WeatherLocation = name
				cfg.WeatherLat = &lat
				cfg.WeatherLon = &lon

				fmt.Printf("Location Verified: %s (%.4f, %.4f)\n", name, lat, lon)
			}
		} else {
			cfg.WeatherLocation = ""
			cfg.WeatherLat = nil
			cfg.WeatherLon = nil
		}
	}

	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		return
	}

	fmt.Println("\n\033[1;32mSetup completed successfully!\033[0m")
}

func runStatus(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	vaultPath, err := cfg.GetVaultPath()
	fmt.Println("=== Configuration Status ===")
	if err != nil {
		fmt.Println("Obsidian Vault: Not configured")
	} else {
		fmt.Printf("Obsidian Vault: %s\n", vaultPath)
	}
	fmt.Printf("Obsidian Folder: %s\n", cfg.ObsidianFolder)
	fmt.Printf("Preferred Unit: %s\n", cfg.DistanceUnit)

	// Garmin status
	if cfg.IsGarminConfigured() {
		fmt.Println("Garmin Connect: Configured. Testing connection...")
		today := time.Now()
		_, err := client.FetchGarminActivities(cfg.Garmin.Email, "", today, today)
		if err != nil {
			fmt.Printf("  \033[0;31m[ERROR] Garmin Login failed: %v\033[0m\n", err)
		} else {
			fmt.Println("  \033[0;32m[OK] Garmin Connected (Session active)\033[0m")
		}
	} else {
		fmt.Println("Garmin Connect: Not configured.")
	}

	// Strava status
	if cfg.IsStravaConfigured() {
		fmt.Println("Strava API: Configured. Testing connection...")
		_, err := client.RefreshStravaToken(cfg.Strava.ClientID, cfg.Strava.ClientSecret, cfg.Strava.RefreshToken)
		if err != nil {
			fmt.Printf("  \033[0;31m[ERROR] Strava token refresh failed: %v\033[0m\n", err)
		} else {
			fmt.Println("  \033[0;32m[OK] Strava Connected & Token Refreshed\033[0m")
		}
	} else {
		fmt.Println("Strava API: Not configured.")
	}
}

// readPassword safely reads terminal input without echoing characters
func readPassword() string {
	fd := int(os.Stdin.Fd())
	bytePass, err := term.ReadPassword(fd)
	if err != nil {
		// Fallback simple read
		var s string
		fmt.Scanln(&s)
		return s
	}
	return string(bytePass)
}
