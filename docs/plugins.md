# Detection Plugins

AgentManager supports custom detection plugins that allow you to extend agent discovery with your own logic. This is useful for detecting agents installed via non-standard methods like Docker, custom build systems, or proprietary package managers.

## Plugin Location

Plugins are stored as JSON files in the plugins directory:

- **macOS/Linux**: `~/.config/agentmgr/plugins/`
- **Windows**: `%APPDATA%\agentmgr\plugins\`

Each plugin file must have the `.plugin.json` extension.

## Plugin Structure

```json
{
  "name": "my-plugin",
  "description": "Detects agents running in Docker containers",
  "method": "docker",
  "platforms": ["darwin", "linux"],
  "detect_script": "docker ps --format json | jq ...",
  "agent_filter": ["specific-agent-id"],
  "enabled": true
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique identifier (lowercase, alphanumeric, hyphens, underscores) |
| `description` | string | No | Human-readable description |
| `method` | string | Yes | Installation method this plugin detects (e.g., "docker", "custom") |
| `platforms` | string[] | No | Platforms to run on (empty = all platforms) |
| `detect_command` | string | * | External command to run for detection |
| `detect_script` | string | * | Inline shell script for detection |
| `agent_filter` | string[] | No | Agent IDs to detect (empty = all agents) |
| `enabled` | boolean | Yes | Whether the plugin is active |

\* Either `detect_command` or `detect_script` must be provided.

## Detection Output Format

Your detection script/command must output JSON in the following format:

```json
{
  "agents": [
    {
      "agent_id": "claude-cli",
      "version": "1.2.3",
      "executable_path": "/usr/local/bin/claude",
      "install_path": "/opt/agents/claude",
      "metadata": {
        "container_id": "abc123",
        "image": "anthropic/claude:latest"
      }
    }
  ]
}
```

### Output Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent_id` | string | Yes | Must match an agent ID in the catalog |
| `version` | string | Yes | Detected version (semver format) |
| `executable_path` | string | No | Path to the agent executable |
| `install_path` | string | No | Installation directory |
| `metadata` | object | No | Additional key-value metadata |

## Environment Variables

The following environment variables are available to your script:

| Variable | Description |
|----------|-------------|
| `AGENTMGR_AGENT_IDS` | Comma-separated list of agent IDs to detect |
| `AGENTMGR_PLATFORM` | Current platform (darwin, linux, windows) |

## CLI Commands

### List Plugins

```bash
agentmgr plugin list
```

### Create a Plugin

```bash
agentmgr plugin create docker-agents \
  --method docker \
  --description "Detect agents in Docker containers" \
  --script 'docker ps --format json | jq ...'
```

### Validate a Plugin

```bash
agentmgr plugin validate my-plugin.plugin.json
```

### Enable/Disable Plugins

```bash
agentmgr plugin enable docker-agents
agentmgr plugin disable docker-agents
```

## Example Plugins

### Docker Container Detection

```json
{
  "name": "docker-agents",
  "description": "Detects AI agents running in Docker containers",
  "method": "docker",
  "platforms": ["darwin", "linux"],
  "detect_script": "docker ps --filter 'label=agentmgr.agent' --format '{{json .}}' | jq -s '{agents: [.[] | {agent_id: .Labels[\"agentmgr.agent\"], version: .Labels[\"agentmgr.version\"], metadata: {container_id: .ID, image: .Image}}]}'",
  "enabled": true
}
```

### Nix Package Detection

```json
{
  "name": "nix-agents",
  "description": "Detects agents installed via Nix",
  "method": "nix",
  "platforms": ["darwin", "linux"],
  "detect_command": "/usr/local/bin/detect-nix-agents.sh",
  "enabled": true
}
```

### Development Build Detection

```json
{
  "name": "dev-builds",
  "description": "Detects locally built development versions",
  "method": "dev",
  "detect_script": "find ~/projects -name 'agentmgr' -type f -executable 2>/dev/null | while read f; do echo \"{\\\"agent_id\\\": \\\"agentmgr\\\", \\\"version\\\": \\\"dev\\\", \\\"executable_path\\\": \\\"$f\\\"}\"; done | jq -s '{agents: .}'",
  "agent_filter": ["agentmgr"],
  "enabled": true
}
```

## Security Considerations

- Plugins execute arbitrary commands with your user privileges
- Only install plugins from trusted sources
- Review plugin scripts before enabling them
- Plugins are opt-in and must be explicitly created or installed

## Troubleshooting

### Plugin Not Running

1. Check if the plugin is enabled: `agentmgr plugin list`
2. Verify the platform matches: check `platforms` field
3. Run the script manually to test output format

### Invalid Output

Run your script manually and verify it outputs valid JSON:

```bash
# Test your script
your-script | jq .
```

### Plugin Not Found

Ensure the file:
1. Is in the correct directory
2. Has the `.plugin.json` extension
3. Is valid JSON (test with `agentmgr plugin validate`)
