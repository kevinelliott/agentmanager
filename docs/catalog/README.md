# Catalog research and update records

AgentManager ships with 269 catalog entries in catalog version 1.0.87. The
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
The seven batches below required at least 100 stars and activity within the two
months before verification. Star counts and distribution versions are snapshots.

| Issue | Added | Catalog version | Evidence |
|-------|-------|-----------------|----------|
| ENGIN-2 | 24 | 1.0.81 | [October 16 research](research-2026-10-16.csv) |
| ENGIN-3 | 20 | 1.0.82 | [October 18 research](research-2026-10-18.csv) |
| ENGIN-4 | 20 | 1.0.83 | [October 18 follow-up research](research-2026-10-18-engin-4.csv) |
| ENGIN-5 | 20 | 1.0.84 | [October 18 additional research](research-2026-10-18-engin-5.csv) |
| ENGIN-6 | 20 | 1.0.85 | [October 18 latest research](research-2026-10-18-engin-6.csv) |
| ENGIN-7 | 20 | 1.0.86 | [October 18 expanded research](research-2026-10-18-engin-7.csv) |
| ENGIN-8 | 20 | 1.0.87 | [October 18 MCP and CLI research](research-2026-10-18-engin-8.csv) |

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
CodexBar here is the CLI formula; Playwright CLI, Playwright MCP and Playwright Test are separate
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
upgraded or rebuilt before installing FastMCP or Langflow through these methods;
refreshing the catalog alone does not update the application. Until a binary
release includes it, use the [source build instructions](../../README.md#from-source)
with the latest main branch.

The ENGIN-5 batch uses published npm/uv packages and Homebrew formulas:

- Context7 CLI is `ctx7`; E2B CLI is `@e2b/cli`; OpenAgents Launcher is
  `@openagents-org/agent-launcher` and runs as `agn`. These are distinct from
  their SDKs, desktop applications or separate MCP server packages.
- LangGraph installs its base CLI. Container workflows need Docker, while
  `langgraph dev` needs the optional `inmem` extra and Python 3.11+.
  MarkItDown also installs its base CLI; additional formats need optional extras
  and sometimes external converters.
- OpenViking's verified wheels require macOS 14+ on arm64 or 15+ on x64,
  glibc 2.31+ on Linux x64/arm64, or Windows x64. Serena's upstream quick start
  selects Python 3.13; set `UV_PYTHON=3.13` in the environment to choose it when
  installing through AgentManager, then initialize its backend separately.
- Autohand's npm package also publishes generic `agent` and `ah` aliases;
  check for conflicts with other CLIs. AgentManager detects it as `autohand`.
- Ollama and llama.cpp install local model tools. Model downloads, server
  startup and optional services remain explicit steps, as do Crawl4AI browser
  setup, sandbox provisioning and evaluation runs.

The ENGIN-6 batch adds MCP management, evaluation, context and session tools:

- TOON CLI uses `@toon-format/cli` and requires Node 24+. MCPJam Inspector
  uses `@mcpjam/inspector` and runs as `inspector`; it is distinct from the
  official `mcp-inspector`. Use a current Node runtime for SDK dependencies.
- MCO uses `@tt-a1i/mco`, whose wrapper needs `python3` (Python 3.10+) on
  PATH. Its bundled Python sources are separate from the unrelated `mco`
  PyPI distribution. This catalog method supports macOS/Linux.
- Skillshare uses the Homebrew CLI formula. amux uses the upstream
  `andyrewlee/amux/amux` formula, builds with Go, and requires Git and
  tmux 3.2+. Synchronizing skills, creating workspaces and starting agents
  are separate operations.
- RuVector needs a working native backend for persistent vector storage.
  Its core binaries cover macOS x64/arm64, glibc Linux x64/arm64 and Windows
  x64. Check `ruvector info`; semantic commands download an embedding model
  on first use. Cisco Skill Scanner wheels require CPython 3.11–3.14 and
  support glibc Linux x64/arm64, macOS 14+ x64/arm64 and Windows x64; source
  builds on other platforms need Go 1.27.1+.
- Context Mode's upstream npm postinstall can repair existing Claude plugin
  registries/settings and normalize hooks during installation or updates.
  Agent adapter and MCP registration still need separate configuration.
- Snyk Agent Scan uses the `snyk-agent-scan` Python package and needs an
  account/token. Scans can start configured MCP commands, contact servers
  and submit component metadata for analysis; follow upstream consent and
  data-handling guidance. Model-backed evaluations, Firecrawl requests and
  Sentry AI features also require explicit configuration and service access.
- Sentry CLI here is npm `sentry` from `getsentry/toolkit`, distinct from
  `@sentry/cli` and its `sentry-cli` executable. Claude Code Log and Claude
  Replay generate artifacts containing session content; review before sharing.

The ENGIN-7 batch adds code context, browser/MCP tools and agent workflows:

- Code2Prompt and grepai prepare and retrieve repository context. grepai uses
  `yoanbernabeu/tap/grepai` with x64/arm64 macOS/Linux binaries and needs an
  embedding provider. Configure it and index selected repositories explicitly.
- Fabric uses Homebrew `fabric-ai`, including the executable name `fabric-ai`.
  It is distinct from the Python Fabric SSH tool. Browserbase Browse uses npm
  `browse` from the Stagehand repository, distinct from the SDK and legacy
  `bb` CLI. Its pinned Stagehand dependency requires Node 22.18+. Configure
  a local browser or cloud credentials separately.
- Pydantic AI CLI uses `clai` (Python 3.11+) on the upstream POSIX terminal
  platforms. Cisco MCP Scanner uses `cisco-ai-mcp-scanner`; upstream selects
  Python 3.13, which can be chosen with `UV_PYTHON=3.13`. Live-server scanning
  can start configured commands or contact servers; select targets and engines
  explicitly. The official MCP Filesystem Server also needs explicit allowed
  directories and client configuration before use.
- Langflow requires Python 3.10–3.14. Its `langflow` metapackage exposes the
  `langflow-base` dependency's CLI with `with_executables_from`, preserving
  package inventory, upgrades and removal. Older binaries need the source
  build or a release containing the ENGIN-4 uv metadata support described above.
- MLX LM's base method targets Apple silicon macOS 14+ and Python 3.11+.
  Linux CPU/CUDA extras are separate. Magentic UI's current MagenticLite release
  tests macOS arm64 and Windows x64 in WSL2; native Linux x64 is expected but
  untested upstream. Run its Linux method inside WSL on Windows, and configure
  the Quicksand VM, Linux KVM access and model credentials separately.
- Chroma's `chromadb` package supplies `chroma`; native wheels cover macOS
  x64/arm64, glibc Linux x64/arm64 and Windows x64. Dependencies can impose
  stricter OS/runtime requirements. Phoenix uses `arize-phoenix` and Python
  3.11–3.14; Hayhooks uses Python 3.10–3.14. Storage, data ingestion, tracing
  instrumentation, pipeline dependencies and server startup are explicit steps.
- n8n requires Node 24+. OpenAgents Control needs Bash, Git and OpenCode
  (Git Bash or WSL on Windows); the npm wrapper does not install its agent
  patterns until invoked. Apify also publishes `actor` and `apify-cli` aliases.
  Its `which` 7 dependency requires Node 22.22.2+ on 22.x, 24.15+ on 24.x,
  or 26+, despite the CLI's broader declared Node 20+ requirement.
  Select workflow activation, project setup, cloud deployment and model or
  browser downloads explicitly; installation does not run these operations.

The ENGIN-8 batch adds browser verification, research and service MCP tools:

- Playwright Test uses `@playwright/test` and supplies `playwright`; browser
  downloads, Linux libraries and project tests require separate setup. Chrome
  DevTools MCP requires a supported Chrome browser. Its usage statistics are
  enabled upstream by default; start it with `--no-usage-statistics` to opt out.
- Tavily, Exa, Brave Search, Perplexity and Firecrawl MCP need their provider
  credentials and MCP client configuration. Service requests can consume credits.
  Firecrawl MCP is separate from Firecrawl CLI. Use a current Node LTS runtime;
  Firecrawl and Neon require Node 22+, Azure requires Node 22+, and DBHub
  requires Node 22.5+. MongoDB supports Node 20.19+, 22.13+, or 24+.
- DBHub uses `@bytebase/dbhub`, not the unrelated unscoped package. Supabase's
  canonical source is `supabase/mcp`. Azure (`azmcp`) and Kubernetes use native
  optional npm packages for macOS/Linux/Windows x64 and arm64; retain optional
  dependencies and satisfy upstream OS library requirements.
- Hugging Face uses Python `huggingface-hub` and the `hf` executable. AWS
  Documentation MCP uses the normalized uv package name
  `awslabs-aws-documentation-mcp-server` while its executable retains dots:
  `awslabs.aws-documentation-mcp-server`. An isolated Python 3.12 installation
  verified this inventory mapping. Both require Python 3.10+.
- Redis and Qdrant also publish Python 3.10+ distributions. Qdrant needs a
  supported native embedding runtime; first use can download an embedding model.
  Configure endpoints, collections, database permissions and model choices
  separately. GitHub, Terraform and Grafana use macOS/Linux Homebrew formulas.
- Set account/project/cluster scope, authentication, read-only settings and tool
  selection before starting service MCP servers. Enabled tools may modify
  resources. Installing a package does not register an MCP client, authenticate,
  start services, provision resources or perform database operations.

## Inclusion announcements

The catalog update pull request records upstream announcements with links and
per-repository outcomes. Announcements use an appropriate upstream discussion
category or existing thread, identify the included tool, and link to
[AgentManager](https://github.com/kevinelliott/agentmanager). Existing announcements
are retained without duplicates. Repositories without a suitable venue are
recorded as skipped; unrelated bug reports are not used for announcements.

The initial ENGIN-2 outreach record is on
[PR #67](https://github.com/kevinelliott/agentmanager/pull/67#issuecomment-6216845583).
The [ENGIN-4 completion record](https://github.com/kevinelliott/agentmanager/pull/69#issuecomment-6247777800)
covers its 64 recent additions. The [ENGIN-5 record](https://github.com/kevinelliott/agentmanager/pull/70#issuecomment-6248132465)
covers 84 additions. The [ENGIN-6 record](https://github.com/kevinelliott/agentmanager/pull/71#issuecomment-6248466338)
covers 104 additions. The [ENGIN-7 record](https://github.com/kevinelliott/agentmanager/pull/72#issuecomment-6248707616)
covers 124 additions. The ENGIN-8 update expands the audit to 144 additions
across ENGIN-2 through ENGIN-8, preserving existing notices without duplicates.
The update PR records new announcements, unsuitable venues and any denied
posting permissions.
