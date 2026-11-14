//go:build !darwin && !windows

package systray

import _ "embed"

// A full-colour icon: Linux panels can be light or dark and do not tint.
//
//go:embed icons/tray.png
var trayIcon []byte
