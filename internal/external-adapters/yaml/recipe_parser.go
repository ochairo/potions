// Package yaml provides YAML-based recipe parsing and repository implementations.
package yaml

import (
	"fmt"
	"os"

	"github.com/ochairo/potions/internal/domain/entities"
	"gopkg.in/yaml.v3"
)

// yamlRecipe represents the raw YAML structure
type yamlRecipe struct {
	Name         string        `yaml:"name"`
	Version      yamlVersion   `yaml:"version"`
	BuildType    string        `yaml:"build_type"`
	Description  string        `yaml:"description"`
	Download     yamlDownload  `yaml:"download"`
	Security     yamlSecurity  `yaml:"security"`
	Configure    yamlBuildStep `yaml:"configure"`
	Build        yamlBuildStep `yaml:"build"`
	Dependencies []string      `yaml:"dependencies"`
}

type yamlVersion struct {
	Source          string `yaml:"source"`
	ExcludePatterns string `yaml:"exclude_patterns"`
	ExtractPattern  string `yaml:"extract_pattern"`
	Cleanup         string `yaml:"cleanup"`
}

type yamlDownload struct {
	OfficialBinary bool                          `yaml:"official_binary"`
	DownloadURL    string                        `yaml:"download_url"`
	Method         string                        `yaml:"method"`
	GitURL         string                        `yaml:"git_url"`
	GitTagPrefix   string                        `yaml:"git_tag_prefix"`
	Platforms      map[string]yamlPlatformConfig `yaml:"platforms"`
}

type yamlPlatformConfig struct {
	OS     string `yaml:"os"`
	Arch   string `yaml:"arch"`
	Suffix string `yaml:"suffix"`
	// Inline map captures any additional custom fields (e.g., target, triple)
	Custom map[string]string `yaml:",inline"`
}

type yamlSecurity struct {
	VerifySignature     bool     `yaml:"verify_signature"`
	ScanVulnerabilities bool     `yaml:"scan_vulnerabilities"`
	GPGKeyIDs           []string `yaml:"gpg_key_ids"`
	GPGKeysURL          string   `yaml:"gpg_keys_url"`
	SignatureURL        string   `yaml:"signature_url"`
}

type yamlBuildStep struct {
	Script         string `yaml:"script"`
	TimeoutMinutes int    `yaml:"timeout_minutes"`
	OutOfTree      bool   `yaml:"out_of_tree"`
	CustomBuild    string `yaml:"custom_build"`
	CustomInstall  string `yaml:"custom_install"`
}

// RecipeParser parses YAML recipe files
type RecipeParser struct{}

// NewRecipeParser creates a new YAML parser
func NewRecipeParser() *RecipeParser {
	return &RecipeParser{}
}

// ParseFile parses a YAML recipe file into a Recipe entity
func (p *RecipeParser) ParseFile(filePath string) (*entities.Recipe, error) {
	//nolint:gosec // G304: filePath is recipe definition path from repository
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return p.Parse(data)
}

// Parse parses YAML bytes into a Recipe entity
func (p *RecipeParser) Parse(data []byte) (*entities.Recipe, error) {
	var yamlDef yamlRecipe
	if err := yaml.Unmarshal(data, &yamlDef); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	if yamlDef.Name == "" {
		return nil, fmt.Errorf("recipe must have a name")
	}

	// Convert to domain entity
	def := &entities.Recipe{
		Name:         yamlDef.Name,
		Version:      convertVersion(yamlDef.Version),
		BuildType:    yamlDef.BuildType,
		Description:  yamlDef.Description,
		Download:     convertDownload(yamlDef.Download),
		Security:     convertSecurity(yamlDef.Security),
		Configure:    convertBuildStep(yamlDef.Configure),
		Build:        convertBuildStep(yamlDef.Build),
		Dependencies: yamlDef.Dependencies,
	}

	return def, nil
}

func convertVersion(yv yamlVersion) entities.VersionConfig {
	return entities.VersionConfig{
		Source:          yv.Source,
		ExcludePatterns: yv.ExcludePatterns,
		ExtractPattern:  yv.ExtractPattern,
		Cleanup:         yv.Cleanup,
	}
}

func convertDownload(yd yamlDownload) entities.RecipeDownload {
	platforms := make(map[string]entities.PlatformConfig)
	for name, cfg := range yd.Platforms {
		// Extract custom fields (exclude known fields: os, arch, suffix)
		custom := make(map[string]string)
		for k, v := range cfg.Custom {
			if k != "os" && k != "arch" && k != "suffix" {
				custom[k] = v
			}
		}

		platforms[name] = entities.PlatformConfig{
			OS:     cfg.OS,
			Arch:   cfg.Arch,
			Suffix: cfg.Suffix,
			Custom: custom,
		}
	}

	return entities.RecipeDownload{
		OfficialBinary: yd.OfficialBinary,
		DownloadURL:    yd.DownloadURL,
		Method:         yd.Method,
		GitURL:         yd.GitURL,
		GitTagPrefix:   yd.GitTagPrefix,
		Platforms:      platforms,
	}
}

func convertSecurity(ys yamlSecurity) entities.RecipeSecurity {
	return entities.RecipeSecurity{
		VerifySignature:     ys.VerifySignature,
		ScanVulnerabilities: ys.ScanVulnerabilities,
		GPGKeyIDs:           ys.GPGKeyIDs,
		GPGKeysURL:          ys.GPGKeysURL,
		SignatureURL:        ys.SignatureURL,
	}
}

func convertBuildStep(yb yamlBuildStep) entities.RecipeBuildStep {
	return entities.RecipeBuildStep{
		Script:         yb.Script,
		TimeoutMinutes: yb.TimeoutMinutes,
		OutOfTree:      yb.OutOfTree,
		CustomBuild:    yb.CustomBuild,
		CustomInstall:  yb.CustomInstall,
	}
}
