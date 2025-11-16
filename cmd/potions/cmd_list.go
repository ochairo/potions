package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/external-adapters/yaml"
)

func runList(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	var (
		recipesDir   = fs.String("recipes-dir", "recipes", "Path to recipes directory")
		platform     = fs.String("platform", "", "Filter by platform (e.g., darwin-arm64)")
		securityOnly = fs.Bool("security-enabled", false, "Only show packages with security scanning enabled")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions list [options]

List all available package recipes.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  potions list
  potions list --platform darwin-arm64
  potions list --security-enabled
`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Initialize repository
	defRepo := yaml.NewRecipeRepository(*recipesDir)

	// Load recipes
	var defs []*entities.Recipe
	var err error

	if *platform != "" {
		defs, err = defRepo.GetRecipesByPlatform(ctx, *platform)
	} else {
		defs, err = defRepo.ListRecipes(ctx)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing packages: %v\n", err)
		os.Exit(1)
	}

	// Filter by security if requested
	if *securityOnly {
		filtered := make([]*entities.Recipe, 0)
		for _, def := range defs {
			if def.Security.ScanVulnerabilities {
				filtered = append(filtered, def)
			}
		}
		defs = filtered
	}

	// Display results
	if *platform != "" {
		fmt.Printf("Packages for platform %s (%d total):\n\n", *platform, len(defs))
	} else {
		fmt.Printf("Available packages (%d total):\n\n", len(defs))
	}

	for _, def := range defs {
		platforms := make([]string, 0, len(def.Download.Platforms))
		for p := range def.Download.Platforms {
			platforms = append(platforms, p)
		}

		fmt.Printf("  %-20s %s\n", def.Name, def.Description)
		fmt.Printf("  %-20s Version source: %s\n", "", def.Version.Source)
		fmt.Printf("  %-20s Platforms: %v\n", "", platforms)

		if def.Security.ScanVulnerabilities {
			fmt.Printf("  %-20s 🔒 Security: vulnerability scanning enabled\n", "")
		}
		if def.Security.VerifySignature {
			fmt.Printf("  %-20s 🔐 Security: GPG signature verification enabled\n", "")
		}

		fmt.Println()
	}
}
