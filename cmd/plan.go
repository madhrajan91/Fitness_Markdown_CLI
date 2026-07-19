package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"running_cli/pkg/db"
	"running_cli/pkg/models"
	"running_cli/pkg/storage"

	"github.com/spf13/cobra"
)

var (
	planName string
	planDesc string
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage training plans and matching metrics",
}

var planImportCmd = &cobra.Command{
	Use:   "import [csv_file_path]",
	Short: "Import a training plan from a CSV file",
	Args:  cobra.ExactArgs(1),
	Run:   runPlanImport,
}

var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active training plans",
	Run:   runPlanList,
}

var planStatusCmd = &cobra.Command{
	Use:   "status [plan_id]",
	Short: "View training compliance and planned workouts for a plan",
	Args:  cobra.MaximumNArgs(1),
	Run:   runPlanStatus,
}

var planLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manually trigger auto-linking of synced activities to scheduled training plans",
	Run:   runPlanLink,
}

func init() {
	planImportCmd.Flags().StringVar(&planName, "name", "My Training Plan", "Name of the training plan")
	planImportCmd.Flags().StringVar(&planDesc, "desc", "", "Description of the training plan")

	planCmd.AddCommand(planImportCmd)
	planCmd.AddCommand(planListCmd)
	planCmd.AddCommand(planStatusCmd)
	planCmd.AddCommand(planLinkCmd)

	rootCmd.AddCommand(planCmd)
}

func runPlanImport(cmd *cobra.Command, args []string) {
	csvPath := args[0]
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		fmt.Printf("Error: File '%s' does not exist.\n", csvPath)
		return
	}

	fmt.Printf("Parsing CSV training plan from %s...\n", csvPath)
	plan, workouts, err := storage.ParseTrainingPlanCSV(csvPath, planName, planDesc)
	if err != nil {
		fmt.Printf("Error parsing CSV: %v\n", err)
		return
	}

	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	planID, err := database.SaveTrainingPlan(plan)
	if err != nil {
		fmt.Printf("Error saving training plan to DB: %v\n", err)
		return
	}

	fmt.Printf("Saved plan '%s' (ID: %d, Dates: %s to %s) to database.\n",
		plan.Name, planID, plan.StartDate.Format("2006-01-02"), plan.EndDate.Format("2006-01-02"))

	inserted := 0
	for _, pw := range workouts {
		pw.PlanID = planID
		if err := database.SavePlannedWorkout(pw); err != nil {
			fmt.Printf("  Warning: Failed to save workout on %s: %v\n", pw.PlannedDate.Format("2006-01-02"), err)
		} else {
			inserted++
		}
	}

	fmt.Printf("Imported %d workouts into training plan schedule.\n", inserted)

	// Run auto-linking right away
	linkedCount, _ := database.AutoLinkWorkouts()
	if linkedCount > 0 {
		fmt.Printf("Successfully matched %d synced activities to newly imported plan.\n", linkedCount)
	}
}

func runPlanList(cmd *cobra.Command, args []string) {
	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	plans, err := database.GetActivePlans()
	if err != nil {
		fmt.Printf("Error querying plans: %v\n", err)
		return
	}

	if len(plans) == 0 {
		fmt.Println("No training plans found. Import one using 'running-cli plan import [csv_file_path]'.")
		return
	}

	fmt.Println("=== Active Training Plans ===")
	fmt.Printf("%-4s | %-25s | %-12s | %-12s | %s\n", "ID", "Plan Name", "Start Date", "End Date", "Description")
	fmt.Println(strings.Repeat("-", 80))

	for _, p := range plans {
		fmt.Printf("%-4d | %-25s | %-12s | %-12s | %s\n",
			p.ID, p.Name, p.StartDate.Format("2006-01-02"), p.EndDate.Format("2006-01-02"), p.Description)
	}
}

func runPlanStatus(cmd *cobra.Command, args []string) {
	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	var planID int64
	if len(args) > 0 {
		var err error
		var val int
		_, err = fmt.Sscanf(args[0], "%d", &val)
		if err != nil {
			fmt.Println("Error: Invalid plan_id. Must be an integer.")
			return
		}
		planID = int64(val)
	} else {
		// Use the first plan as default
		plans, err := database.GetActivePlans()
		if err != nil || len(plans) == 0 {
			fmt.Println("No training plans found.")
			return
		}
		planID = plans[0].ID
	}

	workouts, err := database.GetPlannedWorkoutsForPlan(planID)
	if err != nil {
		fmt.Printf("Error loading workouts: %v\n", err)
		return
	}

	if len(workouts) == 0 {
		fmt.Println("No workouts found for this plan.")
		return
	}

	totalWorkouts := len(workouts)
	completed := 0
	scheduled := 0
	missed := 0

	var listWorkouts []*models.PlannedWorkout

	for _, w := range workouts {
		// Check if scheduled date has passed without completion
		if w.Status == "Scheduled" && w.PlannedDate.Before(time.Now().AddDate(0, 0, 0)) {
			w.Status = "Missed" // dynamic status check
			missed++
		} else if w.Status == "Scheduled" {
			scheduled++
		} else if w.Status == "Completed" {
			completed++
		} else {
			missed++
		}

		listWorkouts = append(listWorkouts, w)
	}

	complianceRate := 0.0
	if totalWorkouts-scheduled > 0 {
		complianceRate = (float64(completed) / float64(totalWorkouts-scheduled)) * 100.0
	}

	fmt.Printf("=== Training Plan Status (Plan ID: %d) ===\n", planID)
	fmt.Printf("Workouts Summary: %d Total | %d Completed | %d Scheduled | %d Missed\n",
		totalWorkouts, completed, scheduled, missed)
	fmt.Printf("Adherence Compliance Rate: %.1f%% (Completed/Passed)\n\n", complianceRate)

	// Display Workouts
	fmt.Printf("%-12s | %-8s | %-10s | %-10s | %-10s | %-12s | %s\n",
		"Date", "Sport", "Target Dist", "Target Dur", "Intensity", "Link Status", "Notes")
	fmt.Println(strings.Repeat("-", 100))

	for _, w := range listWorkouts {
		distStr := "-"
		if w.TargetDistanceMiles != nil {
			distStr = fmt.Sprintf("%.2f mi", *w.TargetDistanceMiles)
		}
		durStr := "-"
		if w.TargetDurationMinutes != nil {
			durStr = fmt.Sprintf("%.0f min", *w.TargetDurationMinutes)
		}

		statusColor := "\033[0;33mScheduled\033[0m"
		if w.Status == "Completed" {
			statusColor = "\033[0;32mCompleted\033[0m"
		} else if w.Status == "Missed" {
			statusColor = "\033[0;31mMissed\033[0m"
		}

		fmt.Printf("%-12s | %-8s | %-10s | %-10s | %-10s | %-12s | %s\n",
			w.PlannedDate.Format("2006-01-02"), w.Sport, distStr, durStr, w.TargetIntensity, statusColor, w.TargetNotes)
	}
}

func runPlanLink(cmd *cobra.Command, args []string) {
	database, err := db.OpenDB()
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer database.Close()

	fmt.Println("Auto-linking activities in database...")
	linkedCount, err := database.AutoLinkWorkouts()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Auto-linked %d workouts with activities successfully!\n", linkedCount)
}
