# Add AI CLI Agent to Catalog

You are tasked with researching and adding a new AI CLI tool to the AgentManager catalog. This requires thorough investigation to ensure accurate information.

## Arguments

**Usage**: `/add-agent <name> <url> [additional context...]`

- **name** (required): The name of the AI CLI tool (e.g., "Claude Code", "Aider")
- **url** (required): A documentation URL, GitHub repository, or homepage for the tool
- **additional context** (optional): Any additional information, hints, or context to help with research. This could include:
  - Known installation methods ("available on npm as @foo/bar")
  - Platform support notes ("macOS and Linux only")
  - Related tools or alternatives
  - Specific features to highlight
  - Any caveats or special considerations

**Examples**:
```
/add-agent Aider https://aider.chat
/add-agent "Codex CLI" https://github.com/openai/codex "npm package is @openai/codex, also has brew cask"
/add-agent Droid https://docs.factory.ai/cli "by Factory AI, has native installer and Homebrew cask"
```

## Research Process

### Phase 1: Initial Research

1. **Fetch the provided URL** to understand the tool's purpose and features
2. **Consider any additional context** provided by the user
3. **Search for additional information** about the tool:
   - Official documentation
   - GitHub repository (if different from provided URL)
   - Package registry pages (npm, PyPI, Homebrew, etc.)
   - Installation guides
   - Blog posts or announcements

### Phase 2: Installation Methods Discovery

For each potential installation method, verify:

1. **npm** - Check if package exists on npmjs.com
   - Package name (e.g., `@org/package` or `package`)
   - Verify the package is the official one (check publisher, downloads, links)
   - Note any prerequisites (Node.js version)

2. **pip/pipx/uv** - Check if package exists on PyPI
   - Package name
   - Verify it's the official package
   - Note Python version requirements

3. **Homebrew** - Check if formula/cask exists
   - Search formulae.brew.sh or homebrew-core/homebrew-cask repos
   - Determine if it's a formula or cask
   - Note tap if not in core (e.g., `brew tap org/tap`)

4. **Native installer** - Look for:
   - Shell script installers (`curl ... | sh`)
   - Direct binary downloads
   - Platform-specific installers (.dmg, .exe, .deb, .rpm)

5. **Other methods** - Check for:
   - `go install` support
   - `cargo install` support
   - winget/scoop/chocolatey for Windows
   - Binary releases on GitHub
   - krew for kubectl plugins
   - nix packages

### Phase 3: Detection Signatures

Determine how to detect the installed tool:

1. **Executable names** - What binary names does it install? Check multiple possible names.
2. **Version command** - How to get version? Common patterns:
   - `tool --version`
   - `tool -v`
   - `tool version`
   - `tool -V`
3. **Version regex** - Pattern to extract version number from output
4. **Method-specific signatures** - How to identify which method was used:
   - npm: `npm list -g <package> --json`
   - pip: Check site-packages or pip show
   - brew: Check Cellar/Caskroom paths
   - native: Common install paths

### Phase 4: Changelog Sources

Identify where release information can be found:
- GitHub releases API (preferred)
- CHANGELOG.md file in repo
- Release notes page
- Blog announcements

## Output Format

Generate a JSON entry for the catalog following this structure:

```json
"<agent-id>": {
  "id": "<agent-id>",
  "name": "<Display Name>",
  "description": "<Brief description - what it does, under 80 chars>",
  "homepage": "<Official homepage URL>",
  "repository": "<GitHub repository URL if available>",
  "documentation": "<Documentation URL if available>",
  "install_methods": {
    "<method>": {
      "method": "<method>",
      "package": "<package name if applicable>",
      "command": "<install command>",
      "update_cmd": "<update command>",
      "uninstall_cmd": "<uninstall command>",
      "platforms": ["darwin", "linux", "windows"],
      "global_flag": "<flag if applicable>",
      "prereqs": ["<prerequisites>"],
      "metadata": {
        "<key>": "<string value>"
      }
    }
  },
  "detection": {
    "executables": ["<binary-name>", "<alt-name>"],
    "version_cmd": "<command to get version>",
    "version_regex": "<regex to extract version - escape backslashes>",
    "signatures": {
      "<method>": {
        "check_cmd": "<command to verify installation method>",
        "paths": ["<common install paths>"]
      }
    }
  },
  "changelog": {
    "type": "github_releases",
    "url": "<GitHub releases API URL>",
    "file_format": "markdown"
  },
  "metadata": {
    "vendor": "<Company/Organization>",
    "license": "<License type>"
  }
}
```

## Critical Requirements

1. **NO FALSE INFORMATION**: If you cannot verify something, mark it as uncertain or omit it. When in doubt, leave it out.

2. **Verify package names**:
   - For npm: Check npmjs.com directly, verify publisher matches official org
   - For pip: Check pypi.org directly
   - For brew: Check formulae.brew.sh or search the homebrew repos

3. **Test commands mentally**: Ensure install commands are syntactically correct and follow conventions

4. **Platform accuracy**: Only list platforms that are actually supported and tested

5. **Metadata values must be strings**: All values in `metadata` objects MUST be strings:
   - WRONG: `"cask": true`
   - RIGHT: `"cask": "true"`
   - WRONG: `"artifacts": { "darwin": "file.tar.gz" }`
   - RIGHT: `"artifact_darwin": "file.tar.gz"`

6. **Escape regex properly**: Version regex patterns need escaped backslashes for JSON:
   - Pattern `(\d+\.\d+\.\d+)` becomes `"([\\d.]+)"` in JSON

7. **Use consistent agent IDs**:
   - Lowercase, hyphenated
   - Match the primary command name when possible
   - Examples: `claude-code`, `aider`, `copilot-cli`

## Research Confidence Levels

After researching, indicate confidence for each finding:
- **HIGH**: Verified from official sources, documentation, or package registries
- **MEDIUM**: Found in multiple reliable sources but not officially documented
- **LOW**: Single source or inferred from patterns

## Workflow

1. Parse the user's input to extract name, URL, and any additional context
2. Fetch and analyze the provided URL
3. Conduct additional research using web search and package registry checks
4. Compile findings with confidence levels
5. Present a summary of the proposed catalog entry
6. **Use the `AskUserQuestion` tool** to ask the user to approve adding the agent:
   - Question: "Add this agent to the catalog?"
   - Options: "Yes, add it" / "No, needs changes"
7. If approved, add the entry to `catalog.json` in alphabetical order by agent ID
8. Validate JSON syntax after editing
9. **Use the `AskUserQuestion` tool** to ask about version bump:
   - Question: "Bump the catalog version?"
   - Options: "Yes, bump patch version" / "Yes, bump minor version" / "No, keep current version"
10. If version bump requested, update the `"version"` field in `catalog.json` accordingly:
    - Patch bump: `1.0.0` → `1.0.1`
    - Minor bump: `1.0.0` → `1.1.0`
11. Also update the `"last_updated"` field to the current date

## Using AskUserQuestion Tool

When asking for user confirmation, use the `AskUserQuestion` tool with appropriate options. Examples:

**For agent approval:**
```
Question: "Does this catalog entry look correct for <agent-name>?"
Header: "Add agent"
Options:
  - "Yes, add to catalog" - Add the agent entry as shown
  - "No, needs changes" - I'll describe what needs to be modified
```

**For version bump:**
```
Question: "Would you like to bump the catalog version?"
Header: "Version"
Options:
  - "Patch bump (x.x.X)" - Increment patch version for additions
  - "Minor bump (x.X.0)" - Increment minor version for significant changes
  - "No change" - Keep the current version
```

---

**Ready to research!** Please provide the AI CLI tool details in the format:
`/add-agent <name> <url> [additional context...]`
