package entities

import "time"

// BinaryAnalysis represents security analysis results for a binary
type BinaryAnalysis struct {
	Platform          string
	HardeningFeatures HardeningFeatures
	SecurityScore     SecurityScore
	Timestamp         time.Time
}

// HardeningFeatures represents security hardening features detected in a binary
type HardeningFeatures struct {
	PIEEnabled      bool   // Position Independent Executable
	StackCanaries   bool   // Stack canary protection
	RELRO           string // "full", "partial", "disabled" - RELocation Read-Only
	NXBit           bool   // No-eXecute bit (non-executable stack)
	CodeSigned      bool   // Code signing (macOS)
	HardenedRuntime bool   // Hardened runtime (macOS)
	FortifySource   bool   // FORTIFY_SOURCE (Linux)
}

// SecurityScore represents a calculated security score for a binary
type SecurityScore struct {
	Score      float64 // 0.0-10.0
	Total      int     // Total number of checks
	Passed     int     // Number of checks passed
	Percentage int     // Percentage of checks passed
}
