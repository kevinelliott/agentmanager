# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.3.1] - 2026-04-24

Patch release addressing review feedback from v1.3.0. Includes one
real shipping-bug fix (cask install for 5 catalog entries) plus a
batch of correctness, doc, and test cleanups.

### Fixed

- **`brew install` for casks declared with the legacy `cask: "true"`
  metadata key.** `parseBrewPackage` previously only recognized the
  canonical `type: "cask"` form, so 5 catalog entries (block-goose,
  openclaw, droid, claude-squad, amazon-q-cli) ran `brew install
  <name>` without `--cask` and Homebrew rejected them as unknown
  formulae. Now accepts both keys.
- `pkg/platform`: `resetLookPathCache()` no longer reassigns the
  package-level `sync.Map` (unsafe vs concurrent readers). Uses
  `Range`+`Delete`.
- `pkg/installer/providers/brew`: `GetLatestVersion` no longer caches
  context-canceled / deadline-exceeded errors. A caller with a short
  ctx previously poisoned the 5-min TTL for every later caller.
- `pkg/installer/providers/progress`: progress writer now wraps an
  `io.Writer` in a serializing lock so concurrent writes from
  `cmd.Stdout` and `cmd.Stderr` (os/exec writes those from separate
  goroutines) do not interleave bytes mid-call.
- `pkg/catalog/embed`: `EmbeddedJSON()` returns a copy of the bytes
  rather than the package-level slice — callers can no longer mutate
  the baseline.
- `pkg/catalog/manager`: `loadEmbedded` now also probes
  `/usr/share/agentmgr/catalog.json` — the path goreleaser's nfpm
  config installs to for `.deb`/`.rpm` users. Previously that file
  was silently ignored at runtime.
- `internal/cli/doctor`: Catalog Sources section mirrors the new
  loader probe order.
- `pkg/api/grpc/server`: explicit `_ = s.listener.Close()` in `Stop()`
  and `ForceStop()` (silences gosec G104; close error during teardown
  is not actionable).

### Removed

- `agent list` `--all` / `-a` and `--check-updates` flags. Both bound
  to local vars that were never read; users setting them got a silent
  no-op. The test now asserts both flags are absent to prevent
  re-introduction.

### Internal

- `pkg/catalog/manager`: `Refresh` doc comments now explicitly note
  that `singleflight` does not merge contexts — the first caller's
  ctx governs the shared HTTP fetch for all coalesced callers.
- `internal/systray/cli_uninstall_darwin.go`: removed dead Linux and
  Windows branches (file is `//go:build darwin`-only, so
  `platform.ID()` is always Darwin at runtime).
- Test hardening: spinner non-TTY test asserts `s.isTTY=false`
  directly instead of timing-based sleep; table ANSI test asserts
  inter-column padding instead of discarding lines; systray
  `mustParseVersion` panics on parse error so test-setup bugs
  surface at the fixture; `TestDialogTracking` uses bare `*exec.Cmd`
  values so Windows CI doesn't hit a PATH lookup for `true`.
- README and CHANGELOG: trivial doc nits (command prefix, broken
  shortcut reference link).

## [1.3.0] - 2026-04-24

Toolchain + dependency refresh. **Minimum Go is now 1.25** (bumped from
1.24), along with `golangci-lint v2` and five dependency updates. Also
includes a real TUI perf win (storage reuse across refreshes), a new
`doctor` breakdown of catalog sources, and small logging + docs polish
landed after v1.2.0.

### Changed (potentially breaking)

- **Minimum Go version: 1.25.** `golang.org/x/sync v0.20.0` requires
  `go >= 1.25.0`. Rather than pin that sub-package to v0.19 and leave
  four other bumps blocked, the module minimum was raised to match.
  Users on Go 1.24 will see a clear `requires go >= 1.25` error on
  `go install`; existing binaries are unaffected.
- **Lint toolchain: golangci-lint v2.11.4** (was v1.64.8). The v1 line
  is discontinued and can't lint Go 1.25 modules. `.golangci.yml`
  migrated to v2 format. CI uses `golangci-lint-action@v7`. Three v2
  rules were excluded as false-positive-heavy on this codebase:
  `gocritic importShadow`, `gosec G703`, and `noctx` — see
  `.golangci.yml` for the rationale.

### Added

- `agentmgr doctor` now reports a **Catalog Sources** section listing
  every resolution layer (SQLite cache, user overrides, system share,
  embedded baseline) with version + age so users can see at a glance
  which layer will serve their catalog.
- `pkg/catalog`: cache-save warn log now routes through
  `logging.FromContext(ctx)`, letting callers inject a request-scoped
  logger (and letting tests assert events fired via `logging.WithContext`).

### Performance

- **TUI refresh**: the Model now reuses `*storage.Store` and
  `*catalog.Manager` for the Program's lifetime. Previously every
  `r` keypress re-ran `sql.Open`, all 8 SQLite migrations, and a
  catalog-manager construction (~80–120ms of warm-up before detection
  started). Saved time lands in the refresh latency users see.

### Dependencies

- `google.golang.org/grpc`       v1.79.3  → v1.80.0
- `golang.org/x/sync`             v0.19.0  → v0.20.0 (forces Go 1.25)
- `github.com/mattn/go-isatty`    v0.0.20  → v0.0.21
- `github.com/mattn/go-sqlite3`   v1.14.33 → v1.14.42
- `golang.org/x/sys`              v0.40.0  → v0.43.0
- `github.com/go-chi/chi/v5`      v5.2.3   → v5.2.4
- `github.com/charmbracelet/x/ansi` transitive bumps

### Docs

- `README.md` feature list surfaces v1.2.0 UX (parallel checks,
  embedded catalog, `-v` streaming, `NO_COLOR` / non-TTY handling,
  structured logs). Prerequisites updated to Go 1.25+.
- `CONTRIBUTING.md` prerequisites updated to Go 1.25+.

### Fixed

- `pkg/tui`: `fmt.Fprintf(&b, ...)` replaces a `WriteString(fmt.Sprintf(...))`
  in the catalog summary renderer (staticcheck QF1012).
- `internal/systray`: stale `//nolint:gosec` directive removed from
  a kdialog call path that no longer matches the relevant rule.

## [1.2.0] - 2026-04-21

UX and operability release. Fresh `go install` users get a working
catalog offline thanks to a `go:embed`'d baseline; long-running
`install`/`update` operations can stream live subprocess output with
`-v`; the helper now supports gRPC alongside REST; and the project
ships its first cut of structured logging.

### Added

- `-v/--verbose` on `agent install` and `agent update [--all]` now
  **streams live subprocess output** (brew / npm / pip / native) to
  stderr via a context-attached progress writer. Default (no `-v`)
  is unchanged: compact spinner only. Providers always capture the
  full output to `Result.Output`, so the change is invisible to
  non-streaming callers.
- **Embedded baseline catalog**: the binary now carries
  `catalog.json` via `//go:embed`, so a first-run `agentmgr catalog
  list` works offline even with no user-scoped override and no
  remote refresh. Resolution order: user overrides
  (`~/.agentmgr/catalog.json`, `~/.config/agentmgr/catalog.json`)
  → system-wide paths (`/usr/local/share/agentmgr`,
  `/etc/agentmgr`) → embedded bytes. The current working directory
  is no longer probed.
- `Makefile sync-catalog` + `check-catalog-sync` targets keep
  `catalog.json` (repo root) and `pkg/catalog/catalog.json`
  (build-time embed source) in sync; CI enforces it.
- Systray helper now **starts the gRPC API server** when
  `API.EnableGRPC=true` (default off, matching REST). Graceful
  shutdown uses `GracefulStop` with a 2s bound and `ForceStop`
  fallback. gRPC and REST can run concurrently on different ports.
- **`pkg/logging`**: thin slog wrapper (`New`, `WithContext`,
  `FromContext`, `Install`) driven by `cfg.Logging.{Level,Format,File}`.
  Both `cmd/agentmgr` and `cmd/agentmgr-helper` now call
  `logging.Install(logging.New(cfg))` so `slog.Default` respects
  operator config. gRPC panic-recovery interceptors emit structured
  `slog.Error` events with `method` / `panic` / `stack` fields.

### Changed

- **Dependency**: `github.com/charmbracelet/bubbles` v0.21.1 → v1.0.0.
  The v1.0 release stabilizes the public API; the components we use
  (`list`, `spinner`, `key`) are signature-compatible.
- **Test coverage**: `internal/cli/output` 0% → 82% (colors, spinner,
  table); `internal/systray` 0.2% → 3.2% (IPC handlers, menu
  formatter, shutdown, dialog tracking).
- `make lint` now **auto-installs** and runs `golangci-lint v1.64.8`
  (matching CI) rather than whatever `latest` happens to be. CI also
  pins v1.64.8 explicitly in the workflow.
- `make build` now depends on `sync-catalog`, so `make build` alone
  is sufficient to produce a binary with an up-to-date embedded
  catalog after editing `catalog.json`.

### Fixed

- Darwin-only `uninstallCLI` moved to a build-tagged file
  (`internal/systray/cli_uninstall_darwin.go`); the stale
  `//nolint:unused` directive that `golangci-lint v1.64.8` flagged
  on darwin is removed.
- Helper shutdown-signal handler now emits a structured log event
  rather than a bare `fmt.Printf`.

### Removed

- `config.CatalogConfig.RefreshOnStart` field removed. Deprecated in
  v1.1.0 and never wired to any startup behavior. Existing configs
  that still set `catalog.refresh_on_start` continue to load without
  error (viper ignores unknown keys); the value is simply dropped.

## [1.1.0] - 2026-04-21

Performance, reliability, and refactor release. Typical `agent list`
/ `agent update --all` / TUI first paint on machines with N installed
agents is now 10–20× faster thanks to parallel version checks and a
consolidated detect pipeline. Ships a critical gRPC CVE fix.

### Security

- Bump `google.golang.org/grpc` to v1.79.3 to pick up the fix for
  CVE-2026-33186 (authorization bypass via missing leading slash in
  `:path`). Transitive `golang.org/x/{net,sync,text}` bumps included.

### Added

- `internal/versionfetch`: concurrent latest-version check helper
  (errgroup + semaphore, default concurrency 8).
- `internal/orchestrator.Pipeline`: shared detect → version-check →
  cache-save pipeline now used by CLI list/update, TUI, and the
  systray helper. Eliminates ~205 lines of near-duplicated code.
- REST `getAgentsWithCache` helper: honors `Detection.CacheDuration`
  and a `?refresh=true` bypass.
- gRPC server: keepalive (30s/10s), 16 MiB message-size limits, and
  panic-recovery unary/stream interceptors.
- SQLite migrations guarded by `PRAGMA user_version` so cold starts
  skip DDL when the schema is current.
- Platform `cachedLookPath` memoizes `exec.LookPath` keyed on `PATH`
  + executable name (all three platforms).
- Catalog `Refresh` coalesced via `singleflight`; `If-None-Match` /
  HTTP 304 support using the previously-stored ETag.
- Brew `GetLatestVersion` process-wide cache with per-key
  `sync.Once` coalescing and 5-minute TTL.
- `Makefile` `lint` target: pinned to golangci-lint v1.64.8 (matches
  CI), auto-installs if missing or mismatched.

### Changed

- `GetLatestVersion` now runs concurrently (bounded to 8) at all four
  call sites (CLI list, CLI update, TUI, systray). 10–20× faster on
  typical N-agent workloads.
- `BinaryStrategy.Detect` split into deterministic match phase +
  parallel version extraction (concurrency 4).
- pip PyPI fallback uses a shared `http.Client` instead of shelling
  out to `curl`.
- SQLite DSN adds `_busy_timeout=5000` and `_synchronous=NORMAL`;
  connection pool clamped (`SetMaxOpenConns(1)`).
- REST endpoints consume the detection cache instead of forcing
  fresh `DetectAll` on every request.
- REST server: `ReadHeaderTimeout: 5s`, `MaxHeaderBytes: 1 MiB`.
- REST `/status`: exposes real server start-time, version, and
  last-refresh timestamps (previously hardcoded `"dev"` / zero).
- Systray shutdown: replaced `time.Sleep(100ms)` with a context +
  `sync.WaitGroup` handshake.
- Systray `checkUpdates`: performs real parallel version fetches
  when `AutoCheck=true` (previously a placeholder).
- Detector channel buffer now sized by the count of applicable
  strategies rather than total (no over-allocation).
- Catalog cache-save failures logged via `log/slog` instead of being
  silently dropped.
- Darwin-only `uninstallCLI` moved to build-tagged
  `internal/systray/cli_uninstall_darwin.go`; stale `//nolint:unused`
  removed.
- CI `golangci-lint` pinned to v1.64.8 (was `latest`).

### Fixed

- `TestListenForNotificationsContextCanceled` flake: `pkg/ipc`
  `listenForNotifications` now responds to context cancellation
  promptly by setting a short read deadline (500ms) around each
  receive. Previously the blocking `conn.Receive()` could leak the
  goroutine until the connection was closed.
- Spinner ANSI escape sequences corrupting piped output when stdout
  is not a TTY. `internal/cli/output/spinner` now detects TTY via
  `go-isatty`; honors `NO_COLOR` and `TERM=dumb`.
- `--no-color` inconsistency across `agent list`, `agent info`,
  `catalog list`, and `catalog search` — unified via
  `output.NoColor(cfg, flag)` + `Printer.NoColor()`.

### Deprecated

- `config.CatalogConfig.RefreshOnStart` is marked deprecated. The
  flag is not wired to any startup behavior today (catalog refresh
  cadence is controlled by `RefreshInterval` + cache TTL). It is
  retained for backward compatibility with existing config files
  and will be removed once the sole remaining TUI display reference
  is cleaned up. (Removed in v1.2.0.)

### Known Issues

- The macOS linker emits `ld: warning: ignoring duplicate libraries:
  '-lobjc'` when building `cmd/agentmgr-helper`. This is a cosmetic
  warning from ld64 caused by Apple's clang auto-linking libobjc for
  both `getlantern/systray` (which declares `-x objective-c` CFLAGS)
  and `progrium/darwinkit` (Cocoa / Foundation frameworks). Neither
  dependency declares `-lobjc` explicitly; the duplicate is injected
  by the toolchain. Suppressing this cleanly would require dropping
  one dependency or patching cgo directives upstream. The warning
  has no runtime impact.

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

[1.3.1]: https://github.com/kevinelliott/agentmanager/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/kevinelliott/agentmanager/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/kevinelliott/agentmanager/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/kevinelliott/agentmanager/compare/v1.0.24...v1.1.0
[1.0.13]: https://github.com/kevinelliott/agentmanager/compare/v1.0.12...v1.0.13
[1.0.12]: https://github.com/kevinelliott/agentmanager/compare/v1.0.11...v1.0.12
[1.0.9]: https://github.com/kevinelliott/agentmanager/compare/v1.0.8...v1.0.9
[1.0.8]: https://github.com/kevinelliott/agentmanager/compare/v1.0.7...v1.0.8
[1.0.7]: https://github.com/kevinelliott/agentmanager/compare/v1.0.6...v1.0.7
[1.0.6]: https://github.com/kevinelliott/agentmanager/compare/v0.2.0...v1.0.6
[0.2.0]: https://github.com/kevinelliott/agentmanager/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/kevinelliott/agentmanager/releases/tag/v0.1.0
