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

// ImportKeys imports GPG keys from a keyserver with fallbacks
func (v *Verifier) ImportKeys(ctx context.Context, keyIDs []string) error {
	if len(keyIDs) == 0 {
		return fmt.Errorf("no key IDs provided")
	}

	// Multiple keyserver fallbacks for redundancy
	keyservers := []string{
		"https://keys.openpgp.org",
		"https://keyserver.ubuntu.com",
		"https://pgp.mit.edu",
	}

	for _, keyID := range keyIDs {
		if keyID == "" {
			continue
		}

		var lastErr error
		imported := false

		// Try each keyserver until one succeeds
		for _, keyserver := range keyservers {
			// Try different keyserver endpoints
			urls := []string{
				fmt.Sprintf("%s/vks/v1/by-fingerprint/%s", keyserver, keyID),
				fmt.Sprintf("%s/pks/lookup?op=get&search=0x%s", keyserver, keyID),
			}

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

				// Security: Verify key fingerprint matches requested ID
				validKey := false
				for _, entity := range entities {
					fingerprint := fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint)
					// Check both full fingerprint and short form (last 16 chars = 8 bytes)
					if fingerprint == keyID || (len(fingerprint) >= 16 && fingerprint[len(fingerprint)-16:] == keyID) {
						// Key found - signature verification will fail if key is expired
						validKey = true
					}
				}

				if !validKey {
					lastErr = fmt.Errorf("no valid keys found matching fingerprint %s", keyID)
					continue
				}

				// Successfully imported and verified
				v.keyring = append(v.keyring, entities...)
				imported = true
				break
			}

			if imported {
				break
			}
		}

		if !imported {
			return fmt.Errorf("failed to import key %s from all keyservers: %w", keyID, lastErr)
		}
	}

	return nil
}

// ImportKeysFromURL imports all GPG keys from a KEYS file URL
// This is commonly used by projects like Apache, Python, Perl that publish KEYS files
func (v *Verifier) ImportKeysFromURL(ctx context.Context, keysURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", keysURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download KEYS file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("KEYS file download failed with status %d", resp.StatusCode)
	}

	// Limit KEYS file size to 10MB (some projects have large keyring files)
	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)

	// Try reading as armored keyring
	entities, err := openpgp.ReadArmoredKeyRing(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to parse KEYS file: %w", err)
	}

	if len(entities) == 0 {
		return fmt.Errorf("no keys found in KEYS file")
	}

	// Import all keys - signature verification will fail if key is expired
	v.keyring = append(v.keyring, entities...)

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

	// Security: Limit signature size to 10KB (GPG signatures are typically < 1KB)
	// This prevents DoS attacks via large signature files
	limitedReader := io.LimitReader(resp.Body, 10*1024)
	sigData, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read signature: %w", err)
	}

	// Security: Basic format validation
	if len(sigData) < 10 {
		return fmt.Errorf("signature file too small to be valid GPG signature")
	}

	// Open the file to verify
	//nolint:gosec // G304: filePath is user-provided for GPG verification
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	// Check if signature is armored (starts with -----BEGIN PGP SIGNATURE-----)
	isArmored := len(sigData) > 27 && string(sigData[:27]) == "-----BEGIN PGP SIGNATURE---"

	var verifyErr error
	if isArmored {
		// Use CheckArmoredDetachedSignature for armored signatures
		sigReader := &sigReader{data: sigData}
		_, verifyErr = openpgp.CheckArmoredDetachedSignature(v.keyring, f, sigReader, nil)
	} else {
		// Use CheckDetachedSignature for binary signatures
		sigReader := &sigReader{data: sigData}
		_, verifyErr = openpgp.CheckDetachedSignature(v.keyring, f, sigReader, nil)
	}

	if verifyErr != nil {
		return fmt.Errorf("signature verification failed: %w", verifyErr)
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

	// Peek at signature file to determine if it's armored
	peekBuf := make([]byte, 27)
	n, _ := io.ReadFull(sigFile, peekBuf)
	isArmored := n == 27 && string(peekBuf[:27]) == "-----BEGIN PGP SIGNATURE---"

	// Reset signature file to beginning
	if _, seekErr := sigFile.Seek(0, 0); seekErr != nil {
		return fmt.Errorf("failed to reset signature file: %w", seekErr)
	}

	// Verify signature using appropriate method
	var verifyErr error
	if isArmored {
		_, verifyErr = openpgp.CheckArmoredDetachedSignature(v.keyring, dataFile, sigFile, nil)
	} else {
		_, verifyErr = openpgp.CheckDetachedSignature(v.keyring, dataFile, sigFile, nil)
	}

	if verifyErr != nil {
		return fmt.Errorf("signature verification failed: %w", verifyErr)
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
