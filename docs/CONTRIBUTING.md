# Contributing

## Prerequisites

- **Go 1.23+** from [go.dev](https://go.dev/dl/)
- **GitHub Token** with `repo` scope

  ```bash
  export GITHUB_TOKEN=your_token_here
  ```

## Setup

```bash
git clone https://github.com/ochairo/potions.git
cd potions
go build -o bin/potions ./cmd/potions
./bin/potions --version
```

## Adding Packages

Create `recipes/myapp.yml`:

```yaml
name: myapp
version_source:
  type: github_release
  repository: owner/myapp
download_url: "https://github.com/owner/myapp/releases/download/{version}/myapp-{version}{suffix}"
platforms:
  darwin-x86_64:
    suffix: "-darwin-x86_64.tar.gz"
    binary_path: "myapp"
  # ... other platforms
```

Test:

```bash
./bin/potions validate recipes/myapp.yml
./bin/potions monitor myapp
./bin/potions build myapp
```

### Recipe Fields

**Required:**

- `name` - Unique identifier
- `version_source` - `github_release`, `github_tag`, `rss`, or `url`
- `download_url` - Template with `{version}`, `{os}`, `{arch}`, `{suffix}`
- `platforms` - `darwin-x86_64`, `darwin-arm64`, `linux-amd64`, `linux-arm64`
  - `suffix` - Appended to download URL
  - `binary_path` - Path to binary in archive

**Optional:**

- `description`, `homepage`, `build_commands`

### Version Sources

```yaml
# GitHub Release
version_source:
  type: github_release
  repository: owner/repo

# GitHub Tag
version_source:
  type: github_tag
  repository: owner/repo
  tag_pattern: "^v?[0-9]+\\.[0-9]+\\.[0-9]+$"

# RSS/URL
version_source:
  type: rss
  url: "https://example.com/releases.xml"
  version_pattern: "Version ([0-9.]+)"
```

### Examples

See `recipes/kubectl.yml`, `recipes/terraform.yml` for reference.

### Common Issues

- **Version prefix**: Don't hardcode `v` in `download_url`, let `version_source` handle it
- **Binary path**: Extract archive locally to verify exact path
- **Suffix**: Match exact filename from releases page

## Testing

```bash
# Unit tests
go test ./...

# Recipe testing
./bin/potions validate recipes/myapp.yml
./bin/potions monitor myapp
./bin/potions build myapp
```

## Code Quality

```bash
# Linting
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run

# Formatting
gofmt -w .
```

## Submitting

```bash
git checkout -b add-myapp
git add recipes/myapp.yml
git commit -m "Add myapp recipe"
git push origin add-myapp
```

Open PR with:

- Recipe validation evidence
- Test results
- Platform-specific notes

## Help

- [GitHub Discussions](https://github.com/ochairo/potions/discussions)
- [GitHub Issues](https://github.com/ochairo/potions/issues)
- Browse `recipes/` for examples
