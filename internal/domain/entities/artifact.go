// Package entities defines core domain models and data structures.
package entities

// Artifact represents a software artifact to be built or analyzed
type Artifact struct {
	Name     string
	Version  string
	Platform string
	Path     string
	Type     string // "binary", "source", "archive", etc.
}
