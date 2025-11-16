// Package gpg provides GPG signature verification capabilities.
package gpg

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Verifier implements GPG signature verification using ProtonMail's go-crypto
// A maintained, modern fork of golang.org/x/crypto/openpgp
// This is in external-adapters to isolate the external dependency
type Verifier struct {
	keyring    openpgp.EntityList
	httpClient *http.Client
}

// NewVerifier creates a new GPG verifier
func NewVerifier() *Verifier {
	return &Verifier{
		keyring: make(openpgp.EntityList, 0),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ImportKeys imports GPG keys from a keyserver
func (v *Verifier) ImportKeys(ctx context.Context, keyIDs []string) error {
	if len(keyIDs) == 0 {
		return fmt.Errorf("no key IDs provided")
	}

	keyserver := "https://keys.openpgp.org"

	for _, keyID := range keyIDs {
		if keyID == "" {
			continue
		}

		// Try different keyserver endpoints
		urls := []string{
			fmt.Sprintf("%s/vks/v1/by-fingerprint/%s", keyserver, keyID),
			fmt.Sprintf("%s/pks/lookup?op=get&search=0x%s", keyserver, keyID),
		}

		var lastErr error
		imported := false

		for _, url := range urls {
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				lastErr = err
				continue
			}

			resp, err := v.httpClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}

			if resp.StatusCode != http.StatusOK {
				_ = resp.Body.Close()
				lastErr = fmt.Errorf("keyserver returned status %d", resp.StatusCode)
				continue
			}

			// Try to parse the key
			entities, err := openpgp.ReadArmoredKeyRing(resp.Body)
			_ = resp.Body.Close()

			if err != nil {
				lastErr = err
				continue
			}

			if len(entities) == 0 {
				lastErr = fmt.Errorf("no keys found in response")
				continue
			}

			// Successfully imported
			v.keyring = append(v.keyring, entities...)
			imported = true
			break
		}

		if !imported {
			return fmt.Errorf("failed to import key %s: %w", keyID, lastErr)
		}
	}

	return nil
}

// ImportKeyFromFile imports a GPG key from a file
func (v *Verifier) ImportKeyFromFile(keyPath string) error {
	//nolint:gosec // G304: keyPath is user-provided for GPG key import
	f, err := os.Open(keyPath)
	if err != nil {
		return fmt.Errorf("failed to open key file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	entities, err := openpgp.ReadArmoredKeyRing(f)
	if err != nil {
		// Try reading as binary
		if _, seekErr := f.Seek(0, 0); seekErr != nil {
			return fmt.Errorf("failed to reset file: %w", seekErr)
		}
		entities, err = openpgp.ReadKeyRing(f)
		if err != nil {
			return fmt.Errorf("failed to read key: %w", err)
		}
	}

	if len(entities) == 0 {
		return fmt.Errorf("no keys found in file")
	}

	v.keyring = append(v.keyring, entities...)
	return nil
}

// VerifySignature verifies a detached GPG signature
func (v *Verifier) VerifySignature(ctx context.Context, filePath, sigURL string) error {
	if len(v.keyring) == 0 {
		return fmt.Errorf("no GPG keys imported, call ImportKeys first")
	}

	// Download signature
	req, err := http.NewRequestWithContext(ctx, "GET", sigURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create signature download request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download signature: %w", err)
	}
	//nolint:errcheck // Defer close
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("signature download failed with status %d", resp.StatusCode)
	}

	// Read signature into memory
	sigData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read signature: %w", err)
	}

	// Open the file to verify
	//nolint:gosec // G304: filePath is user-provided for GPG verification
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	// Create reader for signature data
	sigReader := &sigReader{data: sigData}

	// Verify the signature (ProtonMail go-crypto API)
	_, err = openpgp.CheckDetachedSignature(v.keyring, f, sigReader, nil)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// VerifySignatureFromFile verifies a detached signature from a local file
func (v *Verifier) VerifySignatureFromFile(filePath, sigPath string) error {
	if len(v.keyring) == 0 {
		return fmt.Errorf("no GPG keys imported, call ImportKeys first")
	}

	// Open signature file
	//nolint:gosec // G304: sigPath is user-provided for GPG verification
	sigFile, err := os.Open(sigPath)
	if err != nil {
		return fmt.Errorf("failed to open signature file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer sigFile.Close()

	// Open data file
	//nolint:gosec // G304: filePath is user-provided for GPG verification
	dataFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open data file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer dataFile.Close()

	// Verify signature (ProtonMail go-crypto API)
	_, err = openpgp.CheckDetachedSignature(v.keyring, dataFile, sigFile, nil)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// GetKeyringSize returns the number of keys in the keyring
func (v *Verifier) GetKeyringSize() int {
	return len(v.keyring)
}

// ClearKeyring clears all imported keys
func (v *Verifier) ClearKeyring() {
	v.keyring = make(openpgp.EntityList, 0)
}

// sigReader is a helper to read signature data
type sigReader struct {
	data []byte
	pos  int
}

func (r *sigReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	n = copy(p, r.data[r.pos:])
	r.pos += n

	if r.pos >= len(r.data) {
		return n, io.EOF
	}

	return n, nil
}
