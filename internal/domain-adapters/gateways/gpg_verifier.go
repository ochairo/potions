package gateways

import (
	"context"
	"fmt"

	"github.com/ochairo/potions/internal/external-adapters/gpg"
)

// gpgVerifier wraps the external GPG adapter to implement the domain gateway interface
// Note: Uses deprecated golang.org/x/crypto/openpgp but it's still maintained for security fixes
// For production, consider alternatives like:
// - ProtonMail's go-crypto (maintained fork): github.com/ProtonMail/go-crypto
// - Age encryption: github.com/FiloSottile/age
// - Direct GPG binary execution via exec.Command (zero Go dependencies)
type gpgVerifier struct {
	verifier *gpg.Verifier
}

// NewGPGVerifier creates a new GPG verifier gateway
//
//nolint:revive // unexported-return: Intentionally returns concrete type for testability
func NewGPGVerifier() *gpgVerifier {
	return &gpgVerifier{
		verifier: gpg.NewVerifier(),
	}
}

// ImportGPGKeys imports GPG keys from keyservers
func (g *gpgVerifier) ImportGPGKeys(ctx context.Context, keyIDs []string) error {
	if err := g.verifier.ImportKeys(ctx, keyIDs); err != nil {
		return fmt.Errorf("failed to import GPG keys: %w", err)
	}
	return nil
}

// ImportGPGKeysFromURL imports all GPG keys from a KEYS file URL
func (g *gpgVerifier) ImportGPGKeysFromURL(ctx context.Context, keysURL string) error {
	if err := g.verifier.ImportKeysFromURL(ctx, keysURL); err != nil {
		return fmt.Errorf("failed to import GPG keys from URL: %w", err)
	}
	return nil
}

// ImportGPGKeyFromFile imports a GPG key from a local file
func (g *gpgVerifier) ImportGPGKeyFromFile(keyPath string) error {
	if err := g.verifier.ImportKeyFromFile(keyPath); err != nil {
		return fmt.Errorf("failed to import GPG key from file: %w", err)
	}
	return nil
}

// VerifyGPGSignature verifies a detached GPG signature downloaded from a URL
func (g *gpgVerifier) VerifyGPGSignature(ctx context.Context, filePath, sigURL string) error {
	if err := g.verifier.VerifySignature(ctx, filePath, sigURL); err != nil {
		return fmt.Errorf("GPG signature verification failed: %w", err)
	}
	return nil
}

// VerifyGPGSignatureFromFile verifies a detached GPG signature from a local file
func (g *gpgVerifier) VerifyGPGSignatureFromFile(filePath, sigPath string) error {
	if err := g.verifier.VerifySignatureFromFile(filePath, sigPath); err != nil {
		return fmt.Errorf("GPG signature verification failed: %w", err)
	}
	return nil
}

// GetKeyringSize returns the number of keys loaded
func (g *gpgVerifier) GetKeyringSize() int {
	return g.verifier.GetKeyringSize()
}

// ClearKeyring clears all imported keys
func (g *gpgVerifier) ClearKeyring() {
	g.verifier.ClearKeyring()
}
