package entities

// Recipe represents a software package recipe from YAML
type Recipe struct {
	Name         string
	Version      VersionConfig
	BuildType    string
	Description  string
	Download     RecipeDownload
	Security     RecipeSecurity
	Configure    RecipeBuildStep
	Build        RecipeBuildStep
	Dependencies []string
}

// VersionConfig represents version fetching and processing configuration
type VersionConfig struct {
	Source          string // e.g., "github-release:owner/repo", "url:https://...", "static:latest"
	ExcludePatterns string // Regex patterns to exclude (alpha, beta, rc, etc.)
	ExtractPattern  string // Regex to extract version from tag/response
	Cleanup         string // Sed-like pattern or simple find:replace to clean up version
}

// RecipeDownload represents download configuration
type RecipeDownload struct {
	OfficialBinary bool
	DownloadURL    string
	Platforms      map[string]PlatformConfig
}

// PlatformConfig represents platform-specific configuration
type PlatformConfig struct {
	OS     string
	Arch   string
	Suffix string // Platform-specific suffix for download URLs
}

// RecipeSecurity represents security configuration
type RecipeSecurity struct {
	VerifySignature     bool
	ScanVulnerabilities bool
	GPGKeyIDs           []string
}

// RecipeBuildStep represents a build or configure step
type RecipeBuildStep struct {
	Script         string
	TimeoutMinutes int
	OutOfTree      bool
	CustomBuild    string
	CustomInstall  string
}
