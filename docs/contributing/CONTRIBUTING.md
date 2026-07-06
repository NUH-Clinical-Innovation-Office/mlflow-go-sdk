# Contributing

## Commit Messages

This project uses [commitlint](https://commitlint.js.org/) with the [@commitlint/config-conventional](https://github.com/conventional-changelog/commitlint/tree/master/@commitlint/config-conventional) configuration.

Format: `<type>: <subject>`

### Allowed Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, semicolons, etc)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Build process or auxiliary tool changes
- `ci`: CI configuration changes
- `build`: Build system changes
- `revert`: Reverting previous commits

### Examples

```
feat: add user registration endpoint
fix: resolve race condition in auth handler
docs: update API documentation
```

## Pre-commit Hooks

This project uses [lefthook](https://github.com/evilmartians/lefthook) for managing git hooks. Install lefthook and the configured hooks will run before each commit.

## Development Setup

### Required Tools

- Go 1.26+
- golangci-lint
- sqlc v1.28.0+
- golang-migrate

Install tools with: `make install-tools`

### Running Tests

```bash
# All tests
make test

# Unit tests only
make test-unit

# Integration tests (requires Docker)
make test-integration
```

### Code Quality

Before committing, run:

```bash
make verify
```

This runs: fmt, vet, lint, sqlc-compile, and test.

### CI Pipeline

The CI pipeline runs: `make ci` which runs all verification steps.

## Pull Request Process

1. Fork the repository and create a feature branch
2. Make your changes following the commit message format
3. Ensure all tests pass and code is properly linted
4. Submit a pull request with a clear description of changes
5. PR requires review before merging