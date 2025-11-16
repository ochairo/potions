package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
	"github.com/ochairo/potions/internal/domain/services"
	"github.com/ochairo/potions/internal/external-adapters/yaml"
)

func runValidateRelease(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("validate-release", flag.ExitOnError)
	var (
		artifactsDir = fs.String("artifacts", "current-artifacts", "Directory containing downloaded artifacts")
		recipesDir   = fs.String("recipes", "recipes", "Directory containing recipe YAML files")
		quiet        = fs.Bool("quiet", false, "Only output errors (exit code indicates success/failure)")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions validate-release <package> <version> [options]

Validate that all expected platform artifacts are present for a package release.

Arguments:
  package    Package name (required)
  version    Version to validate (required)

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit Codes:
  0  All expected platforms present (ready for release)
  1  Validation failed (platform mismatch, missing artifacts, etc.)
  2  Usage error or system error

Examples:
  potions validate-release kubectl v1.28.0
  potions validate-release kubectl v1.28.0 --artifacts ./dist
  potions validate-release kubectl v1.28.0 --quiet
`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(2)
	}

	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Error: package name and version are required\n\n")
		fs.Usage()
		os.Exit(2)
	}

	packageName := fs.Arg(0)
	version := fs.Arg(1)

	if err := executeValidateRelease(ctx, packageName, version, *artifactsDir, *recipesDir, *quiet); err != nil {
		if !*quiet {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func executeValidateRelease(ctx context.Context, packageName, version, artifactsDir, recipesDir string, quiet bool) error {
	if !quiet {
		fmt.Printf("🔍 Validating release for %s %s\n", packageName, version)
	}

	// Initialize artifact finder
	artifactFinder := gateways.NewArtifactFinder()

	// Load recipe
	recipeRepo := yaml.NewRecipeRepository(recipesDir)
	recipe, err := recipeRepo.GetRecipe(ctx, packageName)
	if err != nil {
		return fmt.Errorf("failed to load recipe: %w", err)
	}

	// Find all artifacts
	artifacts, err := artifactFinder.FindRecursive(artifactsDir, packageName, version)
	if err != nil {
		return fmt.Errorf("failed to find artifacts: %w", err)
	}

	if !quiet {
		fmt.Printf("📦 Found %d artifact files\n", len(artifacts))
	}

	// Validate
	releaseService := services.NewReleaseService()
	validation := releaseService.ValidateRelease(recipe, packageName, version, artifacts)

	if !quiet {
		fmt.Printf("\n Platform Validation:\n")
		fmt.Printf("  Expected: %d platforms\n", validation.ExpectedCount)
		fmt.Printf("  Available: %d platforms\n", validation.AvailableCount)

		if len(validation.ExpectedPlatforms) > 0 {
			fmt.Printf("  Expected platforms: ")
			for i, p := range validation.ExpectedPlatforms {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", p)
			}
			fmt.Println()
		}

		if len(validation.AvailablePlatforms) > 0 {
			fmt.Printf("  Available platforms: ")
			for i, p := range validation.AvailablePlatforms {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", p)
			}
			fmt.Println()
		}

		if len(validation.MissingPlatforms) > 0 {
			fmt.Printf("  Missing platforms: ")
			for i, p := range validation.MissingPlatforms {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", p)
			}
			fmt.Println()
		}

		if len(validation.UnexpectedPlatforms) > 0 {
			fmt.Printf("  Unexpected platforms: ")
			for i, p := range validation.UnexpectedPlatforms {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", p)
			}
			fmt.Println()
		}

		fmt.Println()
	}

	if !validation.IsReady() {
		errMsg := validation.ErrorMessage(packageName, version)
		if !quiet {
			fmt.Printf("❌ FAILED: %s\n", errMsg)
		}
		return fmt.Errorf("%s", errMsg)
	}

	if !quiet {
		fmt.Println("✅ READY: All expected platforms present")
	}

	return nil
}
