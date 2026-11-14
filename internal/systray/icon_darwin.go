//go:build darwin

package systray

import _ "embed"

// A black-with-alpha template image; macOS tints it for light and dark menu
// bars. It is 64px so it stays sharp on Retina displays at 16pt.
//
//go:embed icons/tray_template.png
var trayIcon []byte
