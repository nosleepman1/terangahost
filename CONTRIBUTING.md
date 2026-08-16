# Contributing to TerangaHost 🇸🇳

Thank you for your interest in contributing to **TerangaHost**! We are thrilled to welcome developers from Senegal, across Africa, and around the globe.

---

## 🛠️ Development Setup

### Prerequisites
- **Go 1.22+** installed.
- **Git**.
- *(Optional)* A test Linux VPS or local VM running **Ubuntu 22.04 LTS or 24.04 LTS**.

### 1. Clone and Run Tests
```bash
git clone https://github.com/nosleepman1/terangahost.git
cd terangahost

# Run all unit tests with race detection
go test -v -race ./...

# Build the executable
go build -o bin/terangahost main.go
```

---

## 📐 Architecture & Standards

TerangaHost strictly follows **Clean Architecture / Ports & Adapters**:

1. **Domain Isolation (`internal/domain/`)**: Pure Go business logic. Never import external CLI or network packages inside `domain`.
2. **Inversion of Control (`domain.Runner`)**: All SSH and shell executions must go through the `Runner` interface to enable testing via `mocks.MockRunner`.
3. **Strict Idempotency (`domain.Step`)**: Every step in `internal/engine/steps/` must implement `PreCheck()` to ensure commands can be safely re-run without breaking servers.

---

## 🌿 Branching & Git Commit Conventions

We follow the **Conventional Commits** specification:

- `feat(scope): ...` for new features or capabilities.
- `fix(scope): ...` for bug fixes.
- `docs(scope): ...` for documentation improvements.
- `test(scope): ...` for adding or improving tests.
- `refactor(scope): ...` for code refactoring without feature changes.

### Pull Request Workflow
1. Fork the repository on GitHub.
2. Create a feature branch: `git checkout -b feat/your-feature-name`.
3. Ensure all tests pass: `go test -v -race ./...`.
4. Commit your changes following Conventional Commits.
5. Push to your fork: `git push origin feat/your-feature-name`.
6. Open a **Pull Request** against the `main` branch.

---

## 💬 Community & Help

Need help or want to discuss ideas?
- Open a GitHub Discussion or Issue.
- Connect with the local Laravel Senegal community.
