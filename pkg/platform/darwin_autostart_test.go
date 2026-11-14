//go:build darwin

package platform

import (
	"strings"
	"testing"
)

func TestLaunchAgentPlist(t *testing.T) {
	d := &darwinPlatform{}
	plist := d.generateLaunchAgentPlist()

	// KeepAlive would make launchd restart the app as soon as it is quit.
	if strings.Contains(plist, "KeepAlive") {
		t.Error("launch agent must not set KeepAlive")
	}
	if !strings.Contains(plist, "<key>RunAtLoad</key>") {
		t.Error("launch agent must run at load")
	}
	if !strings.Contains(plist, "agentmgr-helper</string>") {
		t.Errorf("launch agent does not start the helper:\n%s", plist)
	}
}

func TestXMLEscape(t *testing.T) {
	if got := xmlEscape(`/Users/a&b/<x>`); got != "/Users/a&amp;b/&lt;x&gt;" {
		t.Errorf("xmlEscape = %q", got)
	}
}
