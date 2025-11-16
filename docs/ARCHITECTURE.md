# Architecture

Automated pipeline for monitoring, building, and releasing packages.

## Pipeline Overview

### 1. Version Monitoring

Daily at 00:00 UTC via `.github/workflows/monitor-releases.yml`

1. Parse recipe YAML files from `recipes/`
2. Fetch latest version from source (`github_release`, `github_tag`, `rss`, `url`)
3. Compare with current release
4. Trigger builds for new versions

Rate limiting: Exponential backoff (1s→32s), auto-retry on errors.

### 2. Build Pipeline

Parallel builds across 4 platforms:

```yaml
darwin-x86_64     # macOS Intel
darwin-arm64      # macOS Apple Silicon
linux-amd64       # Linux x86_64
linux-arm64       # Linux ARM64
```

Steps: Download → Extract → Build → Package → Sign (macOS) → Upload

### 3. Security Scanning

Integrated into build workflows:

- Scan binaries for CVEs (Trivy, Grype, OSV-Scanner)
- Generate SBOM (Syft)
- Fail on critical vulnerabilities

### 4. Validation

Runs after all builds complete:

- SHA256 checksum verification
- Binary integrity check
- Platform coverage validation
- Version consistency check

### 5. Release Publishing

Publishes after successful validation:

```sh
package-{version}-darwin-x86_64.tar.gz
package-{version}-darwin-arm64.tar.gz
package-{version}-linux-amd64.tar.gz
package-{version}-linux-arm64.tar.gz
checksums.txt
sbom.json
```

## Recipe Format

```yaml
name: example
version_source:
  type: github_release
  repository: owner/repo
download_url: "https://example.com/{version}/app-{version}{suffix}"
platforms:
  darwin-x86_64:
    suffix: "-darwin-x86_64.tar.gz"
    binary_path: "bin/app"
  # ... other platforms
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

## Key Features

**Parallel Execution**: Monitor and build packages concurrently
**Caching**: Go modules, Docker layers, recipe parsing
**Security**: Pinned dependencies, code signing, reproducible builds
**Error Handling**: Auto-retry, exponential backoff, isolated failures
**Monitoring**: Build metrics, alerts, structured logs
