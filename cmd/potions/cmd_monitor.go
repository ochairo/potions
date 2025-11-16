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
		repoOwner  = fs.String("repo-owner", "ochairo", "GitHub repository owner")
		repoName   = fs.String("repo-name", "potions", "GitHub repository name")
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

	// Initialize GitHub gateway for release checking
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	var githubGW *gateways.HTTPGitHubGateway
	if token != "" {
		githubGW = gateways.NewHTTPGitHubGateway(token)
	}

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

	// Check each package for updates with timeout protection
	var updates []UpdateInfo
	for _, pkgName := range packagesToCheck {
		// Check if context already cancelled (timeout or user interrupt)
		select {
		case <-ctx.Done():
			// Context cancelled - output what we have so far
			if *jsonOutput {
				outputJSON(updates)
			} else {
				outputHuman(updates)
				fmt.Fprintf(os.Stderr, "\n⚠️  Stopped checking packages: %v\n", ctx.Err())
				fmt.Fprintf(os.Stderr, "Checked %d of %d packages.\n", len(updates), len(packagesToCheck))
			}
			os.Exit(1)
		default:
			// Continue checking
		}

		update := checkPackageUpdate(ctx, defRepo, versionFetcher, githubGW, pkgName, *recipesDir, *repoOwner, *repoName)
		updates = append(updates, update)
	}

	// Output all results
	if *jsonOutput {
		outputJSON(updates)
	} else {
		outputHuman(updates)
	}

	// Always exit with code 0 - errors are documented in JSON and human-readable output
	// Individual package errors don't cause failure of the entire monitoring operation
	// The workflow script should parse the JSON to determine if there are updates
}

func checkPackageUpdate(ctx context.Context, defRepo *yaml.RecipeRepository, versionFetcher *gateways.VersionFetcher, githubGW *gateways.HTTPGitHubGateway, pkgName, recipesDir, repoOwner, repoName string) UpdateInfo {
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

	// Check if this version is already released on GitHub
	if githubGW != nil {
		releaseTag := fmt.Sprintf("%s-%s", pkgName, latestVersion)
		_, err := githubGW.GetRelease(ctx, repoOwner, repoName, releaseTag)
		switch {
		case err == nil:
			// Release exists - no update needed
			update.UpdateNeeded = false
			update.CurrentVersion = latestVersion
		case strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found"):
			// Release doesn't exist - update needed
			update.UpdateNeeded = true
		default:
			// Error checking release (e.g., rate limit, network issue)
			// Be conservative: assume update is NOT needed to avoid duplicate releases
			update.UpdateNeeded = false
			update.CurrentVersion = latestVersion
			update.Error = fmt.Sprintf("could not verify release status: %v", err)
		}
	} else {
		// No GitHub token - cannot check releases
		// Be conservative: assume update is NOT needed
		update.UpdateNeeded = false
		update.Error = "no GitHub token available to check existing releases"
	}

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
