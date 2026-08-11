# Research New AI CLI/TUI Agents for Catalog

You are tasked with researching the AI CLI/TUI agent ecosystem to discover tools that should be added to the AgentManager catalog. This skill performs proactive discovery, filtering, user-driven selection, and then sequential addition via the `/add-agent` workflow.

## Arguments

**Usage**: `/research-agents [timeframe] [additional context...]`

- **timeframe** (optional, default `1m`): How far back to look for new/updated tools. Accepted forms:
  - `1d`, `3d`, `5d`, `7d` — past N days
  - `1w`, `2w`, `4w` — past N weeks
  - `1m`, `2m`, `3m`, `6m` — past N months
  - `1y` — past year
  - `all` — no recency filter (use sparingly; will surface stale results)
- **additional context** (optional): Hints to focus the search:
  - "focus on Rust TUIs"
  - "exclude IDE plugins"
  - "must support Claude or GPT"
  - "minimum 1000 stars"

**Examples**:
```
/research-agents
/research-agents 1w
/research-agents 2m "focus on Go-based TUIs"
/research-agents 5d "high traction only, 500+ stars"
```

## Goal

Identify and propose **at least 5 new CLI/TUI agent tools** that are:
1. Not already present in `catalog.json`
2. Not present in the ignore list (`.claude/research-ignore.json`)
3. Genuinely a **CLI or TUI** (not a desktop GUI, SDK/library, IDE plugin, skill bundle, browser extension, web app, or hosted service)
4. Relevant to the catalog's theme (AI coding agents, agent multiplexers/orchestrators, agent harnesses, agent skill systems with CLI surfaces)
5. Created or meaningfully updated within the requested timeframe

If after a thorough search fewer than 5 qualify, that is acceptable — surface what was found, explain what was filtered out, and present the smaller list. **Never fabricate candidates to hit the minimum.**

## Workflow

### Phase 0: Setup — Read Local State

Before searching, build the filter set:

1. **Read `catalog.json`** to extract:
   - All existing agent `id`s
   - All existing agent `name`s (case-insensitive)
   - All existing `repository` URLs
   These will be used to dedupe candidates.

2. **Read `.claude/research-ignore.json`** if it exists. Schema:
   ```json
   {
     "ignored": [
       {
         "id": "<slug>",
         "name": "<display name>",
         "repository": "<repo URL if known>",
         "reason": "<why the user chose to ignore>",
         "ignored_at": "<YYYY-MM-DD>"
       }
     ]
   }
   ```
   If the file does not exist, treat as an empty ignore list. Do NOT create it yet — only create when the user opts to ignore something in Phase 4.

3. **Parse the timeframe argument** into a concrete cutoff date relative to today. Examples:
   - `1w` → 7 days ago
   - `1m` → 30 days ago
   - `3m` → 90 days ago

### Phase 1: Multi-Modal Discovery

Cast a wide net. **Do not rely on a single search angle** — different angles surface different tools. Use parallel queries when possible. Suggested angles:

1. **GitHub search by topic** — search topics relevant to the catalog:
   - `ai-agent`, `coding-agent`, `agent-cli`, `agent-tui`, `claude`, `gpt-cli`, `llm-cli`, `mcp`, `ratatui`, `bubbletea`, `tui`
   - Filter by recency: `--updated ">=YYYY-MM-DD"` or `--created ">=YYYY-MM-DD"`
   - Examples:
     ```bash
     gh search repos --topic=ai-agent --topic=cli --updated=">=2026-05-05" --sort=stars --limit=30
     gh search repos --topic=ratatui --updated=">=2026-05-05" --sort=stars --limit=20
     ```

2. **GitHub search by keyword** — keywords likely to appear in CLI/TUI agent READMEs:
   ```bash
   gh search repos "claude code alternative" --updated=">=YYYY-MM-DD" --sort=stars
   gh search repos "agent orchestrator cli" --updated=">=YYYY-MM-DD" --sort=stars
   gh search repos "tui ai coding" --updated=">=YYYY-MM-DD" --sort=stars
   ```

3. **Package registry sweeps** — fresh publishes are a strong signal:
   - **npm**: search for AI-agent CLI packages published within timeframe (use `npm search` or web-fetch npmjs.com)
   - **PyPI**: check newly published agent-related packages
   - **Homebrew**: check `homebrew-core` and popular taps for new AI CLI formulae/casks
   - **crates.io**: check newest AI/agent crates
   - **Go modules**: check `pkg.go.dev` for fresh AI CLI modules

4. **Web search for editorial signals**:
   - "new AI coding CLI 2026", "Claude Code alternatives", "best agent TUI"
   - Hacker News front-page, /r/LocalLLaMA, /r/ChatGPTCoding launches
   - Vendor announcements (Anthropic, OpenAI, Google, Vercel, Cursor)

5. **Cross-reference dependents** — for already-cataloged hub tools (Claude Code, OpenAI Codex CLI, Aider, etc.), check `Used by` / dependent repos for new derivatives.

Always prefer specific bash invocations (`gh search`, `gh api`) for verifiability over loose web inference.

### Phase 2: Filter Aggressively

For each candidate, apply this gate **before considering it qualified**:

1. **Type check** — is it actually a CLI or TUI?
   - **Accept**: standalone binaries, `npm i -g`, `pip install`, `brew install`, `cargo install`, `go install`, native installers, npx-runnable, pipx, uv-runnable
   - **Reject**: VS Code extensions, JetBrains plugins, Chrome extensions, web apps, desktop apps (Electron/Tauri without a CLI surface), pure libraries/SDKs, skill bundles consumed by other tools (not standalone), Telegram bots, Discord bots
   - **Gray area** (judgment call, surface with note): TUI servers, daemons with CLI control, hybrid CLI+desktop

2. **Dedupe** — drop if `id`, `name`, or `repository` matches anything in catalog.json or the ignore list

3. **Recency** — confirm `pushed_at` (and ideally a recent commit or release) falls within the timeframe

4. **Viability** — drop if:
   - Repo is archived or explicitly sunsetted
   - Author restricts use (e.g., "do not use unless you are X")
   - Project is clearly abandoned (last release >1yr, open issues unanswered for months)
   - License is proprietary/restrictive AND the project is a single-user experiment

5. **Quality signals** (use as ranking, not hard cutoffs):
   - Star count, fork count
   - Activity (recent commits, releases)
   - Documentation quality
   - Distinct value (not a fork of an existing entry with no changes)

### Phase 3: Present Candidates to User

Present a table with these columns:
- **#** — selection index
- **Name** — display name
- **Repo / Source** — `owner/repo` or homepage
- **Stars** — if applicable
- **License** — MIT, Apache-2.0, etc., or "Unspecified"
- **Install** — primary method (npm, pip, brew, cargo, go install, curl-sh, etc.)
- **What** — one-line description
- **Updated** — last push or release date

After the table, give a short summary of:
- How many candidates were surfaced vs. filtered out
- Notable items that were filtered (e.g., "X was excluded — it's a VS Code extension, not a CLI")

Then use the **`AskUserQuestion` tool** to ask which to add.

**Important — handling the 4-option limit**: `AskUserQuestion` allows 2–4 options per question. For >4 candidates:
- Group candidates into batches of up to 4 per question
- Use multiple questions in a single `AskUserQuestion` tool call (the tool allows up to 4 questions per call), each with `multiSelect: true`
- Label questions clearly: "Candidates 1–4 — select all to add", "Candidates 5–8 — select all to add", etc.

Example shape (5 candidates):
```
Question 1: "Candidates 1–4 — which should be added?"
  Header: "Add 1-4"
  multiSelect: true
  Options: [#1 Name, #2 Name, #3 Name, #4 Name]

Question 2: "Candidate 5 — should it be added?"
  Header: "Add 5"
  multiSelect: false
  Options: ["Yes, add", "No, skip"]
```

### Phase 4: Handle Non-Selected Candidates

After collecting the user's selections, identify candidates the user did NOT pick. Then use **`AskUserQuestion`** to ask whether to ignore them in future searches:

```
Question: "Ignore the non-selected candidates in future /research-agents runs?"
Header: "Ignore?"
multiSelect: false
Options:
  - "Yes, ignore all non-selected" — Append all to .claude/research-ignore.json
  - "Let me pick which to ignore" — Branch into a per-candidate decision
  - "No, don't ignore any" — Leave the ignore list unchanged
```

If "Let me pick", batch the non-selected into groups of 4 and ask via multiSelect (same batching pattern as Phase 3).

For each candidate the user opts to ignore, append to `.claude/research-ignore.json`:
- If the file does not exist, create it with the schema above
- Use the current date for `ignored_at`
- Always include a `reason`. If the user didn't state one, infer from context (e.g., "Not a CLI/TUI — desktop GUI", "Stale repo, no recent activity", "Skipped during 2026-06-05 research run") and note that it was an inferred reason
- Validate JSON syntax after writing

### Phase 5: Sequential Addition via /add-agent

For each candidate the user selected to add:

1. Announce: "Adding <name> via /add-agent..."
2. Invoke the `/add-agent` skill via the `Skill` tool with the candidate's name and primary URL, plus any context discovered during research (install method, license, etc.) to seed the workflow
3. Wait for `/add-agent` to complete (it will run its own AskUserQuestion flows for approval, version bump, and commit) before moving to the next
4. If `/add-agent` errors or the user rejects mid-flow for one candidate, continue to the next — do not abort the whole batch
5. Track which were added vs. skipped vs. failed

### Phase 6: Wrap Up

Before summarizing, confirm the embedded catalog is in sync. `/add-agent` is
responsible for running `make sync-catalog`, but verify it actually happened —
CI fails the build when `catalog.json` and `pkg/catalog/catalog.json` drift:

```bash
make check-catalog-sync   # must pass; if not, run `make sync-catalog` and amend/commit
```

After processing all selected candidates, print a final summary:

```
Research run complete:
  Timeframe: <timeframe>
  Surfaced: <N> candidates
  Added: <list of added agent ids>
  Skipped this run: <list>
  Added to ignore list: <list>
  Final catalog size: <count> agents
```

## Critical Requirements

1. **NEVER fabricate or hallucinate candidates** — every candidate must be a real repo/package with a verifiable URL
2. **Verify CLI/TUI status before presenting** — do not pad the list with GUIs or SDKs
3. **Respect the ignore list** — items in `.claude/research-ignore.json` must never be re-surfaced (filter them out silently; do not mention them unless asked)
4. **Respect existing catalog** — items already in `catalog.json` (by id, name, or repository) must be excluded
5. **Cite sources** — when presenting a candidate, the user must be able to click through to verify
6. **Be honest about misses** — if the search returned <5 qualifying candidates, say so. Do not stretch the criteria silently.
7. **Use the `/add-agent` skill for actual catalog modification** — do not duplicate its logic inline. This skill researches; `/add-agent` adds.
8. **Atomic ignore list writes** — when updating `research-ignore.json`, read first, append, write back. Do not overwrite-on-write.

## Ignore List File

**Location**: `/Users/kevin/Cloud/Dropbox/work/kevinelliott/agent-manager/.claude/research-ignore.json`

**Schema**:
```json
{
  "ignored": [
    {
      "id": "<lowercase-hyphenated-slug>",
      "name": "<display name>",
      "repository": "<full GitHub or homepage URL>",
      "reason": "<one-line reason — required>",
      "ignored_at": "<YYYY-MM-DD>"
    }
  ]
}
```

The ignore list is for items the **user has explicitly chosen to deprioritize**. It is not a blacklist for known-bad tools — those should simply not be surfaced.

## Notes on `gh search repos` Date Flags

- Use `--updated=">=YYYY-MM-DD"` to filter by repo update time (last push activity)
- Use `--created=">=YYYY-MM-DD"` to filter by repo creation time
- `--pushed` is NOT a valid flag — use `--updated` instead
- Combine with `--sort=stars --limit=N` to get the most-starred recent repos

## Workflow Summary

1. Parse timeframe and any additional context
2. Read `catalog.json` + `.claude/research-ignore.json` (if it exists)
3. Multi-angle discovery sweep (parallel where possible)
4. Filter for CLI/TUI status, dedupe, recency, viability
5. Present table; ask user (batched multiSelect) which to add
6. Ask user whether to ignore non-selected; update ignore list
7. For each selected: invoke `/add-agent` sequentially
8. Print final summary

---

**Ready to research!** Provide a timeframe and (optionally) focus context:
`/research-agents [timeframe] [additional context...]`
