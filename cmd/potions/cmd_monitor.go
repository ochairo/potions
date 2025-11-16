package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
	"github.com/ochairo/potions/internal/external-adapters/yaml"
)

// UpdateInfo represents information about an available update
type UpdateInfo struct {
	Package        string `json:"package"`
	CurrentVersion string `json:"current_version,omitempty"`
	LatestVersion  string `json:"latest_version"`
	UpdateNeeded   bool   `json:"update_needed"`
	RecipeFile     string `json:"recipe_file"`
	Error          string `json:"error,omitempty"`
}

func runMonitor(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	var (
		all        = fs.Bool("all", false, "Check all packages for updates")
		jsonOutput = fs.Bool("json", true, "Output results as JSON (default)")
		recipesDir = fs.String("recipes-dir", "recipes", "Path to recipes directory")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions monitor [options] [package...]

Check for available updates to packages.

If no packages are specified and --all is not set, checks all packages.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  potions monitor --all                    # Check all packages
  potions monitor kubectl helm age         # Check specific packages
  potions monitor kubectl --json=false     # Human-readable output
`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Initialize repository
	defRepo := yaml.NewRecipeRepository(*recipesDir)

	// Initialize version fetcher
	versionFetcher := gateways.NewVersionFetcher()

	// Determine which packages to check
	var packagesToCheck []string
	if *all || fs.NArg() == 0 {
		// Check all packages
		allDefs, err := defRepo.ListRecipes(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing recipes: %v\n", err)
			os.Exit(1)
		}
		for _, def := range allDefs {
			packagesToCheck = append(packagesToCheck, def.Name)
		}
	} else {
		// Check specified packages
		packagesToCheck = fs.Args()
	}

	if len(packagesToCheck) == 0 {
		fmt.Fprintf(os.Stderr, "No packages to check\n")
		os.Exit(1)
	}

	// Check each package for updates
	var updates []UpdateInfo
	for _, pkgName := range packagesToCheck {
		update := checkPackageUpdate(ctx, defRepo, versionFetcher, pkgName, *recipesDir)
		updates = append(updates, update)
	}

	// Output results
	if *jsonOutput {
		outputJSON(updates)
	} else {
		outputHuman(updates)
	}

	// Exit with code 0 if updates available (for CI/CD), 1 if error
	hasError := false
	for _, update := range updates {
		if update.Error != "" {
			hasError = true
			break
		}
	}

	if hasError {
		os.Exit(1)
	}
}

func checkPackageUpdate(ctx context.Context, defRepo *yaml.RecipeRepository, versionFetcher *gateways.VersionFetcher, pkgName, recipesDir string) UpdateInfo {
	update := UpdateInfo{
		Package:    pkgName,
		RecipeFile: fmt.Sprintf("%s/%s.yml", recipesDir, pkgName),
	}

	// Load recipe
	def, err := defRepo.GetRecipe(ctx, pkgName)
	if err != nil {
		update.Error = fmt.Sprintf("failed to load recipe: %v", err)
		return update
	}

	// Check if version source is configured
	if def.Version.Source == "" {
		update.Error = "no version_source configured"
		return update
	}

	// Fetch latest version
	latestVersion, err := versionFetcher.FetchLatestVersion(def)
	if err != nil {
		update.Error = fmt.Sprintf("failed to fetch version: %v", err)
		return update
	}

	update.LatestVersion = latestVersion
	update.UpdateNeeded = true // Since we don't track current versions yet

	return update
}

func outputJSON(updates []UpdateInfo) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(updates); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func outputHuman(updates []UpdateInfo) {
	fmt.Println("Package Update Check Results")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	updatesAvailable := 0
	errors := 0

	for _, update := range updates {
		//nolint:gocritic // ifElseChain: checking different struct fields, not suitable for switch
		if update.Error != "" {
			fmt.Printf("❌ %-20s ERROR: %s\n", update.Package, update.Error)
			errors++
		} else if update.UpdateNeeded {
			fmt.Printf("📦 %-20s %s (new version available)\n", update.Package, update.LatestVersion)
			updatesAvailable++
		} else {
			fmt.Printf("✅ %-20s %s (up to date)\n", update.Package, update.CurrentVersion)
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d packages checked, %d updates available, %d errors\n",
		len(updates), updatesAvailable, errors)
}
