# CLI Reference

Command-line interface documentation for Potions.

## Architecture

- `←` `↓` `↑` `→`: Dependency Flow
- `─//─`: Disconnected | No Dependency

```bash
┌───────────────────────────┐    ┌────────────────────────────┐
│           cmd/            │    │         pkg/client         │  ← Frameworks & Drivers
│      (CLI Interface)      ├─//─┤        (Public API)        │
└──────────────┬────────────┘    └──────────────┬─────────────┘
               │                                │
           depends on                       depends on
               │                                │
               └───────────────┬────────────────┘
                               ↓
┌─────────────────────────────────────────────────────────────┐
│ ┌──────────────────────────┐    ┌─────────────────────────┐ │  ← Interface Adapters
│ │     domain-adapters/     │    │   external-adapters/    │ │
│ │ (Domain interfaces impl) ├─//─┤ (External API clients)  │ │
│ └────────────┬─────────────┘    └────────────┬────────────┘ │
│              │                               │              │
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

```bash
cmd/                      → Entry point
├─ domain-adapters/       → Domain interfaces (GitHub, filesystem)
├─ external-adapters/     → External API clients
├─ domain-orchestrators/  → Workflow coordination
└─ domain/                → Core business logic
   ├─ services/           → Business rules
   ├─ entities/           → Domain objects
   └─ interfaces/         → Contracts
```
