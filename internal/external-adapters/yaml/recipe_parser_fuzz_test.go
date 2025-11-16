package yaml

import (
	"testing"
)

// FuzzRecipeParser tests the YAML parser against random/malformed inputs
// to detect crashes, panics, or unexpected behavior.
//
// Run with: go test -fuzz=FuzzRecipeParser -fuzztime=30s
func FuzzRecipeParser(f *testing.F) {
	// Seed corpus with valid YAML examples
	f.Add([]byte(`name: test
build_type: official_binary
download:
  platforms:
    linux-amd64:
      os: linux
      arch: amd64
`))

	f.Add([]byte(`name: kubectl
build_type: official_binary
description: Kubernetes CLI
version:
  source: github:kubernetes/kubernetes
  extract_pattern: 'v(\d+\.\d+\.\d+)$'
  cleanup: s/^v//
download:
  official_binary: true
  download_url: https://dl.k8s.io/release/${VERSION}/bin/${OS}/${ARCH}/kubectl
  platforms:
    linux-amd64:
      os: linux
      arch: amd64
security:
  verify_signature: true
  scan_vulnerabilities: true
  gpg_key_ids:
    - ABCD1234
`))

	f.Add([]byte(`name: nginx
build_type: source_build
configure:
  script: ./configure --prefix=/usr/local
  timeout_minutes: 10
build:
  script: make && make install
  timeout_minutes: 30
download:
  platforms:
    linux-amd64:
      os: linux
      arch: amd64
dependencies:
  - gcc
  - make
`))

	// Seed with edge cases
	f.Add([]byte(``))                            // Empty input
	f.Add([]byte(`name: ""` + "\n"))             // Empty name
	f.Add([]byte(`{}`))                          // Empty JSON-style YAML
	f.Add([]byte(`[]`))                          // Array instead of object
	f.Add([]byte(`name: test\n  bad`))           // Invalid indentation
	f.Add([]byte(`name: test\nname: duplicate`)) // Duplicate keys

	parser := NewRecipeParser()

	f.Fuzz(func(_ *testing.T, data []byte) {
		// The parser should handle any input without crashing
		// We don't care if it returns an error - just that it doesn't panic
		_, _ = parser.Parse(data)
	})
}
