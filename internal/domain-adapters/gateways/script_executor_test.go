package gateways

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

func TestScriptExecutor_ExecuteScript_Success(t *testing.T) {
	se := NewScriptExecutor()

	result := se.ExecuteScript(context.Background(), ExecuteScriptConfig{
		Script:      "echo 'Hello, World!'",
		Description: "test echo",
	})

	if !result.Success {
		t.Errorf("ExecuteScript() failed: %v", result.Error)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExecuteScript() exit code = %d, want 0", result.ExitCode)
	}

	if result.Stdout != "Hello, World!\n" {
		t.Errorf("ExecuteScript() stdout = %q, want %q", result.Stdout, "Hello, World!\n")
	}
}

func TestScriptExecutor_ExecuteScript_Failure(t *testing.T) {
	se := NewScriptExecutor()

	result := se.ExecuteScript(context.Background(), ExecuteScriptConfig{
		Script:      "exit 42",
		Description: "test failure",
	})

	if result.Success {
		t.Error("ExecuteScript() should have failed")
	}

	if result.ExitCode != 42 {
		t.Errorf("ExecuteScript() exit code = %d, want 42", result.ExitCode)
	}
}

func TestScriptExecutor_ExecuteScript_WithEnvironment(t *testing.T) {
	se := NewScriptExecutor()

	result := se.ExecuteScript(context.Background(), ExecuteScriptConfig{
		Script: "echo $TEST_VAR",
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
		Description: "test env vars",
	})

	if !result.Success {
		t.Errorf("ExecuteScript() failed: %v", result.Error)
	}

	if result.Stdout != "test_value\n" {
		t.Errorf("ExecuteScript() stdout = %q, want %q", result.Stdout, "test_value\n")
	}
}

func TestScriptExecutor_ExecuteScript_Timeout(t *testing.T) {
	se := NewScriptExecutor()

	result := se.ExecuteScript(context.Background(), ExecuteScriptConfig{
		Script:      "sleep 5",
		Timeout:     100 * time.Millisecond,
		Description: "test timeout",
	})

	if result.Success {
		t.Error("ExecuteScript() should have timed out")
	}

	if result.Error == nil {
		t.Error("ExecuteScript() should have returned an error")
	}
}

func TestScriptExecutor_ExecuteScript_WorkingDirectory(t *testing.T) {
	se := NewScriptExecutor()
	tempDir := t.TempDir()

	// Create a test file in the temp directory
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result := se.ExecuteScript(context.Background(), ExecuteScriptConfig{
		Script:      "ls test.txt",
		WorkingDir:  tempDir,
		Description: "test working directory",
	})

	if !result.Success {
		t.Errorf("ExecuteScript() failed: %v", result.Error)
	}

	if result.Stdout != "test.txt\n" {
		t.Errorf("ExecuteScript() stdout = %q, want %q", result.Stdout, "test.txt\n")
	}
}

func TestScriptExecutor_ExecuteBuildScripts(t *testing.T) {
	se := NewScriptExecutor()

	// Create temporary directories
	workDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a mock binary file
	binaryPath := filepath.Join(workDir, "test-binary")
	//nolint:gosec // G306: Test executable script needs 0700 permissions
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0700); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	def := &entities.Recipe{
		Name: "test-package",
		Configure: entities.RecipeBuildStep{
			Script: "echo 'Configuring...'",
		},
		Build: entities.RecipeBuildStep{
			CustomInstall: `
				mkdir -p $PREFIX/bin
				cp test-binary $PREFIX/bin/
			`,
			TimeoutMinutes: 1,
		},
	}

	artifact := &entities.Artifact{
		Name:     "test-package",
		Version:  "1.0.0",
		Platform: "linux-amd64",
		Path:     workDir,
		Type:     "binary",
	}

	err := se.ExecuteBuildScripts(context.Background(), def, artifact, outputDir)
	if err != nil {
		t.Errorf("ExecuteBuildScripts() error = %v", err)
	}

	// Verify the binary was copied
	installedBinary := filepath.Join(outputDir, "bin", "test-binary")
	if _, err := os.Stat(installedBinary); os.IsNotExist(err) {
		t.Errorf("Binary was not installed at %s", installedBinary)
	}
}

func TestScriptExecutor_ValidateScript(t *testing.T) {
	se := NewScriptExecutor()

	tests := []struct {
		name    string
		script  string
		wantErr bool
	}{
		{
			name:    "valid script",
			script:  "echo 'hello'",
			wantErr: false,
		},
		{
			name:    "empty script",
			script:  "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			script:  "   \n  \t  ",
			wantErr: true,
		},
		{
			name:    "dangerous rm -rf /",
			script:  "rm -rf /",
			wantErr: true,
		},
		{
			name:    "dangerous mkfs",
			script:  "mkfs /dev/sda",
			wantErr: true,
		},
		{
			name:    "fork bomb",
			script:  ":(){:|:&};:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := se.ValidateScript(tt.script)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
