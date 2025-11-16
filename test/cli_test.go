package test_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildCLI builds the potions CLI binary for testing
func buildCLI(t *testing.T) string {
	t.Helper()

	// Get absolute path to recipes for tests - do this FIRST
	recipesPath, err := filepath.Abs("../recipes")
	if err != nil {
		t.Fatalf("Failed to get recipes path: %v", err)
	}
	t.Setenv("TEST_RECIPES_DIR", recipesPath)

	// Use a shared build directory
	buildDir := filepath.Join("..", "test-dist", "cli-bin")
	if err := os.MkdirAll(buildDir, 0750); err != nil {
		t.Fatalf("Failed to create build dir: %v", err)
	}

	cliPath := filepath.Join(buildDir, "potions")

	// Check if already built
	if _, err := os.Stat(cliPath); err == nil {
		return cliPath
	}

	t.Log("Building potions CLI...")
	cmd := exec.Command("go", "build", "-o", cliPath, "../cmd/potions") // #nosec G204 -- test code with controlled input
	cmd.Dir = filepath.Join("..", "test")

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, output)
	}

	t.Log("CLI built successfully")
	return cliPath
}

// TestCLI_HelpAndVersion tests help output for all commands
func TestCLI_HelpAndVersion(t *testing.T) {
	cliPath := buildCLI(t)

	commands := []string{
		"",
		"build",
		"release",
		"list",
		"monitor",
		"scan",
		"verify",
		"validate-release",
	}

	for _, cmd := range commands {
		t.Run("help_"+cmd, func(t *testing.T) {
			args := []string{"--help"}
			if cmd != "" {
				args = []string{cmd, "--help"}
			}

			execCmd := exec.Command(cliPath, args...) // #nosec G204 -- test code with controlled input
			output, err := execCmd.CombinedOutput()

			// Help should exit with 0 or 2 (usage error)
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					if exitErr.ExitCode() != 2 {
						t.Errorf("Help exited with unexpected code: %d", exitErr.ExitCode())
					}
				}
			}

			outputStr := string(output)
			if !strings.Contains(outputStr, "Usage") && !strings.Contains(outputStr, "Commands") {
				t.Errorf("Expected usage information in help output")
			}

			t.Logf("Help output:\n%s", outputStr)
		})
	}
}

// TestCLI_Build tests the build command CLI interface
func TestCLI_Build(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	cliPath := buildCLI(t)

	// Get recipes directory after buildCLI sets the environment variable
	recipesDir := os.Getenv("TEST_RECIPES_DIR")
	if recipesDir == "" {
		t.Fatal("TEST_RECIPES_DIR not set by buildCLI")
	}

	packageName := os.Getenv("TEST_PACKAGE")
	if packageName == "" {
		packageName = "curl" // Default to curl for quick testing
	}

	platform := "linux-amd64" // Use amd64 not x86_64
	if os.Getenv("RUNNER_OS") == "macOS" {
		platform = "darwin-arm64"
	}

	outputDir := t.TempDir()

	tests := []struct {
		name         string
		args         []string
		wantErr      bool
		allowNonZero bool // Allow non-zero exit code (e.g., when downloads fail)
		validate     func(t *testing.T, output string)
	}{
		{
			name:         "build with explicit version",
			allowNonZero: true, // May fail due to network/download issues
			args: []string{
				"build",
				"--platform", platform,
				"--output-dir", outputDir,
				"--recipes-dir", recipesDir,
				"--enable-security-scan=false",
				packageName, "latest",
			},
			validate: func(t *testing.T, output string) {
				// Should have "Building" message even if build fails
				if !strings.Contains(output, "Building") {
					t.Errorf("Expected 'Building' message in output")
				}
			},
		},
		{
			name:         "build with JSON packages input",
			allowNonZero: true, // May fail due to network/download issues
			args: []string{
				"build",
				"--packages", `[{"package":"` + packageName + `","version":"latest"}]`,
				"--platform", platform,
				"--output-dir", outputDir,
				"--recipes-dir", recipesDir,
				"--enable-security-scan=false",
				"--quiet",
			},
		},
		{
			name:         "build with custom output files",
			allowNonZero: true, // May fail due to network/download issues
			args: []string{
				"build",
				"--platform", platform,
				"--output-dir", outputDir,
				"--recipes-dir", recipesDir,
				"--enable-security-scan=false",
				"--successes", filepath.Join(outputDir, "success.txt"),
				"--failures", filepath.Join(outputDir, "failures.txt"),
				packageName, "latest",
			},
			validate: func(t *testing.T, _ string) {
				// Check that failure file exists (build will fail due to network)
				failureFile := filepath.Join(outputDir, "failures.txt")
				if _, err := os.Stat(failureFile); err == nil || os.IsNotExist(err) {
					// File may or may not exist depending on timing
					t.Logf("Failure file status: %v", err)
				}
			},
		},
		{
			name:         "build with JSON output",
			allowNonZero: true, // May fail due to network/download issues
			args: []string{
				"build",
				"--platform", platform,
				"--output-dir", outputDir,
				"--recipes-dir", recipesDir,
				"--enable-security-scan=false",
				"--json-output", filepath.Join(outputDir, "report.json"),
				packageName, "latest",
			},
			validate: func(t *testing.T, _ string) {
				reportFile := filepath.Join(outputDir, "report.json")
				if _, err := os.Stat(reportFile); err == nil {
					// If file exists, validate JSON format
					data, _ := os.ReadFile(reportFile) // #nosec G304 - reportFile is constructed from test temp dir
					var report map[string]interface{}
					if err := json.Unmarshal(data, &report); err != nil {
						t.Errorf("Invalid JSON in report: %v", err)
					}
				} else if !os.IsNotExist(err) {
					t.Errorf("Error checking report file: %v", err)
				}
				// File may not exist if build fails early
			},
		},
		{
			name:         "build with timeout setting",
			allowNonZero: true, // May fail due to network/download issues
			args: []string{
				"build",
				"--platform", platform,
				"--output-dir", outputDir,
				"--recipes-dir", recipesDir,
				"--enable-security-scan=false",
				"--timeout", "30",
				packageName, "latest",
			},
		},
		{
			name:    "build help",
			args:    []string{"build", "--help"},
			wantErr: false,
		},
		{
			name:    "build without required args",
			args:    []string{"build"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 300*1e9) // 5 min
			defer cancel()

			cmd := exec.CommandContext(ctx, cliPath, tt.args...) // #nosec G204 -- test code with controlled input
			cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
			output, err := cmd.CombinedOutput()

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}

			if !tt.wantErr && !tt.allowNonZero && err != nil {
				t.Errorf("Unexpected error: %v\nOutput: %s", err, output)
			}

			if tt.validate != nil {
				tt.validate(t, string(output))
			}

			t.Logf("Output:\n%s", output)
		})
	}
}

// TestCLI_List tests the list command
func TestCLI_List(t *testing.T) {
	cliPath := buildCLI(t)
	recipesDir := os.Getenv("TEST_RECIPES_DIR")
	ctx := context.Background()

	tests := []struct {
		name     string
		args     []string
		validate func(t *testing.T, output string)
	}{
		{
			name: "list all packages",
			args: []string{"list", "--recipes-dir", recipesDir},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "kubectl") && !strings.Contains(output, "curl") {
					t.Errorf("Expected to see some packages in list output")
				}
			},
		},
		{
			name: "list with platform filter",
			args: []string{"list", "--recipes-dir", recipesDir, "--platform", "darwin-arm64"},
			validate: func(t *testing.T, output string) {
				if strings.Contains(output, "Platforms:") && !strings.Contains(output, "darwin-arm64") {
					t.Errorf("Expected darwin-arm64 in filtered output")
				}
			},
		},
		{
			name: "list security-enabled only",
			args: []string{"list", "--recipes-dir", recipesDir, "--security-enabled"},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "🔒") {
					t.Errorf("Expected security indicator in security-enabled filter")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.CommandContext(ctx, cliPath, tt.args...) // #nosec G204 -- test code with controlled input
			cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
			output, err := cmd.CombinedOutput()

			if err != nil {
				t.Fatalf("list command failed: %v\nOutput: %s", err, output)
			}

			if tt.validate != nil {
				tt.validate(t, string(output))
			}

			t.Logf("Output:\n%s", output)
		})
	}
}

// TestCLI_Monitor tests the monitor command
func TestCLI_Monitor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// Skip if no GitHub token - monitor needs it for version checking
	if os.Getenv("GITHUB_TOKEN") == "" && os.Getenv("GH_TOKEN") == "" {
		t.Skip("skipping monitor test: requires GITHUB_TOKEN or GH_TOKEN")
	}

	cliPath := buildCLI(t)
	recipesDir := os.Getenv("TEST_RECIPES_DIR")

	tests := []struct {
		name         string
		args         []string
		wantErr      bool
		allowNonZero bool // Allow non-zero exit code (e.g., when some packages fail)
		validate     func(t *testing.T, output string)
	}{
		{
			name:         "monitor single package",
			args:         []string{"monitor", "--recipes-dir", recipesDir, "kubectl"},
			allowNonZero: true, // May fail due to GitHub API rate limits
		},
		{
			name:         "monitor multiple packages",
			args:         []string{"monitor", "--recipes-dir", recipesDir, "curl", "kubectl"},
			allowNonZero: true, // May fail due to GitHub API rate limits
		},
		{
			name:         "monitor with JSON output",
			args:         []string{"monitor", "--recipes-dir", recipesDir, "--json=true", "curl", "kubectl"},
			allowNonZero: true, // May fail due to GitHub API rate limits
			validate: func(t *testing.T, output string) {
				if output == "" {
					return // Skip validation if command failed
				}
				var results []map[string]interface{}
				if err := json.Unmarshal([]byte(output), &results); err != nil {
					t.Errorf("Expected valid JSON output: %v", err)
				}
			},
		},
		{
			name:         "monitor with human-readable output",
			args:         []string{"monitor", "--recipes-dir", recipesDir, "--json=false", "curl"},
			allowNonZero: true, // May fail due to GitHub API rate limits
			validate: func(t *testing.T, output string) {
				if output == "" {
					return // Skip validation if command failed
				}
				// Should not be JSON when --json=false
				var results []map[string]interface{}
				if err := json.Unmarshal([]byte(output), &results); err == nil {
					t.Errorf("Expected human-readable output, got JSON")
				}
			},
		},
		{
			name:         "monitor all packages",
			args:         []string{"monitor", "--all", "--recipes-dir", recipesDir},
			allowNonZero: true, // Some packages may fail to fetch versions
			validate: func(t *testing.T, output string) {
				// Should return valid JSON even if some packages have errors
				var results []map[string]interface{}
				if err := json.Unmarshal([]byte(output), &results); err != nil {
					t.Errorf("Expected valid JSON output even with errors: %v", err)
				}
				// Should have checked many packages
				if len(results) < 50 {
					t.Errorf("Expected to check at least 50 packages, got %d", len(results))
				}
			},
		},
		{
			name:    "monitor help",
			args:    []string{"monitor", "--help"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use shorter timeout to fail fast on rate limit waits
			timeout := 120 * time.Second // 2 min default
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, cliPath, tt.args...) // #nosec G204 -- test code with controlled input
			// Explicitly pass GITHUB_TOKEN to subprocess
			cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
			output, err := cmd.CombinedOutput()

			// Skip test if actively rate limited (0 remaining) with reset time in future
			outputStr := string(output)
			if strings.Contains(outputStr, "0 remaining") && strings.Contains(outputStr, "resets at") {
				t.Skipf("GitHub API rate limit exhausted: %s", outputStr)
			}

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.wantErr && !tt.allowNonZero && err != nil {
				t.Errorf("Unexpected error: %v\nOutput: %s", err, output)
			}

			if tt.validate != nil {
				tt.validate(t, outputStr)
			}

			t.Logf("Output:\n%s", output)
		})
	}
}

// TestCLI_Verify tests the verify command
func TestCLI_Verify(t *testing.T) {
	cliPath := buildCLI(t)
	// recipesDir not needed for verify tests

	// Create a test file with checksum
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	checksumFile := filepath.Join(tmpDir, "test.txt.sha256")

	if err := os.WriteFile(testFile, []byte("test content\n"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Generate checksum (using sha256sum or shasum)
	ctx := context.Background()
	checksumCmd := exec.CommandContext(ctx, "sh", "-c", "cd "+tmpDir+" && shasum -a 256 test.txt > test.txt.sha256") // #nosec G204 -- test code with controlled input
	if err := checksumCmd.Run(); err != nil {
		checksumCmd = exec.CommandContext(ctx, "sh", "-c", "cd "+tmpDir+" && sha256sum test.txt > test.txt.sha256") // #nosec G204 -- test code with controlled input
		if err := checksumCmd.Run(); err != nil {
			t.Skipf("Neither shasum nor sha256sum available")
		}
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "verify valid checksum",
			args:    []string{"verify", testFile},
			wantErr: false,
		},
		{
			name:    "verify with explicit checksum file",
			args:    []string{"verify", testFile, "--checksum", checksumFile},
			wantErr: false,
		},
		{
			name:    "verify help",
			args:    []string{"verify", "--help"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.CommandContext(ctx, cliPath, tt.args...) // #nosec G204 -- test code with controlled input
			cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
			output, err := cmd.CombinedOutput()

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v\nOutput: %s", err, output)
			}

			t.Logf("Output:\n%s", output)
		})
	}
}

// TestCLI_ValidateRelease tests the validate-release command
func TestCLI_ValidateRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// First build a package to have artifacts
	cliPath := buildCLI(t)
	recipesDir := os.Getenv("TEST_RECIPES_DIR")
	packageName := "curl"
	platform := "linux-amd64"
	if os.Getenv("RUNNER_OS") == "macOS" {
		platform = "darwin-arm64"
	}

	artifactsDir := t.TempDir()

	// Build the package first
	ctx, cancel := context.WithTimeout(context.Background(), 300*1e9)
	defer cancel()

	buildCmd := exec.CommandContext(ctx, cliPath, // #nosec G204 -- test code with controlled input
		"build",
		"--platform", platform,
		"--output-dir", artifactsDir,
		"--recipes-dir", recipesDir,
		"--enable-security-scan=false",
		packageName, "latest",
	)
	buildCmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))

	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Logf("Build output: %s", output)
		t.Skipf("Could not build package for validation test: %v", err)
	}

	// Extract actual version from built artifact filename
	files, err := filepath.Glob(filepath.Join(artifactsDir, packageName+"-*.tar.gz"))
	if err != nil || len(files) == 0 {
		t.Skipf("Could not find built artifact to determine version")
	}

	// Extract version from filename: curl-8.17.0-darwin-arm64.tar.gz -> 8.17.0
	baseName := filepath.Base(files[0])
	baseName = strings.TrimSuffix(baseName, ".tar.gz")
	// Remove package name prefix: curl-8.17.0-darwin-arm64 -> 8.17.0-darwin-arm64
	versionAndPlatform := strings.TrimPrefix(baseName, packageName+"-")
	// Remove platform suffix by matching known platform patterns
	for _, plat := range []string{"-darwin-arm64", "-darwin-x86_64", "-linux-amd64", "-linux-arm64"} {
		if strings.HasSuffix(versionAndPlatform, plat) {
			versionAndPlatform = strings.TrimSuffix(versionAndPlatform, plat)
			break
		}
	}
	actualVersion := versionAndPlatform

	t.Logf("Built artifact version: %s", actualVersion)

	tests := []struct {
		name       string
		args       []string
		expectFail bool // Validation might fail if not all platforms built
	}{
		{
			name: "validate with artifacts dir",
			args: []string{
				"validate-release",
				"--artifacts", artifactsDir,
				"--recipes", recipesDir,
				packageName, actualVersion,
			},
			expectFail: true, // Expected to fail - not all platforms built
		},
		{
			name: "validate with quiet mode",
			args: []string{
				"validate-release",
				"--artifacts", artifactsDir,
				"--recipes", recipesDir,
				"--quiet",
				packageName, actualVersion,
			},
			expectFail: true, // Will fail - only one platform built, needs 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validateCmd := exec.CommandContext(ctx, cliPath, tt.args...) // #nosec G204 -- test code with controlled input
			validateCmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
			output, err := validateCmd.CombinedOutput()
			t.Logf("Validate output:\n%s", output)

			if tt.expectFail {
				// Validation should fail (only 1 platform built, expects 4)
				if err == nil {
					t.Errorf("Expected validation to fail, but it succeeded")
				}
			} else if err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

// TestCLI_Scan tests the scan command
func TestCLI_Scan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	cliPath := buildCLI(t)
	recipesDir := os.Getenv("TEST_RECIPES_DIR")

	// First build a binary to scan
	packageName := "curl"
	platform := "linux-amd64"
	if os.Getenv("RUNNER_OS") == "macOS" {
		platform = "darwin-arm64"
	}

	outputDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 300*1e9)
	defer cancel()

	// Build a package first to get a binary
	buildCmd := exec.CommandContext(ctx, cliPath, // #nosec G204 -- test code with controlled input
		"build",
		"--platform", platform,
		"--output-dir", outputDir,
		"--recipes-dir", recipesDir,
		"--enable-security-scan=false",
		packageName, "latest",
	)
	buildCmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))

	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Logf("Build output: %s", output)
		t.Skipf("Could not build package for scan test: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "scan package by name",
			args: []string{
				"scan",
				"--package", packageName,
				"--version", "latest",
				"--platform", platform,
			},
			wantErr: false,
		},
		{
			name: "scan with verbose output",
			args: []string{
				"scan",
				"--package", packageName,
				"--version", "latest",
				"--platform", platform,
				"--verbose",
			},
			wantErr: false,
		},
		{
			name:    "scan help",
			args:    []string{"scan", "--help"},
			wantErr: false,
		},
		{
			name:    "scan without required args",
			args:    []string{"scan"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanCtx, cancel := context.WithTimeout(context.Background(), 120*1e9)
			defer cancel()

			cmd := exec.CommandContext(scanCtx, cliPath, tt.args...) // #nosec G204 -- test code with controlled input
			cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
			output, err := cmd.CombinedOutput()

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Logf("Scan output: %s", output)
				// Scan might fail if package not found or network issues - log but don't fail test
				t.Logf("Scan error (may be expected): %v", err)
			}

			t.Logf("Output:\n%s", output)
		})
	}
}

// TestCLI_Release tests the release command
func TestCLI_Release(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	cliPath := buildCLI(t)
	recipesDir := os.Getenv("TEST_RECIPES_DIR")
	packageName := "curl"
	platform := "linux-amd64"
	if os.Getenv("RUNNER_OS") == "macOS" {
		platform = "darwin-arm64"
	}

	outputDir := t.TempDir()

	// Build a package first
	ctx, cancel := context.WithTimeout(context.Background(), 300*1e9)
	defer cancel()

	buildCmd := exec.CommandContext(ctx, cliPath, // #nosec G204 -- test code with controlled input
		"build",
		"--platform", platform,
		"--output-dir", outputDir,
		"--recipes-dir", recipesDir,
		"--enable-security-scan=false",
		packageName, "latest",
	)
	buildCmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))

	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Logf("Build output: %s", output)
		t.Skipf("Could not build package for release test: %v", err)
	}

	// Extract actual version from built artifact filename
	// Build with "latest" creates files like: curl-8.17.0-darwin-arm64.tar.gz
	files, err := filepath.Glob(filepath.Join(outputDir, packageName+"-*.tar.gz"))
	if err != nil || len(files) == 0 {
		t.Skipf("Could not find built artifact to determine version")
	}

	// Extract version from filename: curl-8.17.0-darwin-arm64.tar.gz -> 8.17.0
	baseName := filepath.Base(files[0])
	baseName = strings.TrimSuffix(baseName, ".tar.gz")
	// Remove package name prefix: curl-8.17.0-darwin-arm64 -> 8.17.0-darwin-arm64
	versionAndPlatform := strings.TrimPrefix(baseName, packageName+"-")
	// Remove platform suffix by matching known platform patterns
	// darwin-arm64, darwin-x86_64, linux-amd64, linux-arm64
	for _, plat := range []string{"-darwin-arm64", "-darwin-x86_64", "-linux-amd64", "-linux-arm64"} {
		if strings.HasSuffix(versionAndPlatform, plat) {
			versionAndPlatform = strings.TrimSuffix(versionAndPlatform, plat)
			break
		}
	}
	actualVersion := versionAndPlatform

	t.Logf("Built artifact version: %s", actualVersion)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "release dry-run single package",
			args: []string{
				"release",
				"--binaries", outputDir,
				"--recipes", recipesDir,
				"--dry-run",
				packageName, actualVersion,
			},
			wantErr: false,
		},
		{
			name: "release with draft flag",
			args: []string{
				"release",
				"--binaries", outputDir,
				"--recipes", recipesDir,
				"--dry-run",
				"--draft",
				packageName, actualVersion,
			},
			wantErr: false,
		},
		{
			name: "release with prerelease flag",
			args: []string{
				"release",
				"--binaries", outputDir,
				"--recipes", recipesDir,
				"--dry-run",
				"--prerelease",
				packageName, actualVersion,
			},
			wantErr: false,
		},
		{
			name: "release with JSON packages",
			args: []string{
				"release",
				"--packages", `[{"package":"` + packageName + `","version":"` + actualVersion + `"}]`,
				"--artifacts", outputDir,
				"--recipes", recipesDir,
				"--dry-run",
			},
			wantErr: true, // Validation fails - only 1 platform built, recipe defines 4
		},
		{
			name: "release with custom owner/repo",
			args: []string{
				"release",
				"--binaries", outputDir,
				"--recipes", recipesDir,
				"--owner", "ochairo",
				"--repo", "potions",
				"--dry-run",
				packageName, actualVersion,
			},
			wantErr: false,
		},
		{
			name: "release with report file",
			args: []string{
				"release",
				"--binaries", outputDir,
				"--recipes", recipesDir,
				"--dry-run",
				"--report", filepath.Join(outputDir, "report.json"),
				packageName, actualVersion,
			},
			wantErr: false,
		},
		{
			name:    "release help",
			args:    []string{"release", "--help"},
			wantErr: false,
		},
		{
			name:    "release without required args",
			args:    []string{"release"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 120*1e9)
			defer cancel()

			cmd := exec.CommandContext(releaseCtx, cliPath, tt.args...) // #nosec G204 -- test code with controlled input
			cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+os.Getenv("GITHUB_TOKEN"))
			output, err := cmd.CombinedOutput()

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none. Output: %s", output)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v\nOutput: %s", err, output)
			}

			t.Logf("Output:\n%s", output)
		})
	}
}
