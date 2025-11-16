package entities

import "time"

// SecurityAttestation represents a security attestation following SLSA provenance format
type SecurityAttestation struct {
	Version       string
	Timestamp     time.Time
	Subject       AttestationSubject
	PredicateType string
	Predicate     AttestationPredicate
}

// AttestationSubject identifies what is being attested
type AttestationSubject struct {
	Name   string
	Digest DigestSet
}

// DigestSet contains cryptographic digests of the subject
type DigestSet struct {
	SHA256 string
}

// AttestationPredicate contains the attestation claims
type AttestationPredicate struct {
	BuildType           string
	VerificationSummary VerificationSummary
	HardeningFeatures   *HardeningFeatures // Optional
	BuildMetadata       BuildMetadata
}

// VerificationSummary summarizes security verification results
type VerificationSummary struct {
	SupplyChainVerified bool
	HardeningAnalyzed   bool
	ChecksumVerified    bool
	VersionValidated    bool
}

// BuildMetadata contains metadata about the build process
type BuildMetadata struct {
	Builder        string
	BuildID        string
	BuildTimestamp time.Time
	SourceCommit   string
}
