# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < latest| :x:                |

**Note:** We currently support only the latest release with security updates.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

We take the security of our prebuilt binaries and distribution system seriously. If you believe you've found a security vulnerability, please report it to us privately.

### How to Report

**GitHub Security Advisories:** Use the [private vulnerability reporting feature](https://github.com/ochairo/potions/security/advisories/new) to report vulnerabilities privately.

### What to Include

Please include the following information in your report:

- **Description** of the vulnerability
- **Steps to reproduce** the issue
- **Affected versions** (if known)
- **Potential impact** of the vulnerability
- **Suggested fix** (if you have one)
- **Your contact information** for follow-up questions

### What to Expect

- **Initial Response:** Within 48 hours of your report
- **Progress Updates:** We'll keep you informed as we investigate
- **Fix Timeline:** We aim to release security patches within 7 days for critical issues
- **Credit:** We'll acknowledge your contribution (unless you prefer to remain anonymous)
- **Disclosure:** We'll coordinate public disclosure with you after a fix is available

## Security Measures

This project implements several security measures:

### Binary Distribution Security

- **Checksums:** SHA256 and SHA512 checksums for all binaries
- **SBOM:** Software Bill of Materials (CycloneDX format) for dependency tracking
- **Provenance:** SLSA Level 3 provenance attestations for build reproducibility
- **Cosign Signatures:** Keyless Sigstore/Cosign signatures for all release artifacts
- **GitHub Attestations:** SLSA provenance attestations generated via GitHub's native attestation API
- **GPG Signatures:** Optional GPG signatures for release artifacts (configurable)
- **Vulnerability Scanning:** Automated OSV vulnerability scanning for all packages
- **Artifact Verification:** Automated checksum verification before release
- **Runtime Verification:** `potions verify` command supports GPG, Cosign, and attestation verification

### Code Security

- **Static Analysis:** CodeQL, gosec, and staticcheck for Go code
- **Dependency Review:** Automated dependency vulnerability scanning
- **Secret Scanning:** Gitleaks for credential detection
- **License Compliance:** Automated license checking for dependencies
- **CODEOWNERS:** Security-critical files require maintainer approval
- **Weekly Security Audits:** Automated auditing of release artifacts

### Infrastructure Security

- **GitHub Actions:** All workflows use pinned commit SHAs (not tags)
- **Least Privilege:** Minimal permissions for CI/CD workflows
- **Artifact Integrity:** All build artifacts are verified before distribution
- **Environment Protection:** Production releases can require manual approval
- **Secret Protection:** GPG keys stored in protected GitHub environment
- **Artifact Retention:** 3-day retention for builds, 30-day for audit trails

## Security Best Practices for Users

When using binaries from this project:

1. **Verify Checksums:** Always verify SHA256/SHA512 checksums after download
   ```bash
   potions verify package.tar.gz --checksum package.tar.gz.sha256
   ```

2. **Verify Signatures:** Verify Cosign keyless signatures
   ```bash
   potions verify package.tar.gz --cosign-sig package.tar.gz.sig --cosign-cert package.tar.gz.pem
   ```

3. **Verify Attestations:** Verify GitHub SLSA attestations
   ```bash
   potions verify package.tar.gz --attest-file package.tar.gz.attestation.jsonl --owner ochairo --repo potions
   ```

4. **Verify All:** Use `--all` flag to automatically verify all available signatures
   ```bash
   potions verify package.tar.gz --all --owner ochairo --repo potions
   ```

5. **Check SBOM:** Review the Software Bill of Materials for dependencies
   ```bash
   cat package.sbom.json | jq '.components[] | {name, version}'
   ```

6. **Review Provenance:** Verify the build provenance attestation
7. **Stay Updated:** Use the latest version to get security patches
8. **Report Issues:** If you find something suspicious, report it immediately

## Known Security Considerations

### Supply Chain Security

This project distributes prebuilt binaries from upstream sources. Security considerations:

- **Upstream Trust:** We rely on upstream projects for source security
- **Build Process:** All builds are automated and reproducible via GitHub Actions
- **Artifact Storage:** Binaries are stored in GitHub Releases with checksums
- **Verification:** We verify upstream checksums when available

### Vulnerability Response

When vulnerabilities are discovered in distributed packages:

1. We assess the impact on distributed binaries
2. We update to patched versions within 24-48 hours for critical issues
3. We notify users via GitHub Releases and security advisories
4. We maintain a public record of addressed vulnerabilities

## Security Updates

Subscribe to security updates:

- **GitHub Watch:** Enable "Security alerts" notifications
- **RSS Feed:** Subscribe to our [releases feed](https://github.com/ochairo/potions/releases.atom)
- **Security Advisories:** Watch our [security advisories](https://github.com/ochairo/potions/security/advisories)

## Scope

This security policy covers:

- ✅ The `potions` CLI tool and build system
- ✅ GitHub Actions workflows and CI/CD pipeline
- ✅ Build and packaging processes
- ✅ Security artifacts (checksums, SBOM, provenance)
- ⚠️ Upstream binary sources (report to upstream projects)
- ⚠️ User-specific deployment issues (support, not security)

## Contact

For security-related questions or concerns:

- **Security Issues:** Use GitHub Security Advisories
- **General Questions:** Open a GitHub Discussion
- **Non-Security Bugs:** Open a GitHub Issue

---

**Last Updated:** November 15, 2025
