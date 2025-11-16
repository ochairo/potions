<div align="center">

# ⚗️ Potions

Curated distribution of developer tools

Pre-compiled binaries for macOS and Linux, automatically built and signed.

[![CI](https://img.shields.io/github/actions/workflow/status/ochairo/potions/tests-cli.yml?branch=main&label=tests&logo=github)](https://github.com/ochairo/potions/actions)
[![Security](https://img.shields.io/github/actions/workflow/status/ochairo/potions/scan-application.yml?branch=main&label=security&logo=github)](https://github.com/ochairo/potions/actions/workflows/scan-application.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ochairo/potions)](https://goreportcard.com/report/github.com/ochairo/potions)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/ochairo/potions/badge)](https://scorecard.dev/viewer/?uri=github.com/ochairo/potions)
[![License](https://img.shields.io/github/license/ochairo/potions)](https://github.com/ochairo/potions/blob/main/LICENSE)

[Features](#-features) • [Recipes](#-supported-recipes) • [Documentation](#-documentation)

</div>

## ✨ Features

- **Automated Monitoring**: Daily checks for new upstream versions via GitHub API, RSS, and custom URLs
- **Multi-Platform Builds**: macOS (Intel/ARM) and Linux (x64/ARM64) with code signing and notarization
- **Security Scanning**: Vulnerability detection and SBOM generation for all releases
- **Reproducible**: Deterministic builds with SHA256 verification
- **YAML Configuration**: Simple recipe format for adding new packages

## 📜 Supported Recipes

- [View all supported recipes →](./recipes/)

## 🏛️ Documentation

- [Architecture](./docs/ARCHITECTURE.md) - Build and release pipeline
- [Contributing](./docs/CONTRIBUTING.md) - Development setup and adding packages

<br><br>

<div align="center">

[Report Bug](https://github.com/ochairo/potions/issues) • [Request Feature](https://github.com/ochairo/potions/issues) • [Documentation](./docs/)

**Made with ❤︎ by [ochairo](https://github.com/ochairo)**

</div>
