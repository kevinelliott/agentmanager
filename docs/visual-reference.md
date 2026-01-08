# AgentManager Visual Reference Guide

## Color Palette Reference

### Primary Colors

```
Success (Green):      #00D787  RGB(0, 215, 135)   HSL(158, 100%, 42%)
Warning (Orange):     #FFB86C  RGB(255, 184, 108)  HSL(31, 100%, 71%)
Error (Red):          #FF5555  RGB(255, 85, 85)    HSL(0, 100%, 67%)
Info (Cyan):          #8BE9FD  RGB(139, 233, 253)  HSL(191, 97%, 77%)
Muted (Gray):         #6272A4  RGB(98, 114, 164)   HSL(225, 27%, 51%)
```

### Brand Colors

```
Primary (Purple):     #BD93F9  RGB(189, 147, 249)  HSL(265, 89%, 78%)
Secondary (Green):    #50FA7B  RGB(80, 250, 123)   HSL(135, 94%, 65%)
Border (Dark Gray):   #44475A  RGB(68, 71, 90)     HSL(232, 14%, 31%)
Background (Dark):    #282A36  RGB(40, 42, 54)     HSL(231, 15%, 18%)
Foreground (White):   #F8F8F2  RGB(248, 248, 242)  HSL(60, 30%, 96%)
```

---

## Symbol Reference

### Status Indicators

```
✓  Success (U+2713)
✗  Failure (U+2717)
●  Active/Running (U+25CF)
○  Inactive/Disabled (U+25CB)
▸  Selected/Current (U+25B8)
•  List bullet (U+2022)
…  Truncated/More (U+2026)
```

### Progress Indicators

```
█  Full block (U+2588)
▓  Dark shade (U+2593)
▒  Medium shade (U+2592)
░  Light shade (U+2591)
▌  Left half (U+258C)
```

### Box Drawing

```
┌─┬─┐  Top borders
├─┼─┤  Middle borders
└─┴─┘  Bottom borders
│     Vertical
─     Horizontal

╭─╮   Rounded top
╰─╯   Rounded bottom
```

### Arrows

```
→  Right arrow (U+2192)
←  Left arrow (U+2190)
↑  Up arrow (U+2191)
↓  Down arrow (U+2193)
```

---

## Detailed TUI Mockups

### 1. Main Dashboard (120x40 terminal)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ AgentManager v1.0.0                                                                          [?] Help [q] Quit       │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── Status ──────────────────────────────────────────────────┐ ┌── Quick Actions ──────────────────────────────────┐
│                                                            │ │                                                   │
│  Total Agents:        4                                    │ │  [i] Install Agent                                │
│  Up to Date:          3                                    │ │  [u] Update Selected                              │
│  Updates Available:   1                                    │ │  [U] Update All                                   │
│  Systray Helper:      ● Running                            │ │  [c] Browse Catalog                               │
│  Last Checked:        2 minutes ago                        │ │  [s] Settings                                     │
│                                                            │ │  [r] Refresh                                      │
│  Next Auto Check:     in 3h 58m                           │ │                                                   │
│                                                            │ │                                                   │
└────────────────────────────────────────────────────────────┘ └───────────────────────────────────────────────────┘

┌── Installed Agents ────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  ▸ claude-desktop        v1.2.3 → v1.2.5                              Update Available                           │
│    • Official Claude desktop application by Anthropic                                                            │
│    • Installed: 2025-12-15  •  Method: dmg  •  Size: 287.4 MB                                                    │
│                                                                                                                    │
│    cursor                v0.42.0                                      Up to date                                 │
│    • The AI-first Code Editor                                                                                     │
│    • Installed: 2025-11-20  •  Method: app-bundle  •  Size: 412.8 MB                                             │
│                                                                                                                    │
│    zed                   v0.165.2                                     Up to date                                 │
│    • High-performance, multiplayer code editor                                                                    │
│    • Installed: 2025-10-05  •  Method: homebrew  •  Size: 98.3 MB                                                │
│                                                                                                                    │
│    windsurf              v1.0.1                                       Up to date                                 │
│    • AI-powered code editor by Codeium                                                                            │
│    • Installed: 2026-01-04  •  Method: app-bundle  •  Size: 523.1 MB                                             │
│                                                                                                                    │
│  [↑↓] Navigate  [Enter] Details  [u] Update  [d] Delete  [/] Filter                                              │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Recent Activity ─────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  2 minutes ago          Checked for updates (1 found)                                                             │
│  1 day ago              Updated claude-desktop: 1.2.2 → 1.2.3                                                     │
│  3 days ago             Installed windsurf v1.0.1                                                                 │
│  1 week ago             Updated cursor: 0.41.0 → 0.42.0                                                           │
│  2 weeks ago            Updated zed: 0.164.0 → 0.165.2                                                            │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 Press [?] for keyboard shortcuts • [i] to install agents • [u] to update selected • [U] to update all
```

### 2. Agent Detail View (120x40)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Agent Details                                                                  [u] Update [d] Delete [q] Back [?] Help │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── claude-desktop ──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  Official Claude desktop application by Anthropic                                                                 │
│                                                                                                                    │
├── Current Installation ────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                    │
│  Version:                  v1.2.3                                                                                 │
│  Status:                   Update Available → v1.2.5                                                              │
│  Installation Method:      dmg                                                                                     │
│  Installation Path:        /Applications/Claude.app                                                                │
│  Size:                     287.4 MB                                                                                │
│  Installed:                2025-12-15 10:30:00                                                                     │
│  Last Updated:             2025-12-15 10:30:00 (23 days ago)                                                       │
│  Last Checked:             2026-01-07 14:22:00 (2 minutes ago)                                                     │
│                                                                                                                    │
├── Latest Version (v1.2.5) ─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                    │
│  Released:                 2026-01-05 14:00:00 (2 days ago)                                                        │
│  Download Size:            245.3 MB                                                                                │
│  Installed Size:           ~290 MB (estimated)                                                                     │
│                                                                                                                    │
│  What's New in v1.2.5:                                                                                            │
│                                                                                                                    │
│  Bug Fixes                                                                                                         │
│  • Fixed rendering issues on macOS Sonoma and Sequoia                                                             │
│  • Resolved crash when switching between conversations rapidly                                                     │
│  • Fixed copy/paste formatting in code blocks                                                                      │
│                                                                                                                    │
│  Improvements                                                                                                      │
│  • Improved response streaming performance (30% faster)                                                            │
│  • Enhanced error messages for network issues                                                                      │
│  • Better memory management for long conversations                                                                 │
│                                                                                                                    │
│  Security                                                                                                          │
│  • Updated dependencies to fix CVE-2025-0123                                                                       │
│  • Improved certificate validation                                                                                 │
│                                                                                                                    │
├── Configuration ───────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                    │
│  Auto Update:              ● Enabled                                                                              │
│  Launch at Login:          ○ Disabled                                                                             │
│  Update Channel:           stable                                                                                 │
│  Backup Before Update:     ● Enabled                                                                              │
│  Keep Old Versions:        ○ Disabled                                                                             │
│                                                                                                                    │
├── Links ───────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                    │
│  Homepage:                 https://claude.ai/download                                                              │
│  Repository:               https://github.com/anthropics/claude-desktop                                            │
│  Documentation:            https://docs.anthropic.com/claude/desktop                                               │
│  Release Notes:            https://github.com/anthropics/claude-desktop/releases/tag/v1.2.5                        │
│  Support:                  https://support.anthropic.com                                                           │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 [u] Update to v1.2.5  [d] Delete Agent  [e] Edit Config  [c] Copy Info  [↑↓] Scroll  [q] Back
```

### 3. Catalog Browse View (120x40)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Agent Catalog                                                             [r] Refresh [/] Search [Esc] Back [?] Help │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── Available Agents ────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  Filter: [code editor_________________________________]  [x] Clear   12 agents • Showing 3 matches                 │
│                                                                                                                    │
│  ▸ cursor                         v0.42.0                                                     Installed            │
│    The AI-first Code Editor                                                                                        │
│    Build software faster with AI. Cursor is the IDE of the future.                                                │
│                                                                                                                    │
│    Categories: Code Editor, AI Tools, Productivity                                                                 │
│    Platforms:  macOS, Linux, Windows                                                                               │
│    License:    Proprietary                                                                                         │
│    Updated:    2026-01-06 (1 day ago)                                                                              │
│                                                                                                                    │
│  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  │
│                                                                                                                    │
│    windsurf                       v1.0.1                                                  Not Installed            │
│    AI-powered code editor by Codeium                                                                               │
│    Windsurf combines the power of Codeium's AI with a modern code editor built on VS Code.                        │
│                                                                                                                    │
│    Categories: Code Editor, AI Tools, VS Code Based                                                                │
│    Platforms:  macOS, Linux, Windows                                                                               │
│    License:    Proprietary                                                                                         │
│    Updated:    2026-01-05 (2 days ago)                                                                             │
│                                                                                                                    │
│  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  │
│                                                                                                                    │
│    zed                            v0.165.2                                                    Installed            │
│    High-performance, multiplayer code editor                                                                       │
│    Code at the speed of thought. Zed is a lightning-fast, collaborative code editor written in Rust.              │
│                                                                                                                    │
│    Categories: Code Editor, Collaboration, Rust                                                                    │
│    Platforms:  macOS, Linux                                                                                        │
│    License:    Apache 2.0, GPL 3.0                                                                                 │
│    Updated:    2026-01-06 (1 day ago)                                                                              │
│                                                                                                                    │
│  [↑↓] Navigate  [Enter] Install/Details  [Space] Quick Info  [/] Filter  [c] Clear Filter                        │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Quick Info ──────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  cursor v0.42.0                                                                              Installed: v0.42.0   │
│                                                                                                                    │
│  The AI-first Code Editor - Build software faster with AI                                                         │
│                                                                                                                    │
│  ⭐ Popular  •  Updated daily  •  Active development                                                               │
│                                                                                                                    │
│  Installation methods: app-bundle, homebrew-cask                                                                   │
│  Default size: ~410 MB                                                                                             │
│                                                                                                                    │
│  [Enter] View Full Details  •  Already installed                                                                  │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 [/] Filter by name or category  •  [Enter] Install/Details  •  [r] Refresh catalog  •  Page 1/1 (3 shown)
```

### 4. Installation Progress (120x40)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Installing claude-desktop v1.2.5                                                                    [Ctrl+C] Cancel │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── Installation Progress ───────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  ✓ Pre-flight checks                                                                              [Completed]     │
│    • Verifying system compatibility                                                                ✓              │
│    • Checking available disk space (290 MB required)                                               ✓              │
│    • Verifying write permissions for /Applications                                                 ✓              │
│                                                                                                                    │
│  ✓ Fetching release information                                                                   [Completed]     │
│    • Querying GitHub API for latest release                                                        ✓              │
│    • Parsing release metadata                                                                      ✓              │
│    • Validating download URLs                                                                      ✓              │
│                                                                                                                    │
│  ▸ Downloading installer                                                                          [In Progress]   │
│    • File: Claude-1.2.5-arm64.dmg                                                                                  │
│    • Source: github.com/anthropics/claude-desktop/releases                                                         │
│                                                                                                                    │
│      ████████████████████████████████████████████░░░░░░░░░░░░░░░░░░  67%                                          │
│                                                                                                                    │
│      Downloaded:  164.3 MB / 245.3 MB                                                                             │
│      Speed:       12.5 MB/s (average: 11.8 MB/s)                                                                  │
│      Elapsed:     14 seconds                                                                                       │
│      Remaining:   ~6 seconds                                                                                       │
│                                                                                                                    │
│  ○ Verifying download                                                                             [Pending]       │
│    • Computing SHA256 checksum                                                                                     │
│    • Comparing with published hash                                                                                 │
│                                                                                                                    │
│  ○ Installing application                                                                         [Pending]       │
│    • Mounting disk image                                                                                           │
│    • Copying application bundle                                                                                    │
│    • Setting permissions                                                                                           │
│    • Ejecting disk image                                                                                           │
│                                                                                                                    │
│  ○ Post-installation                                                                              [Pending]       │
│    • Verifying installation                                                                                        │
│    • Cleaning up temporary files                                                                                   │
│    • Updating agent registry                                                                                       │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Activity Log ────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  14:32:15  INFO   Starting installation of claude-desktop v1.2.5                                                  │
│  14:32:15  INFO   System check passed: macOS 14.2.1 (arm64)                                                       │
│  14:32:15  INFO   Available disk space: 145.2 GB                                                                  │
│  14:32:16  INFO   Fetching release info from GitHub API                                                           │
│  14:32:16  INFO   Found release v1.2.5 published on 2026-01-05                                                    │
│  14:32:17  INFO   Starting download: Claude-1.2.5-arm64.dmg (245.3 MB)                                            │
│  14:32:17  INFO   Download URL: https://github.com/anthropics/claude-desktop/releases/download/v1.2.5/...         │
│  14:32:19  INFO   Download progress: 10% (24.5 MB)                                                                │
│  14:32:22  INFO   Download progress: 25% (61.3 MB)                                                                │
│  14:32:25  INFO   Download progress: 50% (122.7 MB)                                                               │
│  14:32:29  INFO   Download progress: 67% (164.3 MB)                                                               │
│                                                                                                                    │
│  [↑↓] Scroll log                                                                                                  │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 Downloading installer... Press [Ctrl+C] to cancel
```

### 5. Multi-Agent Update View (120x40)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Update Agents                                                  [a] Select All  [A] Deselect  [U] Update  [Esc] Back │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── Available Updates (3) ───────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  [●] claude-desktop          v1.2.3  →  v1.2.5                                             Released: 2 days ago   │
│      • Fixed rendering issues on macOS Sonoma and Sequoia                                                         │
│      • Improved response streaming performance (30% faster)                                                        │
│      • Security updates for dependency vulnerabilities                                                             │
│      • Enhanced error messages for network issues                                                                  │
│      Download: 245.3 MB  •  Install time: ~2 minutes                                                              │
│                                                                                                                    │
│  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  │
│                                                                                                                    │
│  [●] cursor                  v0.42.0  →  v0.43.1                                           Released: 1 day ago    │
│      • Added support for Claude Sonnet 4.5                                                                         │
│      • Improved code completion accuracy                                                                           │
│      • Fixed bug with multi-file editing                                                                           │
│      • Performance improvements for large projects                                                                 │
│      Download: 198.7 MB  •  Install time: ~2 minutes                                                              │
│                                                                                                                    │
│  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  │
│                                                                                                                    │
│  [ ] zed                     v0.165.2  →  v0.166.0                                         Released: 3 hours ago  │
│      • New Vim mode improvements                                                                                   │
│      • Added support for more languages                                                                            │
│      • Fixed LSP integration issues                                                                                │
│      • UI polish and bug fixes                                                                                     │
│      Download: 45.2 MB  •  Install time: ~1 minute                                                                │
│                                                                                                                    │
│  [↑↓] Navigate  [Space] Toggle selection  [Enter] View full changelog                                            │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Update Options ──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  [●] Create backup before updating                                                                                │
│      Backs up current version to ~/.local/share/agentmgr/backups                                                   │
│                                                                                                                    │
│  [●] Verify checksums                                                                                             │
│      Validate SHA256 hash of downloaded files                                                                      │
│                                                                                                                    │
│  [ ] Keep old versions                                                                                            │
│      Retain previous versions in backup directory (uses more disk space)                                           │
│                                                                                                                    │
│  [●] Notify on completion                                                                                         │
│      Show system notification when updates finish                                                                  │
│                                                                                                                    │
│  [Tab] Toggle option                                                                                              │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Update Summary ──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  Selected:          2 agents (claude-desktop, cursor)                                                              │
│  Total Download:    444.0 MB                                                                                       │
│  Estimated Time:    ~4 minutes                                                                                     │
│  Required Space:    ~1.2 GB (including backups)                                                                    │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 [Space] Toggle selection  •  [a] Select all  •  [U] Begin update  •  [Enter] View changelog  •  [Esc] Cancel
```

### 6. Settings View (120x40)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Settings                                                                      [s] Save [r] Reset [Esc] Back [?] Help │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── General ─────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  Config Path:         ~/.config/agentmgr/config.yaml                                                               │
│  Data Directory:      ~/.local/share/agentmgr                                                                      │
│  Cache Directory:     ~/.cache/agentmgr                                                                            │
│  Logs Directory:      ~/.local/share/agentmgr/logs                                                                 │
│                                                                                                                    │
│  Theme:               [ Auto ▼]  (Auto, Light, Dark)                                                              │
│  Language:            [ English ▼]  (English, Español, Français, Deutsch, 日本語, 中文)                            │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Updates ─────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  Auto Check for Updates:        [●] Enabled                                                                       │
│      Automatically check for agent updates in the background                                                       │
│                                                                                                                    │
│  Check Interval:                [ 6 hours ▼]                                                                      │
│      How often to check for updates (1h, 3h, 6h, 12h, 24h, Weekly)                                                │
│                                                                                                                    │
│  Auto Install Updates:          [ ] Disabled                                                                      │
│      Automatically install updates when found (not recommended)                                                    │
│                                                                                                                    │
│  Update Channel:                [ Stable ▼]                                                                       │
│      Which release channel to follow (Stable, Beta, Nightly)                                                       │
│                                                                                                                    │
│  Backup Before Update:          [●] Enabled                                                                       │
│      Create backup of current version before updating                                                              │
│                                                                                                                    │
│  Backup Retention:              [ 3 versions ▼]                                                                   │
│      Number of old versions to keep (1, 3, 5, 10, All)                                                            │
│                                                                                                                    │
│  Verify Checksums:              [●] Enabled                                                                       │
│      Verify SHA256 hash of downloads for security                                                                  │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Systray Helper ──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  Launch at Login:               [●] Enabled                                                                       │
│      Start systray helper automatically when you log in                                                            │
│                                                                                                                    │
│  Show Notifications:            [●] Enabled                                                                       │
│      Display system notifications for updates and events                                                           │
│                                                                                                                    │
│  Notification Types:                                                                                              │
│      [●] Updates available                                                                                        │
│      [●] Installation complete                                                                                    │
│      [ ] Update check completed (no updates)                                                                       │
│      [●] Errors and failures                                                                                      │
│                                                                                                                    │
│  Minimize to Tray:              [ ] Disabled                                                                      │
│      Minimize TUI to system tray instead of closing                                                                │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Network ─────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  HTTP Proxy:                    [_______________________________________________________________]                 │
│      HTTP proxy server (e.g., http://proxy.example.com:8080)                                                      │
│                                                                                                                    │
│  HTTPS Proxy:                   [_______________________________________________________________]                 │
│      HTTPS proxy server (leave empty to use HTTP proxy)                                                           │
│                                                                                                                    │
│  Timeout:                       [ 30 seconds ▼]                                                                   │
│      Network request timeout (10s, 30s, 60s, 120s, 300s)                                                          │
│                                                                                                                    │
│  Verify SSL:                    [●] Enabled                                                                       │
│      Verify SSL certificates (disable only for testing)                                                            │
│                                                                                                                    │
│  Max Concurrent Downloads:      [ 3 ▼]                                                                            │
│      Maximum simultaneous downloads (1, 2, 3, 5, 10)                                                              │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 ● Changes pending  •  [s] Save and apply  •  [r] Reset to defaults  •  [Esc] Discard and go back
```

### 7. Error Display (120x40)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Installation Failed                                                                               [r] Retry [q] Back │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── Error Details ───────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                    │
│  ✗ Network Error                                                                                                  │
│                                                                                                                    │
│  Failed to download installer for claude-desktop v1.2.5                                                            │
│                                                                                                                    │
│  The download was interrupted after 164.3 MB due to a connection timeout. This typically happens when:            │
│  • Your internet connection is unstable                                                                            │
│  • The GitHub releases server is experiencing issues                                                               │
│  • A firewall or proxy is blocking the connection                                                                  │
│                                                                                                                    │
├── Technical Details ───────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                    │
│  Error Type:       Network Timeout                                                                                │
│  Error Code:       ETIMEDOUT                                                                                      │
│  URL:              https://github.com/anthropics/claude-desktop/releases/download/v1.2.5/...                       │
│  Downloaded:       164.3 MB / 245.3 MB (67%)                                                                       │
│  Duration:         29.8 seconds (timeout: 30 seconds)                                                              │
│  Timestamp:        2026-01-07 14:32:45                                                                             │
│                                                                                                                    │
├── Troubleshooting Steps ───────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                    │
│  1. Check your internet connection                                                                                │
│     Test connectivity: ping -c 3 github.com                                                                        │
│                                                                                                                    │
│  2. Check if you're behind a proxy or firewall                                                                     │
│     Configure proxy in settings if needed                                                                          │
│     Command: agentmgr config set network.http_proxy http://proxy:8080                                             │
│                                                                                                                    │
│  3. Try increasing the timeout                                                                                     │
│     Command: agentmgr config set network.timeout 60                                                                │
│                                                                                                                    │
│  4. Check GitHub status                                                                                            │
│     Visit: https://www.githubstatus.com                                                                            │
│                                                                                                                    │
│  5. Try downloading manually                                                                                       │
│     Visit: https://github.com/anthropics/claude-desktop/releases/tag/v1.2.5                                        │
│                                                                                                                    │
│  6. Retry the installation                                                                                         │
│     AgentManager will resume from where it left off (164.3 MB already downloaded)                                  │
│                                                                                                                    │
├── Additional Help ─────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                                    │
│  View logs:            agentmgr helper logs                                                                        │
│  Report issue:         https://github.com/kevinelliott/agent-manager/issues                                        │
│  Documentation:        https://github.com/kevinelliott/agent-manager#troubleshooting                               │
│  Community Discord:    https://discord.gg/agentmanager                                                             │
│                                                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

 [r] Retry installation  •  [s] Go to Settings  •  [l] View Logs  •  [q] Back to Dashboard
```

---

## Responsive Layout Examples

### Narrow Terminal (80x24)

```
╭────────────────────────────────────────────────────────────────────────╮
│ AgentManager                                        [?] Help [q] Quit │
╰────────────────────────────────────────────────────────────────────────╯

┌── Agents (4) ──────────────────────────────────────────────────────────┐
│                                                                        │
│  ▸ claude-desktop   v1.2.3 → v1.2.5    Update Available               │
│    cursor           v0.42.0             Up to date                     │
│    zed              v0.165.2            Up to date                     │
│    windsurf         v1.0.1              Up to date                     │
│                                                                        │
│  [↑↓] Nav  [↵] Details  [u] Update  [i] Install                       │
└────────────────────────────────────────────────────────────────────────┘

┌── Actions ─────────────────────────────────────────────────────────────┐
│  [i] Install  [u] Update  [U] All  [c] Catalog  [s] Settings          │
└────────────────────────────────────────────────────────────────────────┘

 1 update • Systray: ● Running • [?] Help
```

### Wide Terminal (160x50)

```
╭──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ AgentManager v1.0.0                                                                                                                   [?] Help [q] Quit       │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

┌── Status ──────────────────────────┐ ┌── Quick Actions ──────────────────┐ ┌── System Info ───────────────────────────────────────────────────┐
│                                    │ │                                   │ │                                                                  │
│  Total Agents:        4            │ │  [i] Install Agent                │ │  Platform:        macOS 14.2.1 (arm64)                           │
│  Up to Date:          3            │ │  [u] Update Selected              │ │  Config:          ~/.config/agentmgr/config.yaml                 │
│  Updates Available:   1            │ │  [U] Update All                   │ │  Data Dir:        ~/.local/share/agentmgr                        │
│  Systray Helper:      ● Running    │ │  [c] Browse Catalog               │ │  Cache Size:      1.2 GB                                         │
│  Last Checked:        2 min ago    │ │  [s] Settings                     │ │  Backups:         3 versions (2.4 GB)                            │
│  Next Check:          in 3h 58m    │ │  [r] Refresh                      │ │  Log Level:       info                                           │
│                                    │ │                                   │ │                                                                  │
└────────────────────────────────────┘ └───────────────────────────────────┘ └──────────────────────────────────────────────────────────────────┘

┌── Installed Agents (Detailed View) ────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                                                            │
│  ▸ claude-desktop        v1.2.3 → v1.2.5                                                                                      Update Available            │
│    Official Claude desktop application by Anthropic                                                                                                       │
│    Installed: 2025-12-15 10:30 (23 days ago)  •  Method: dmg  •  Size: 287.4 MB  •  Path: /Applications/Claude.app                                       │
│    Latest: v1.2.5 (released 2 days ago, 245.3 MB)  •  Auto-update: Enabled  •  Channel: stable                                                           │
│                                                                                                                                                            │
│    cursor                v0.42.0                                                                                              Up to date                  │
│    The AI-first Code Editor - Build software faster with AI                                                                                               │
│    Installed: 2025-11-20 09:15 (48 days ago)  •  Method: app-bundle  •  Size: 412.8 MB  •  Path: /Applications/Cursor.app                                │
│    Latest: v0.42.0 (you have the latest)  •  Auto-update: Enabled  •  Channel: stable                                                                    │
│                                                                                                                                                            │
│  [... more agents ...]                                                                                                                                     │
│                                                                                                                                                            │
│  [↑↓] Navigate  [Enter] Full Details  [u] Update Selected  [d] Delete  [e] Edit Config  [/] Filter  [Space] Multi-select                                 │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

┌── Recent Activity (Last 7 Days) ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                                                                                            │
│  2 minutes ago          Checked for updates (1 found: claude-desktop)                                                                                     │
│  1 day ago              Updated claude-desktop: 1.2.2 → 1.2.3 (successful, 245.3 MB downloaded in 21s)                                                   │
│  3 days ago             Installed windsurf v1.0.1 (523.1 MB, installation time: 1m 32s)                                                                   │
│  1 week ago             Updated cursor: 0.41.0 → 0.42.0 (successful, 198.7 MB downloaded in 18s)                                                         │
│  2 weeks ago            Updated zed: 0.164.0 → 0.165.2 (successful, 45.2 MB downloaded in 4s)                                                            │
│                                                                                                                                                            │
│  [↑↓] Scroll  [l] View full logs                                                                                                                          │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Animation Examples

### Spinner Variants

```
Standard spinner (for indefinite tasks):
⠋ Loading...
⠙ Loading...
⠹ Loading...
⠸ Loading...
⠼ Loading...
⠴ Loading...
⠦ Loading...
⠧ Loading...

Dots (for waiting):
Loading.
Loading..
Loading...
Loading

Progress spinner (shows activity):
◐ Processing files...
◓ Processing files...
◑ Processing files...
◒ Processing files...
```

### Progress Bar Variants

```
Simple bar:
[████████████████████░░░░░░░░] 67%

Detailed bar with stats:
████████████████████░░░░░░░░░░░  67%  (164.3 MB / 245.3 MB)

Multi-line progress:
Downloading claude-desktop v1.2.5
████████████████████░░░░░░░░░░░  67%
12.5 MB/s  •  6s remaining

Segmented progress (for multiple files):
File 1: ████████████████████████  100%
File 2: ████████░░░░░░░░░░░░░░░░   33%
File 3: ░░░░░░░░░░░░░░░░░░░░░░░░    0%
Overall: ████████░░░░░░░░░░░░░░░░   44%
```

---

## Icons and Symbols Reference

### Status Icons

```
✓  Successful/Complete
✗  Failed/Error
●  Active/Running
○  Inactive/Stopped
▸  Selected/Current
►  Playing/In Progress
⏸  Paused
⏹  Stopped
⚠  Warning
ℹ  Information
```

### File Type Icons (optional, for rich mode)

```
📦  Package/Bundle
📄  Document
📁  Folder
🔧  Configuration
📊  Data/Metrics
🔐  Security/Certificate
🌐  Network/Web
💾  Database
```

### Action Icons

```
⬇  Download
⬆  Upload
🔄  Refresh/Sync
🗑  Delete
✏  Edit
👁  View
⚙  Settings
❓  Help
```

---

This completes the comprehensive visual reference guide for AgentManager!
