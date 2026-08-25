# Contributing to goulm-memory

Thank you for considering contributing to goulm-memory! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Pull Request Process](#pull-request-process)
- [Coding Guidelines](#coding-guidelines)
- [Testing](#testing)
- [Documentation](#documentation)
- [Community](#community)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment. Be kind and constructive in all interactions.

## Getting Started

### Prerequisites

- **Go 1.26+** — [Download Go](https://go.dev/dl/)
- **Git** — [Download Git](https://git-scm.com/)
- A GitHub account

### Fork and Clone

```bash
# Fork the repository on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/goulm-memory.git
cd goulm-memory

# Add the upstream remote
git remote add upstream https://github.com/LRGolden/goulm-memory.git
```

## Development Setup

```bash
# Verify everything works
go test ./pkg/memory/ -v

# Run benchmarks
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecall" -benchmem

# Run the demo
go run ./cmd/demo recall -q "authentication" -limit 5
```

## How to Contribute

### Reporting Bugs

1. Check [existing issues](https://github.com/LRGolden/goulm-memory/issues) to avoid duplicates
2. Open a new issue with a clear title and description
3. Include:
   - Go version (`go version`)
   - Operating system
   - Minimal reproduction code
   - Expected vs actual behavior

### Suggesting Features

1. Open an issue with the `feature` label
2. Describe the problem you're trying to solve
3. Explain your proposed solution
4. Note any alternatives you considered

### Submitting Code

1. **Bug fixes** — Always welcome. Include a test that reproduces the bug.
2. **New features** — Open an issue first to discuss the design.
3. **Performance improvements** — Include benchmark results showing the improvement.
4. **Documentation** — Always welcome.

## Pull Request Process

### Before You Start

1. Open an issue describing what you want to change
2. Wait for maintainer feedback (especially for new features)
3. Fork and create a branch from `main`

### Branch Naming

```
bugfix/short-description     # Bug fixes
feature/short-description    # New features
docs/short-description       # Documentation changes
perf/short-description       # Performance improvements
```

### Making Changes

```bash
# Create your branch
git checkout -b feature/my-feature

# Make your changes, then test
go test ./pkg/memory/ -v

# Run benchmarks if performance-related
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecall" -benchmem

# Commit with a clear message
git commit -m "pkg/memory: add support for X"

# Push and create a PR
git push origin feature/my-feature
```

### Commit Messages

Follow Go conventions:

```
pkg/memory: add support for X

Description of what changed and why.

Fixes #123
```

- Start with the package name
- Use imperative mood ("add", not "added")
- Keep the first line under 72 characters
- Reference related issues

### PR Description

Include:
1. **What** — What does this PR do?
2. **Why** — Why is this change needed?
3. **How** — How does it work?
4. **Tests** — What tests did you add/run?
5. **Benchmarks** — If performance-related, include before/after results

### Review Checklist

Before submitting, verify:

- [ ] `go test ./pkg/memory/ -v` passes
- [ ] `go vet ./pkg/memory/` has no warnings
- [ ] New code has tests
- [ ] Documentation is updated (if applicable)
- [ ] No new external dependencies (this project has zero dependencies by design)
- [ ] Code follows existing style

## Coding Guidelines

### Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use the existing code style as reference
- Keep functions focused and small
- Use descriptive variable names
- Add comments for non-obvious logic

### Architecture

goulm-memory is designed with these constraints:

- **Zero external dependencies** — Only Go stdlib. No third-party packages.
- **Single package** — Core logic lives in `pkg/memory/`
- **Multi-process safe** — Use advisory file locks for persistence
- **Atomic writes** — All file operations must be atomic

### Error Handling

- Return errors, don't panic
- Use `fmt.Errorf` with `%w` for wrapping
- Check errors at every step

```go
// Good
data, err := os.ReadFile(path)
if err != nil {
    return fmt.Errorf("reading file: %w", err)
}

// Bad
data, _ := os.ReadFile(path)
```

## Testing

### Running Tests

```bash
# All tests
go test ./pkg/memory/ -v

# Specific test
go test ./pkg/memory/ -run "TestRemember" -v

# Benchmarks
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecall" -benchmem

# Race detector
go test ./pkg/memory/ -race -v
```

### Writing Tests

- Place tests in the same package (`memory`)
- Use table-driven tests
- Test edge cases (empty input, nil pointers, concurrent access)
- Use `t.Helper()` for test helpers
- Benchmark performance-critical code

```go
func TestRecall(t *testing.T) {
    tests := []struct {
        name    string
        query   string
        limit   int
        want    int
    }{
        {"basic query", "auth", 5, 5},
        {"empty query", "", 5, 0},
        {"zero limit", "auth", 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Documentation

- Update relevant docs when changing API or behavior
- Keep code comments concise and accurate
- Add examples for new public functions
- Update CHANGELOG.md for user-facing changes

### Documentation Structure

```
docs/
  QUICKSTART.md     # Getting started guide
  API.md           # API reference
  ADVANCED.md      # Advanced usage
  EMBEDDINGS.md    # Embedding provider integration
  SERVER.md        # HTTP server
  TOOLS.md         # Tool registry
  ARCHITECTURE.md  # Internal design
  FORMATS.md       # Persistence formats
  VECTOR_SEARCH.md # Vector search methods
  CHANGELOG.md     # Release history
  es/              # Spanish translations (archived snapshots)
```

## Community

- **Issues** — Report bugs, request features, ask questions
- **Discussions** — Share ideas, show how you use goulm-memory
- **Pull Requests** — Contribute code, docs, or fixes

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
