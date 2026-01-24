# Contributing to AgentManager

Thank you for your interest in contributing to AgentManager! This document provides guidelines and instructions for contributing.

## Code of Conduct

This project follows a standard open-source code of conduct. Please be respectful and constructive in all interactions. We expect all contributors to:

- Use welcoming and inclusive language
- Respect differing viewpoints and experiences
- Accept constructive criticism gracefully
- Focus on what is best for the community

## How to Report Bugs

1. **Search existing issues** first to avoid duplicates
2. **Open a new issue** at [GitHub Issues](https://github.com/kevinelliott/agentmanager/issues)
3. **Include:**
   - A clear, descriptive title
   - Steps to reproduce the bug
   - Expected vs. actual behavior
   - Your environment (OS, Go version, agentmgr version)
   - Relevant logs or error messages
   - Screenshots if applicable

## How to Suggest Features

1. **Check existing issues and discussions** to see if it's already proposed
2. **Open a new issue** with the `enhancement` label
3. **Describe:**
   - The problem you're trying to solve
   - Your proposed solution
   - Alternative approaches you've considered
   - How this benefits other users

## Development Setup

### Prerequisites

- **Go 1.24+** - [Download](https://go.dev/dl/)
- **Make** - Usually pre-installed on macOS/Linux
- **golangci-lint** - Installed automatically by `make lint`, or manually:
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```

### Building

```bash
# Clone the repository
git clone https://github.com/kevinelliott/agentmanager.git
cd agentmanager

# Install dependencies
make deps

# Build all binaries
make build

# Run tests
make test

# Run all checks (fmt, vet, lint, test)
make check
```

### Useful Make Commands

| Command | Description |
|---------|-------------|
| `make build` | Build agentmgr and agentmgr-helper |
| `make test` | Run tests with race detection and coverage |
| `make test-verbose` | Run tests with verbose output |
| `make test-coverage` | Generate HTML coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code with go fmt |
| `make vet` | Run go vet |
| `make check` | Run all checks (fmt, vet, lint, test) |
| `make clean` | Remove build artifacts |

## Pull Request Process

1. **Fork** the repository and create your branch from `main`
2. **Follow the code style** guidelines below
3. **Write or update tests** for your changes
4. **Ensure all checks pass:**
   ```bash
   make check
   ```
5. **Commit** with clear, descriptive messages
6. **Push** your branch and open a Pull Request
7. **Fill out the PR template** completely
8. **Address review feedback** promptly

### PR Guidelines

- Keep PRs focused and reasonably sized
- Reference related issues using `Fixes #123` or `Closes #123`
- Update documentation if your changes affect user-facing behavior
- Add yourself to CONTRIBUTORS if this is your first contribution

## Code Style Guidelines

### Go Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` for formatting (run `make fmt`)
- Keep functions focused and reasonably sized
- Use meaningful variable and function names
- Export only what needs to be public

### Linting

We use `golangci-lint` with the configuration in `.golangci.yml`. Run:

```bash
make lint
```

Fix all linting errors before submitting your PR. Common issues:

- Unused variables or imports
- Missing error handling
- Ineffective assignments
- Formatting issues

### Project Structure

```
agentmgr/
├── cmd/                    # Binary entry points
│   ├── agentmgr/           # Main CLI/TUI
│   └── agentmgr-helper/    # Background systray helper
├── pkg/                    # Public library packages
│   ├── agent/              # Agent types and versions
│   ├── catalog/            # Catalog management
│   ├── detector/           # Agent detection
│   ├── installer/          # Installation management
│   ├── storage/            # SQLite storage
│   ├── config/             # Configuration
│   ├── ipc/                # IPC communication
│   ├── api/                # gRPC & REST APIs
│   └── platform/           # Platform abstraction
├── internal/               # Private packages
│   ├── cli/                # CLI commands
│   ├── tui/                # TUI interface
│   └── systray/            # Systray helper
└── catalog.json            # Default agent catalog
```

## Testing Requirements

- **All new code must have tests**
- **Maintain or improve coverage** - don't decrease test coverage
- **Test both success and error paths**
- Run the full test suite before submitting:
  ```bash
  make test
  ```

### Test Commands

```bash
make test                    # Run all tests
make test-verbose            # Verbose output
make test-pkg PKG=agent      # Test specific package
make test-unit               # Unit tests only (pkg/)
make test-coverage           # Generate coverage report
make test-integration        # Run integration tests
```

## Adding New Agents to the Catalog

To add a new AI CLI agent to the catalog:

1. **Edit `catalog.json`** and add an entry in the `agents` object
2. **Follow the existing schema** - use other agents as reference
3. **Required fields:**
   - `id` - Unique identifier (lowercase, hyphenated)
   - `name` - Display name
   - `description` - Brief description
   - `homepage` - Project homepage URL
   - `install_methods` - At least one installation method

4. **Installation method structure:**
   ```json
   {
     "method": "npm",
     "package": "@scope/package-name",
     "command": "npm install -g @scope/package-name",
     "update_cmd": "npm update -g @scope/package-name",
     "uninstall_cmd": "npm uninstall -g @scope/package-name",
     "platforms": ["darwin", "linux", "windows"],
     "global_flag": "-g"
   }
   ```

5. **Test detection** by running:
   ```bash
   make build
   ./bin/agentmgr catalog show <agent-id>
   ```

6. **Increment the catalog version** in `catalog.json`

### Supported Installation Methods

- `npm` - Node.js package manager
- `pip`, `pipx`, `uv` - Python package managers
- `brew` - Homebrew (macOS/Linux)
- `go` - Go install
- `native` - Shell script or direct download
- `binary` - Pre-built binary download
- `chocolatey`, `winget`, `scoop` - Windows package managers

## Questions?

If you have questions, feel free to:

- Open a [GitHub Discussion](https://github.com/kevinelliott/agentmanager/discussions)
- Check existing issues for similar questions

Thank you for contributing!
