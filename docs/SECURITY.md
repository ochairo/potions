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
- **Provenance:** SLSA provenance attestations for build reproducibility
- **GPG Signatures:** (Coming soon) GPG signatures for release artifacts
- **Vulnerability Scanning:** Automated OSV vulnerability scanning for all packages

### Code Security

- **Static Analysis:** CodeQL, gosec, and staticcheck for Go code
- **Dependency Review:** Automated dependency vulnerability scanning
- **Secret Scanning:** Gitleaks for credential detection
- **License Compliance:** Automated license checking for dependencies

### Infrastructure Security

- **GitHub Actions:** All workflows use pinned dependencies
- **Least Privilege:** Minimal permissions for CI/CD workflows
- **Artifact Integrity:** All build artifacts are verified before distribution

## Security Best Practices for Users

When using binaries from this project:

1. **Verify Checksums:** Always verify SHA256/SHA512 checksums after download
2. **Check SBOM:** Review the Software Bill of Materials for dependencies
3. **Review Provenance:** Verify the build provenance attestation
4. **Stay Updated:** Use the latest version to get security patches
5. **Report Issues:** If you find something suspicious, report it immediately

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
