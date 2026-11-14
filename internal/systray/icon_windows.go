//go:build windows

package systray

import _ "embed"

// The Windows tray only accepts .ico data.
//
//go:embed icons/tray.ico
var trayIcon []byte
