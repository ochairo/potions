package yaml

import (
	"testing"
)

func TestRecipeParser_Parse_Valid(t *testing.T) {
	parser := NewRecipeParser()
	yamlData := []byte(`name: kubectl
build_type: official_binary
description: Kubernetes CLI
version:
  source: github:kubernetes/kubernetes
  extract_pattern: 'v(\d+\.\d+\.\d+)$'
  cleanup: s/^v//
download:
  official_binary: true
  download_url: https://dl.k8s.io/release/\${VERSION}/bin/\${OS}/\${ARCH}/kubectl
  platforms:
    linux-amd64:
      os: linux
      arch: amd64
    darwin-arm64:
      os: darwin
      arch: arm64
security:
  verify_signature: true
  scan_vulnerabilities: true
  gpg_key_ids:
    - ABCD1234
`)

	recipe, err := parser.Parse(yamlData)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if recipe.Name != "kubectl" {
		t.Errorf("Name = %v, want kubectl", recipe.Name)
	}
	if recipe.BuildType != "official_binary" {
		t.Errorf("BuildType = %v, want official_binary", recipe.BuildType)
	}
	if recipe.Version.Source != "github:kubernetes/kubernetes" {
		t.Errorf("Version.Source = %v, want github:kubernetes/kubernetes", recipe.Version.Source)
	}
	if !recipe.Security.VerifySignature {
		t.Error("Security.VerifySignature should be true")
	}
	if len(recipe.Download.Platforms) != 2 {
		t.Errorf("Platforms count = %d, want 2", len(recipe.Download.Platforms))
	}
}

func TestRecipeParser_Parse_MissingName(t *testing.T) {
	parser := NewRecipeParser()
	yamlData := []byte(`build_type: official_binary
description: Test package
`)

	_, err := parser.Parse(yamlData)
	if err == nil {
		t.Error("Parse() should return error for missing name")
	}
	if err != nil && err.Error() != "recipe must have a name" {
		t.Errorf("Parse() error = %v, want 'recipe must have a name'", err)
	}
}

func TestRecipeParser_Parse_InvalidYAML(t *testing.T) {
	parser := NewRecipeParser()
	yamlData := []byte(`name: test
  invalid: [broken yaml
`)

	_, err := parser.Parse(yamlData)
	if err == nil {
		t.Error("Parse() should return error for invalid YAML")
	}
}

func TestRecipeParser_Parse_EmptyPlatforms(t *testing.T) {
	parser := NewRecipeParser()
	yamlData := []byte(`name: test-package
build_type: official_binary
download:
  official_binary: true
  download_url: https://example.com/\${VERSION}/binary
  platforms: {}
`)

	recipe, err := parser.Parse(yamlData)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(recipe.Download.Platforms) != 0 {
		t.Errorf("Platforms should be empty, got %d", len(recipe.Download.Platforms))
	}
}

func TestRecipeParser_Parse_WithBuildSteps(t *testing.T) {
	parser := NewRecipeParser()
	yamlData := []byte(`name: nginx
build_type: source_build
configure:
  script: ./configure --prefix=/usr/local
  timeout_minutes: 10
  out_of_tree: false
build:
  script: make && make install
  timeout_minutes: 30
  custom_install: cp nginx /usr/local/bin/
download:
  platforms:
    linux-amd64:
      os: linux
      arch: amd64
`)

	recipe, err := parser.Parse(yamlData)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if recipe.Configure.Script != "./configure --prefix=/usr/local" {
		t.Errorf("Configure.Script = %v", recipe.Configure.Script)
	}
	if recipe.Configure.TimeoutMinutes != 10 {
		t.Errorf("Configure.TimeoutMinutes = %d, want 10", recipe.Configure.TimeoutMinutes)
	}
	if recipe.Build.Script != "make && make install" {
		t.Errorf("Build.Script = %v", recipe.Build.Script)
	}
	if recipe.Build.CustomInstall != "cp nginx /usr/local/bin/" {
		t.Errorf("Build.CustomInstall = %v", recipe.Build.CustomInstall)
	}
}

func TestRecipeParser_Parse_WithDependencies(t *testing.T) {
	parser := NewRecipeParser()
	yamlData := []byte(`name: complex-app
build_type: source_build
dependencies:
  - gcc
  - make
  - openssl
download:
  platforms:
    linux-amd64:
      os: linux
      arch: amd64
`)

	recipe, err := parser.Parse(yamlData)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(recipe.Dependencies) != 3 {
		t.Errorf("Dependencies count = %d, want 3", len(recipe.Dependencies))
	}
	expectedDeps := map[string]bool{"gcc": true, "make": true, "openssl": true}
	for _, dep := range recipe.Dependencies {
		if !expectedDeps[dep] {
			t.Errorf("Unexpected dependency: %s", dep)
		}
	}
}

func TestRecipeParser_ParseFile_NotFound(t *testing.T) {
	parser := NewRecipeParser()
	_, err := parser.ParseFile("/nonexistent/path/test.yml")
	if err == nil {
		t.Error("ParseFile() should return error for nonexistent file")
	}
}
