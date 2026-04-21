# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `pkg/ipc`: `listenForNotifications` now responds to context cancellation
  promptly by using a short read deadline (500ms) around each receive.
  Previously the blocking `conn.Receive()` could leak the goroutine until
  the connection was closed, causing `TestListenForNotificationsContextCanceled`
  to flake on CI.
- `internal/cli/output/spinner`: spinner now detects TTY via `go-isatty` at
  construction. When stdout is piped/redirected/not a terminal (or
  `TERM=dumb` / `NO_COLOR` is set), `Start` and `Stop` are no-ops so ANSI
  escape sequences no longer corrupt piped output.
- `internal/cli`: `--no-color` handling unified via `output.NoColor(cfg, flag)`
  helper and `Printer.NoColor()` accessor. Previously three different code
  paths (agent list, agent info, catalog list/search) derived the value
  differently, which could leave the spinner colored while the printer was
  monochrome (or vice versa).

### Deprecated

- `config.CatalogConfig.RefreshOnStart` is marked deprecated. The flag is
  not wired to any startup behavior today (catalog refresh cadence is
  controlled by `RefreshInterval` + cache TTL). It is retained for
  backward compatibility with existing config files and will be removed
  once the sole remaining TUI display reference is cleaned up.

### Known Issues

- The macOS linker emits `ld: warning: ignoring duplicate libraries:
  '-lobjc'` when building `cmd/agentmgr-helper`. This is a cosmetic
  warning from ld64 caused by Apple's clang auto-linking libobjc for both
  `getlantern/systray` (which declares `-x objective-c` CFLAGS) and
  `progrium/darwinkit` (Cocoa / Foundation frameworks). Neither
  dependency declares `-lobjc` explicitly; the duplicate is injected by
  the toolchain. Suppressing this cleanly would require dropping one
  dependency or patching cgo directives upstream. The warning has no
  runtime impact.

## [1.0.16] - 2026-01-16

### Fixed

- Binary/native detection no longer reports package manager installations as "native"
  - Executables in npm, pip, brew, nvm, pyenv, asdf, mise, etc. paths are now correctly excluded
  - Only truly native installations (e.g., `/usr/local/bin`) are reported as native

## [1.0.15] - 2026-01-16

### Fixed

- `agentmgr agent update` now correctly detects available updates
  - Previously reported "already up to date" even when updates were available
  - The update command now checks latest versions before comparing

## [1.0.14] - 2026-01-16

### Added

- OpenAPI/Swagger documentation for REST API (`api/openapi.yaml`)
- `agentmgr api` command group for viewing API documentation and endpoints
- Detection plugin system for custom agent discovery logic
- `agentmgr plugin` command group (list, create, validate, enable, disable)
- Plugin documentation in `docs/plugins.md`

### Changed

- REST API now includes `/openapi.yaml` endpoint for self-documentation
- Catalog API response now includes `category` and `tags` fields

## [1.0.13] - 2026-01-16

### Added

- `agentmgr agent list --verify` flag for agent health checks
- `agentmgr config export` and `config import` commands
- Agent categories and tags in catalog schema
- Performance benchmarks for catalog and detection
- Better error messages with actionable hints for npm, pip, and brew failures
- Windows support in CI integration tests

### Changed

- CI now runs integration tests on Ubuntu, macOS, and Windows

## [1.0.12] - 2026-01-16

### Added

- `agentmgr doctor` command for system health checks
- `agentmgr upgrade` command for self-updating from GitHub releases
- Homebrew formula for source-based installation
- Integration tests in CI pipeline

### Changed

- CI now runs integration tests on Ubuntu and macOS

## [1.0.9] - 2026-01-14

### Added

- Shell completions for bash, zsh, fish, and PowerShell
- Improved test coverage across packages

### Changed

- Updated README with all 32 supported agents
- Updated Go requirement to 1.24

## [1.0.8] - 2026-01-13

### Added

- Agent CLI to catalog
- Ralph TUI to catalog

### Fixed

- Resolved linting errors throughout codebase
- Fixed broken tests
- Fixed catalog URL and validation for git-cloned projects

## [1.0.7] - 2026-01-10

### Added

- Juno Code to catalog

## [1.0.6] - 2026-01-10

### Added

- TunaCode to catalog
- Goose to catalog
- Pi Coding Agent to catalog
- Dexter to catalog
- Helpful guidance for npm EACCES permission errors
- Colors and animations to CLI output
- Version checking to agent list command
- Dependabot configuration for Go modules

### Changed

- Bumped catalog version to 1.0.6
- Improved table alignment with ANSI codes

### Fixed

- Fixed BinaryStrategy to check catalog for install methods before reporting
- Fixed systray functionality
- Fixed PID handling before Process.Release() in helper start

## [0.2.0] - 2026-01-08

### Added

- TLS support for secure connections
- Complete systray functionality
- Config get/set commands with value persistence
- TUI and helper commands integration with libraries
- Comprehensive test suite for:
  - gRPC API package
  - REST API package
  - IPC package
  - Catalog Manager
  - Detector strategies
  - Installer providers
  - Platform package
  - TUI styles package
  - CLI utility functions
  - TUI package
  - Systray package

### Changed

- Improved agent list and install commands
- Enhanced catalog list functionality
- Used constants instead of string literals in platform functions

## [0.1.0] - 2026-01-08

### Added

- Initial release of AgentManager
- Core agent management functionality
- CLI interface for managing AI coding agents
- Agent catalog with multiple supported agents
- Detection strategies for installed agents
- Installation support via multiple providers (Homebrew, npm, pip, Cargo, Go, binary downloads)
- Platform detection and support (macOS, Linux, Windows)
- TUI (Terminal User Interface) mode
- Helper daemon for background operations
- gRPC and REST API support
- IPC (Inter-Process Communication) support
- Configuration management
- Makefile for common development tasks

[1.0.13]: https://github.com/kevinelliott/agentmanager/compare/v1.0.12...v1.0.13
[1.0.12]: https://github.com/kevinelliott/agentmanager/compare/v1.0.11...v1.0.12
[1.0.9]: https://github.com/kevinelliott/agentmanager/compare/v1.0.8...v1.0.9
[1.0.8]: https://github.com/kevinelliott/agentmanager/compare/v1.0.7...v1.0.8
[1.0.7]: https://github.com/kevinelliott/agentmanager/compare/v1.0.6...v1.0.7
[1.0.6]: https://github.com/kevinelliott/agentmanager/compare/v0.2.0...v1.0.6
[0.2.0]: https://github.com/kevinelliott/agentmanager/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/kevinelliott/agentmanager/releases/tag/v0.1.0
