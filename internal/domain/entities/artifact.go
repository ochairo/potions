// Package entities defines core domain models and data structures.
package entities

// Artifact represents a software artifact to be built or analyzed
type Artifact struct {
	Name         string
	Version      string
	Platform     string
	Path         string // Working directory path (extracted or downloaded file)
	DownloadPath string // Original downloaded file path (for GPG verification)
	Type         string // "binary", "source", "archive", etc.
}
