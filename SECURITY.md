# Security Policy

## Supported Versions

AgentManager is a fast-moving project; security fixes land on the
latest minor line. Older minors are not patched.

| Version | Supported          |
| ------- | ------------------ |
| 1.3.x   | :white_check_mark: |
| < 1.3   | :x:                |

If you are running a binary older than the latest 1.x release, the
recommended remediation for any vulnerability is to upgrade to the
current release. Distributors who carry older lines should backport
fixes from `main` themselves.

## Reporting a Vulnerability

**Do not open a public GitHub issue.** Use GitHub's private
vulnerability reporting:

1. Go to <https://github.com/kevinelliott/agentmanager/security/advisories/new>
2. Fill out the advisory form with as much detail as you can:
   - affected version(s) (output of `agentmgr version`)
   - reproduction steps or proof of concept
   - impact you've assessed (RCE, local privilege escalation,
     credential disclosure, etc.)
   - suggested mitigation if you have one

A maintainer will acknowledge receipt within 5 business days.

## What Counts

In scope:

- Code execution paths in `cmd/agentmgr` and `cmd/agentmgr-helper`
- The IPC, REST, and gRPC servers in `pkg/ipc`, `pkg/api/rest`,
  `pkg/api/grpc`
- Detection / installer subprocess flows in `pkg/detector` and
  `pkg/installer/providers` (path traversal, command injection,
  privilege escalation via crafted catalog entries, etc.)
- Catalog loading and remote refresh in `pkg/catalog` (TLS, ETag,
  signature handling)
- Storage in `pkg/storage` (data exfiltration via SQLite injection)

Out of scope:

- Vulnerabilities in third-party dependencies — please report those
  upstream. Dependabot already opens advisories for known CVEs in our
  dependency tree (see `.github/dependabot.yml`).
- Behavior of agents installed *by* AgentManager. Those are separate
  upstream projects with their own security policies.
- The cosmetic `ld: warning: ignoring duplicate libraries: '-lobjc'`
  emitted on macOS builds (no runtime impact; documented in
  CHANGELOG).

## Disclosure Timeline

Default coordinated-disclosure window is 90 days from acknowledgement.
We will work with you on timelines if active exploitation is observed
or if a fix needs more than that to land cleanly. Public advisories
are published via GitHub Security Advisories with credit to the
reporter (unless you ask to remain anonymous).

## Hardening Already in Place

These are the defensive measures already shipped — useful context for
researchers triaging behavior:

- gRPC server: keepalive, 16 MiB message-size caps, panic-recovery
  unary/stream interceptors (`pkg/api/grpc/server.go`).
- REST server: `ReadHeaderTimeout`, `MaxHeaderBytes`, detection cache
  to bound per-request work (`pkg/api/rest/server.go`).
- SQLite: WAL + `_busy_timeout` + `_synchronous=NORMAL`,
  `SetMaxOpenConns(1)`, `PRAGMA user_version` migration guard
  (`pkg/storage/sqlite.go`).
- Catalog refresh: ETag-aware `If-None-Match`; `singleflight`
  coalescing to avoid duplicate fetches (`pkg/catalog/manager.go`).
- IPC: short read deadline + ctx-aware receive loop in
  `listenForNotifications` (`pkg/ipc/ipc.go`).
- Static analysis: `gosec` runs on every PR (see
  `.github/workflows/ci.yml`). Findings against a PR's diff fail
  the build.
- Dependency hygiene: dependabot weekly with grouping
  (`.github/dependabot.yml`); patch + minor batched, security
  advisories per-package, majors per-package for review.
