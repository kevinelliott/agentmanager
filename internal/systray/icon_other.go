//go:build !darwin && !windows

package systray

import _ "embed"

// A full-color icon: Linux panels can be light or dark and do not tint.
//
//go:embed icons/tray.png
var trayIcon []byte
