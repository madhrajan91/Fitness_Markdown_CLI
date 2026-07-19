package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "running-cli",
	Short: "running-cli pulls activities from Garmin & Strava, matches them, and syncs Obsidian weekly logs.",
	Long: `A CLI tool written in Go to fetch, merge, and organize sports activities from Garmin Connect
and Strava into Obsidian vaults as weekly summaries, races index, and weather forecasts.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Add global flags if necessary
}
