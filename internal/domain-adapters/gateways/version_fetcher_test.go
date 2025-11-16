package gateways

import (
	"os"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

func TestVersionFetcher_ExtractVersion(t *testing.T) {
	vf := NewVersionFetcher()

	tests := []struct {
		name    string
		input   string
		pattern string
		want    string
		wantErr bool
	}{
		{
			name:    "extract version with v prefix",
			input:   "v1.28.3\n",
			pattern: `v[0-9]+\.[0-9]+\.[0-9]+`,
			want:    "v1.28.3",
			wantErr: false,
		},
		{
			name:    "extract version without prefix",
			input:   "Current version: 3.14.0",
			pattern: `[0-9]+\.[0-9]+\.[0-9]+`,
			want:    "3.14.0",
			wantErr: false,
		},
		{
			name:    "no match",
			input:   "No version here",
			pattern: `[0-9]+\.[0-9]+\.[0-9]+`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid regex",
			input:   "v1.0.0",
			pattern: `[invalid(`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vf.extractVersion(tt.input, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionFetcher_TransformVersion(t *testing.T) {
	vf := NewVersionFetcher()

	tests := []struct {
		name       string
		input      string
		sedPattern string
		want       string
		wantErr    bool
	}{
		{
			name:       "remove v prefix",
			input:      "v1.28.3",
			sedPattern: "s/^v//",
			want:       "1.28.3",
			wantErr:    false,
		},
		{
			name:       "replace dots with dashes",
			input:      "1.2.3",
			sedPattern: `s/\./-/g`,
			want:       "1-2-3",
			wantErr:    false,
		},
		{
			name:       "replace first dot only",
			input:      "1.2.3",
			sedPattern: `s/\./x/`,
			want:       "1x2.3",
			wantErr:    false,
		},
		{
			name:       "invalid sed pattern - no s/ prefix",
			input:      "1.0.0",
			sedPattern: "invalid",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "invalid sed pattern - malformed",
			input:      "1.0.0",
			sedPattern: "s/only-one-part",
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vf.transformVersion(tt.input, tt.sedPattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("transformVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionFetcher_ShouldFilterVersion(t *testing.T) {
	vf := NewVersionFetcher()

	tests := []struct {
		name          string
		version       string
		filterPattern string
		want          bool
	}{
		{
			name:          "filter alpha version",
			version:       "1.0.0-alpha",
			filterPattern: "(alpha|beta|rc)",
			want:          true,
		},
		{
			name:          "filter beta version",
			version:       "2.1.0-beta.1",
			filterPattern: "(alpha|beta|rc)",
			want:          true,
		},
		{
			name:          "allow stable version",
			version:       "1.28.3",
			filterPattern: "(alpha|beta|rc)",
			want:          false,
		},
		{
			name:          "filter nightly build",
			version:       "1.0.0-nightly-20231110",
			filterPattern: "(nightly|snapshot|dev)",
			want:          true,
		},
		{
			name:          "invalid regex - don't filter",
			version:       "1.0.0",
			filterPattern: "[invalid(",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vf.shouldFilterVersion(tt.version, tt.filterPattern)
			if got != tt.want {
				t.Errorf("shouldFilterVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionFetcher_FetchLatestVersion_URL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	vf := NewVersionFetcher()

	// Test with kubectl's stable.txt endpoint
	def := &entities.Recipe{
		Name: "kubectl",
		Version: entities.VersionConfig{
			Source:         "url:https://dl.k8s.io/release/stable.txt",
			ExtractPattern: `v[0-9]+\.[0-9]+\.[0-9]+`,
			Cleanup:        "s/^v//",
		},
	}

	version, err := vf.FetchLatestVersion(def)
	if err != nil {
		t.Fatalf("FetchLatestVersion() error = %v", err)
	}

	if version == "" {
		t.Error("FetchLatestVersion() returned empty version")
	}

	// Version should be in format X.Y.Z (no 'v' prefix due to transform)
	if version[0] == 'v' {
		t.Errorf("FetchLatestVersion() = %v, should not have 'v' prefix after transform", version)
	}

	t.Logf("kubectl latest version: %s", version)
}

func TestVersionFetcher_FetchLatestVersion_GitHubRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if no GitHub token (to avoid rate limiting)
	if os.Getenv("GITHUB_TOKEN") == "" && os.Getenv("GH_TOKEN") == "" {
		t.Skip("skipping GitHub API test: no GITHUB_TOKEN or GH_TOKEN set")
	}

	vf := NewVersionFetcher()

	// Test with helm GitHub releases
	def := &entities.Recipe{
		Name: "helm",
		Version: entities.VersionConfig{
			Source:         "github-release:helm/helm",
			ExtractPattern: `[0-9]+\.[0-9]+\.[0-9]+`,
		},
	}

	version, err := vf.FetchLatestVersion(def)
	if err != nil {
		t.Fatalf("FetchLatestVersion() error = %v", err)
	}

	if version == "" {
		t.Error("FetchLatestVersion() returned empty version")
	}

	t.Logf("helm latest version: %s", version)
}

func TestVersionFetcher_FetchLatestVersion_GitHubTag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if no GitHub token (to avoid rate limiting)
	if os.Getenv("GITHUB_TOKEN") == "" && os.Getenv("GH_TOKEN") == "" {
		t.Skip("skipping GitHub API test: no GITHUB_TOKEN or GH_TOKEN set")
	}

	vf := NewVersionFetcher()

	// Test with a repository that uses tags
	def := &entities.Recipe{
		Name: "test-tags",
		Version: entities.VersionConfig{
			Source: "github-tag:FiloSottile/age",
		},
	}

	version, err := vf.FetchLatestVersion(def)
	if err != nil {
		t.Fatalf("FetchLatestVersion() error = %v", err)
	}

	if version == "" {
		t.Error("FetchLatestVersion() returned empty version")
	}

	t.Logf("age latest tag: %s", version)
}

func TestVersionFetcher_FetchLatestVersion_FilterPrerelease(t *testing.T) {
	vf := NewVersionFetcher()

	tests := []struct {
		name    string
		def     *entities.Recipe
		wantErr bool
	}{
		{
			name: "no version source",
			def: &entities.Recipe{
				Name: "test",
			},
			wantErr: true,
		},
		{
			name: "unsupported version source",
			def: &entities.Recipe{
				Name: "test",
				Version: entities.VersionConfig{
					Source: "invalid:something",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := vf.FetchLatestVersion(tt.def)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchLatestVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
