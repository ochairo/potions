package entities

// SecurityReport represents the result of a security vulnerability scan
type SecurityReport struct {
	Vulnerabilities []Vulnerability
	Score           float64
	ScanDate        string
	Metadata        ScanMetadata
}

// Vulnerability represents a single security vulnerability
type Vulnerability struct {
	ID          string
	Severity    string // CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN
	Description string
	Score       float64 // CVSS score (0.0-10.0)
	Component   string
	FixedIn     string // Version where vulnerability is fixed (optional)
}

// ScanMetadata contains information about the scan execution
type ScanMetadata struct {
	Scanner        string
	ScannerVersion string
	Duration       string
}
