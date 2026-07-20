package main

import (
	"embed"
	"running_cli/cmd"
)

//go:embed all:dashboard/dist
var assets embed.FS

func main() {
	cmd.DashboardAssets = assets
	cmd.Execute()
}
