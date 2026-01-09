# CLI Output Package

This package provides colorized output and loading animations for the agentmgr CLI.

## Features

### Colors

The output package provides a consistent color scheme across CLI commands:

- **Green**: Success messages and installed agents
- **Purple**: Headers and emphasis
- **Orange**: Warnings and updates available
- **Red**: Errors
- **Cyan**: Info messages
- **Gray**: Muted text (methods, secondary info)

### Color Control

Colors can be disabled in three ways:

1. **--no-color flag**: `agentmgr agent list --no-color`
2. **NO_COLOR environment variable**: `export NO_COLOR=1`
3. **Configuration file**: Set `ui.use_colors: false` in config

### Spinner Animations

Loading spinners provide visual feedback during long-running operations:

```go
spinner := output.NewSpinner(
    output.WithMessage("Loading catalog..."),
    output.WithNoColor(!cfg.UI.UseColors),
)
spinner.Start()

// Do work...

spinner.Success("Operation completed!")
// or
spinner.Error("Operation failed")
// or
spinner.Stop()
```

The spinner automatically:
- Displays an animated spinner during operation
- Clears the spinner line when done
- Shows appropriate icons for success/error/warning/info

### Printer

The Printer provides a unified interface for colored output:

```go
printer := output.NewPrinter(cfg, noColor)

printer.Success("Installation complete!")
printer.Info("Checking for updates...")
printer.Warning("Agent version is outdated")
printer.Error("Failed to connect to registry")
```

### Styles

Access style helpers via `printer.Styles()`:

```go
styles := printer.Styles()

// Format agent names, versions, methods, headers
agentName := styles.FormatAgentName("aider")
version := styles.FormatVersion("1.2.3", hasUpdate)
method := styles.FormatMethod("npm")
header := styles.FormatHeader("AGENT")

// Status icons
icon := styles.UpdateIcon()      // ⬆
icon = styles.InstalledIcon()    // ●
icon = styles.NotInstalledIcon() // ○

// Badges
badge := styles.FormatBadge("new", "success")
```

## Usage in Commands

### Agent List

Shows a colored table with:
- Purple headers
- Bold agent names
- Orange versions for outdated agents
- Gray installation methods
- Status icons (● for installed, ⬆ for updates available)
- Spinner while detecting agents and checking versions

### Agent Install

Shows spinner during:
- Catalog loading
- Package installation

Progress messages:
- "Loading catalog..."
- "Installing {agent} via {method}..."
- "✓ Installed {agent} {version} successfully"

### Agent Update

Shows spinner during:
- Catalog loading
- Agent detection
- Version checking
- Update operations

Colored output for:
- List of available updates
- Update progress for each agent
- Success/error messages

## Spinner Frames

Available spinner frame sets:

```go
output.SpinnerFrames.Dots     // ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏
output.SpinnerFrames.Line     // -\|/
output.SpinnerFrames.Arrow    // ←↖↑↗→↘↓↙
output.SpinnerFrames.Pulse    // ◐◓◑◒
output.SpinnerFrames.Binary   // 010010 001001 100100
output.SpinnerFrames.Circle   // ◜◠◝◞◡◟
```

Default is `Dots` which works well across all terminals.

## Color Palette

The color palette is based on the Dracula theme:

- Purple: `#BD93F9`
- Green: `#50FA7B`
- Orange: `#FFB86C`
- Red: `#FF5555`
- Cyan: `#8BE9FD`
- Yellow: `#F1FA8C`
- Gray: `#6272A4`
- White: `#F8F8F2`

These colors provide good contrast and accessibility in both light and dark terminals.

## Terminal Compatibility

The package uses Lip Gloss and termenv which automatically detect terminal capabilities:

- **True Color (24-bit)**: Full color support
- **ANSI 256 (8-bit)**: Approximate colors
- **ANSI (4-bit)**: Basic colors
- **ASCII**: No colors (when NO_COLOR is set)

The output gracefully degrades based on terminal capabilities.
