# Architecture

## Dependency Rule

- `←` `↓` `↑` `→`: Dependency Flow
- `─//─`: Disconnected | No Dependency

```sh
┌─────────────────────────────────────────────────────────────┐
│                            cmd/                             │  ← Frameworks & Drivers
│                        (Entry Point)                        │
└──────────────────────────────┬──────────────────────────────┘
                               │
                           depends on
                               │
                               ↓
┌─────────────────────────────────────────────────────────────┐
│ ┌──────────────────────────┐    ┌─────────────────────────┐ │  ← Interface Adapters
│ │     domain-adapters/     │    │   external-adapters/    │ │
│ │ (Domain interfaces impl) ├─//─┤ (External API clients)  │ │
│ └────────────┬─────────────┘    └────────────┬────────────┘ │
│              ↓                               ↓              │
│              └───────────────┬───────────────┘              │
└──────────────────────────────┼──────────────────────────────┘
                               │
                            depends on
                               │
                               ↓
┌─────────────────────────────────────────────────────────────┐
│                    domain-orchestrators/                    │  ← Application Business Rules
└──────────────────────────────┬──────────────────────────────┘
                               │
                            depends on
                               │
                               ↓
┌─────────────────────────────────────────────────────────────┐
│                           domain/                           │  ← Enterprise Business Rules
│                                                             │
│      ┌───────────────────────────────────────────────┐      │
│      │                  services/                    │      │
│      │               (Business Logic)                │      │
│      └───────┬──────────────────────────────┬────────┘      │
│              │                              │               │
│          depends on                        uses             │
│              │                              │               │
│              ↓                              ↓               │
│      ┌────────────────┐            ┌──────────────────┐     │
│      │  interfaces/   │ implements │    entities/     │     │
│      │  (Contracts)   │ ←──────────┤  (Core Objects)  │     │
│      └────────────────┘            └──────────────────┘     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```sh
potions/
│
├── cmd/
│   └── potions/                       # Entry point - unified CLI
│       ├── main.go                     # Main entry with subcommand routing
│       ├── cmd_build.go                # Build command (version→download→security→install)
│       ├── cmd_list.go                 # List packages command
│       ├── cmd_monitor.go              # Version update monitoring
│       └── cmd_other.go                # Placeholder commands (scan, verify, release)
│
├── internal/
│   │
│   ├── domain/                         # CORE LAYER (zero external deps)
│   │   ├── entities/                   # Business objects (7 entities)
│   │   │   ├── artifact.go
│   │   │   ├── security_report.go
│   │   │   ├── sbom.go
│   │   │   ├── binary_analysis.go
│   │   │   ├── attestation.go
│   │   │   └── definition.go
│   │   ├── interfaces/                  # Contracts/ports
│   │   │   ├── gateways/
│   │   │   │   └── security.go          # SecurityGateway interface
│   │   │   ├── repositories/
│   │   │   │   └── definition_repository.go
│   │   │   └── services/
│   │   │       └── security.go          # SecurityService interface
│   │   └── services/                    # Business logic
│   │       └── security.go              # Security business rules (95.5% coverage)
│   │
│   ├── domain-orchestrators/            # USE CASE LAYER
│   │   ├── security_orchestrator.go     # 5-step security workflow
│   │   └── build_orchestrator.go        # Complete build pipeline
│   │
│   ├── domain-adapters/                 # INFRASTRUCTURE LAYER
│   │   └── gateways/                    # External service implementations
│   │       ├── osv_gateway.go           # OSV HTTP API (no binary)
│   │       ├── binary_analyzer.go       # ELF/Mach-O analysis
│   │       ├── sbom_generator.go        # CycloneDX SBOM
│   │       ├── checksum_verifier.go     # SHA256 operations
│   │       ├── gpg_verifier.go          # GPG wrapper
│   │       ├── composite_security_gateway.go
│   │       ├── version_fetcher.go       # Version source fetching
│   │       ├── downloader.go            # HTTP + tarball extraction
│   │       └── script_executor.go       # Shell script execution
│   │
│   └── external-adapters/               # EXTERNAL LIBRARIES LAYER
│       ├── gpg/                         # ProtonMail/go-crypto wrapper
│       │   └── verifier.go              # GPG signature verification
│       └── yaml/                        # YAML definition handling
│           ├── definition_parser.go     # YAML → entities.Definition
│           └── definition_repository.go # File-based repository
│
├── recipes/                         # Package definitions (139 YAML files)
├── bin/                                 # Compiled binaries
│   └── potions                         # Main CLI binary
└── docs/                                # Documentation
    ├── ARCHITECTURE.md                  # This file
    ├── GO_MIGRATION_PLAN.md             # Migration status
    └── MIGRATION_STATUS.md              # Implementation tracking
```

## Key Architectural Principles

### 1. Dependency Inversion

Services depend on **interfaces**, not concrete implementations:

```go
// ✅ Good: Service depends on interface
type installerService struct {
    downloadGW gateways.DownloadGateway  // Interface
}

// ❌ Bad: Service depends on concrete type
type installerService struct {
    httpClient *http.Client  // Concrete implementation
}
```

### 2. Single Responsibility

Each layer has one reason to change:

- **Entities**: Change when business rules change
- **Services**: Change when business logic changes
- **Orchestrators**: Change when workflows change
- **Adapters**: Change when infrastructure changes

### 3. Testability

Mock interfaces for fast, isolated tests:

```go
mockDownload := &MockDownloadGateway{}
service := NewInstallerService(mockDownload, ...)
service.Install(ctx, pkg)
assert.True(t, mockDownload.WasCalled())
```

### 4. Flexibility

Swap implementations without changing business logic:

- Change from file storage to database → update repository adapter
- Add backup service → update gateway implementation
- Switch messaging protocols → update gateway adapter
