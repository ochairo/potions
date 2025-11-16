package gateways

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// ScriptExecutor handles execution of build scripts
type ScriptExecutor struct {
	defaultTimeout time.Duration
}

// NewScriptExecutor creates a new script executor
func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{
		defaultTimeout: 30 * time.Minute,
	}
}

// ExecuteScriptConfig contains configuration for executing a shell script.
type ExecuteScriptConfig struct {
	Script      string
	WorkingDir  string
	Env         map[string]string
	Timeout     time.Duration
	Description string
}

// ExecuteResult contains the result of script execution
type ExecuteResult struct {
	Success  bool
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Error    error
}

// ExecuteScript runs a shell script with the given configuration
func (se *ScriptExecutor) ExecuteScript(ctx context.Context, config ExecuteScriptConfig) *ExecuteResult {
	startTime := time.Now()
	result := &ExecuteResult{}

	// Use default timeout if not specified
	timeout := config.Timeout
	if timeout == 0 {
		timeout = se.defaultTimeout
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create shell command
	// Use /bin/sh for maximum compatibility
	//nolint:gosec // G204: Script execution is intentional and controlled by recipe configuration
	cmd := exec.CommandContext(execCtx, "/bin/sh", "-c", config.Script)

	// Set working directory
	if config.WorkingDir != "" {
		cmd.Dir = config.WorkingDir
	}

	// Build environment variables
	env := os.Environ()
	for key, value := range config.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = env

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Log execution start
	if config.Description != "" {
		fmt.Fprintf(os.Stderr, "Executing: %s\n", config.Description)
	}

	// Execute command
	err := cmd.Run()
	result.Duration = time.Since(startTime)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		result.Error = err
		var exitErr *exec.ExitError
		//nolint:gocritic // ifElseChain: checking different error types, not suitable for switch
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("script execution timeout after %v", timeout)
			result.ExitCode = -1
		} else {
			result.ExitCode = -1
		}
		return result
	}

	result.Success = true
	result.ExitCode = 0
	return result
}

// ExecuteBuildScripts executes all build-related scripts for a package
func (se *ScriptExecutor) ExecuteBuildScripts(
	ctx context.Context,
	def *entities.Recipe,
	artifact *entities.Artifact,
	outputDir string,
) error {
	// Determine working directory based on artifact type
	workingDir := artifact.Path

	// If it's an extracted tarball, the path points to the extraction directory
	// If it's a binary, the path points to the binary file itself
	if artifact.Type == "binary" && !isDirectory(artifact.Path) {
		workingDir = filepath.Dir(artifact.Path)
	}

	// Set up environment variables
	env := map[string]string{
		"PREFIX":      outputDir,
		"PACKAGE":     def.Name,
		"VERSION":     artifact.Version,
		"PLATFORM":    artifact.Platform,
		"SOURCE_DIR":  workingDir,
		"INSTALL_DIR": outputDir,
	}

	// Determine timeout
	timeout := se.defaultTimeout
	if def.Build.TimeoutMinutes > 0 {
		timeout = time.Duration(def.Build.TimeoutMinutes) * time.Minute
	}

	// Execute configure script if present
	if def.Configure.Script != "" {
		result := se.ExecuteScript(ctx, ExecuteScriptConfig{
			Script:      def.Configure.Script,
			WorkingDir:  workingDir,
			Env:         env,
			Timeout:     timeout,
			Description: "configure",
		})

		if !result.Success {
			return fmt.Errorf("configure script failed (exit %d): %w\nStderr: %s",
				result.ExitCode, result.Error, result.Stderr)
		}

		if result.Stdout != "" {
			fmt.Fprintf(os.Stderr, "Configure output: %s\n", result.Stdout)
		}
	}

	// Execute custom_build script if present
	if def.Build.CustomBuild != "" {
		result := se.ExecuteScript(ctx, ExecuteScriptConfig{
			Script:      def.Build.CustomBuild,
			WorkingDir:  workingDir,
			Env:         env,
			Timeout:     timeout,
			Description: "build",
		})

		if !result.Success {
			return fmt.Errorf("build script failed (exit %d): %w\nStderr: %s",
				result.ExitCode, result.Error, result.Stderr)
		}

		if result.Stdout != "" {
			fmt.Fprintf(os.Stderr, "Build output: %s\n", result.Stdout)
		}
	}

	// Execute custom_install script (build step)
	if def.Build.CustomInstall != "" {
		result := se.ExecuteScript(ctx, ExecuteScriptConfig{
			Script:      def.Build.CustomInstall,
			WorkingDir:  workingDir,
			Env:         env,
			Timeout:     timeout,
			Description: "build/install",
		})

		if !result.Success {
			return fmt.Errorf("build/install script failed (exit %d): %w\nStderr: %s",
				result.ExitCode, result.Error, result.Stderr)
		}

		if result.Stdout != "" {
			fmt.Fprintf(os.Stderr, "Build output: %s\n", result.Stdout)
		}

		fmt.Fprintf(os.Stderr, "Build completed in %v\n", result.Duration)
	}

	return nil
}

// isDirectory checks if a path is a directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ValidateScript performs basic validation on a shell script
func (se *ScriptExecutor) ValidateScript(script string) error {
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("script is empty")
	}

	// Check for potentially dangerous commands (basic security check)
	dangerous := []string{
		"rm -rf /",
		"mkfs",
		"dd if=/dev/zero",
		":(){:|:&};:", // fork bomb
	}

	for _, pattern := range dangerous {
		if strings.Contains(script, pattern) {
			return fmt.Errorf("script contains potentially dangerous pattern: %s", pattern)
		}
	}

	return nil
}
