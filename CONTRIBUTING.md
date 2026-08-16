# Contributing to TerangaHost

Thank you for your interest in contributing to TerangaHost.

---

## Development Setup

### Prerequisites
- Go 1.22 or higher.
- Git.
- (Optional) A target testing instance running Ubuntu 22.04 LTS or 24.04 LTS.

### Build and Test
```bash
git clone https://github.com/nosleepman1/terangahost.git
cd terangahost

# Run test suite with race detection
go test -v -race ./...

# Compile binary
go build -o bin/terangahost main.go
```

---

## Architectural Principles

TerangaHost strictly adheres to Clean Architecture / Ports & Adapters:

1. **Domain Isolation (`internal/domain/`)**: Pure Go business entities and interfaces with zero third-party dependencies.
2. **Inversion of Control (`domain.Runner`)**: All shell executions and file operations must implement the `Runner` interface to guarantee complete unit testability via `mocks.MockRunner`.
3. **Idempotency (`domain.Step`)**: Every step in `internal/engine/steps/` must implement `PreCheck()` to guarantee safe re-execution without configuration drift.

---

## Git Conventions

We adhere to the Conventional Commits specification:

- `feat(scope): ...` for new features or capabilities.
- `fix(scope): ...` for bug fixes.
- `docs(scope): ...` for documentation changes.
- `test(scope): ...` for unit or integration tests.
- `refactor(scope): ...` for code improvements without functional changes.

### Pull Request Workflow
1. Fork the repository.
2. Create a feature branch: `git checkout -b feat/your-feature-name`.
3. Ensure all tests pass: `go test -v -race ./...`.
4. Commit your changes following Conventional Commits.
5. Push to your fork: `git push origin feat/your-feature-name`.
6. Open a Pull Request against the `main` branch.
