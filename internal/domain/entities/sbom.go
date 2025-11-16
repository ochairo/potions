package entities

import "time"

// SBOM represents a Software Bill of Materials
type SBOM struct {
	BOMFormat   string // "CycloneDX"
	SpecVersion string // "1.4"
	Version     int
	Components  []Component
	Metadata    Metadata
}

// Component represents a software component in the SBOM
type Component struct {
	Type    string // "application", "library", "framework", etc.
	Name    string
	Version string
	Hashes  []Hash
}

// Hash represents a cryptographic hash of a component
type Hash struct {
	Algorithm string // "SHA256", "SHA512", etc.
	Value     string
}

// Metadata contains SBOM generation metadata
type Metadata struct {
	Timestamp time.Time
	Tools     []Tool
}

// Tool represents a tool used to generate the SBOM
type Tool struct {
	Name    string
	Version string
}
