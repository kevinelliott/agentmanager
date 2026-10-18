# Catalog research and update records

AgentManager ships with 189 catalog entries in catalog version 1.0.83. The
[root catalog](../../catalog.json) and [embedded catalog](../../pkg/catalog/catalog.json)
contain the same definitions. The [supported agents table](../../README.md#supported-agents)
lists each entry's ID and installation methods.

Use `agentmgr catalog refresh --force` to fetch the published catalog, then
`agentmgr catalog show ID` to inspect a tool's installation methods, supported
platforms and setup notes before installing it.

## Research evidence

Each research record captures the canonical GitHub repository and repository ID,
star count and verification time, recent activity evidence, upstream installation
source, distribution version, executable names, supported platforms and relevance.
The three batches below required at least 100 stars and activity within the two
months before verification. Star counts and distribution versions are snapshots.

| Issue | Added | Catalog version | Evidence |
|-------|-------|-----------------|----------|
| ENGIN-2 | 24 | 1.0.81 | [October 16 research](research-2026-10-16.csv) |
| ENGIN-3 | 20 | 1.0.82 | [October 18 research](research-2026-10-18.csv) |
| ENGIN-4 | 20 | 1.0.83 | [October 18 follow-up research](research-2026-10-18-engin-4.csv) |

Recent activity is identified as a default-branch commit or a repository push in
the CSV. A push record does not imply that the default branch changed recently.
Installation evidence includes package metadata, published executable mappings
and upstream Homebrew formulas where applicable.

## Installation and setup

Catalog platform lists describe the available installation method. Architecture,
OS version and runtime restrictions appear in each entry's `metadata.notes` and
research record. For example, Peekaboo requires macOS 15+, and the current CASS
Homebrew formula supports Apple Silicon macOS and x64/arm64 Linux.

Installing a CLI does not complete its account authentication, project
initialization, session indexing, browser downloads, MCP client configuration or
optional agent hooks. Follow the linked upstream documentation for these steps.
CodexBar here is the CLI formula; Playwright CLI and Playwright MCP are separate
tools; Beads Rust supplies `br` and is separate from Beads.

Amplifier uses the upstream Git distribution through a native uv adapter on
macOS/Linux. Its source installation has no automatic release checks; use
`agentmgr agent update amplifier --force` to pull upstream changes. FastMCP 4
uses uv with `with_executables_from` installation metadata to expose the
`fastmcp-slim` dependency's CLI while retaining normal package inventory, update
checks and removal. Commands that might start an application are omitted from
automatic version probing.

For uv catalog methods, `metadata.with_executables_from` accepts uv's
comma-separated package list and is passed as a single installation argument.
Keep `package` set to the main distribution. uv retains the executable setting
for subsequent `tool upgrade` operations; inventory and removal also use the
main distribution name. See the [uv tool documentation](https://docs.astral.sh/uv/concepts/tools/#installing-executables-from-additional-packages).
This support was introduced with ENGIN-4. Older AgentManager binaries must be
upgraded or rebuilt before installing FastMCP through this catalog method;
refreshing the catalog alone does not update the application. Until a binary
release includes it, use the [source build instructions](../../README.md#from-source)
with the latest main branch.

## Inclusion announcements

The catalog update pull request records upstream announcements with links and
per-repository outcomes. Announcements use an appropriate upstream discussion
category or existing thread, identify the included tool, and link to
[AgentManager](https://github.com/kevinelliott/agentmanager). Existing announcements
are retained without duplicates. Repositories without a suitable venue are
recorded as skipped; unrelated bug reports are not used for announcements.

The initial ENGIN-2 outreach record is on
[PR #67](https://github.com/kevinelliott/agentmanager/pull/67#issuecomment-6216845583).
The ENGIN-4 completion record covers the ENGIN-2, ENGIN-3 and ENGIN-4 additions
merged within the preceding 48 hours.
