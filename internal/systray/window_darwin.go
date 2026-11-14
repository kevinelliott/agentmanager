//go:build darwin

package systray

import (
	"encoding/json"
	"sync"

	"github.com/progrium/darwinkit/dispatch"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
	"github.com/progrium/darwinkit/macos/webkit"
	"github.com/progrium/darwinkit/objc"
)

// canUninstallCLI reports whether this platform can remove the CLI it installed.
const canUninstallCLI = true

// The window is created once and kept for the life of the helper. Closing it
// only hides it, so reopening is instant and keeps whatever the person was
// looking at.
var (
	mainWinMu      sync.Mutex
	mainWin        appkit.Window
	mainWebView    webkit.WebView
	mainWinCreated bool

	// mainWinKeep holds Go objects that AppKit refers to but does not retain,
	// such as the window delegate, so they are not garbage collected.
	mainWinKeep []any
)

// showMainWindow opens the AgentManager window, or brings it forward if it is
// already open. route is a UI route such as "/settings"; empty keeps the
// current view.
func (a *App) showMainWindow(route string) {
	dispatch.MainQueue().DispatchAsync(func() {
		mainWinMu.Lock()
		defer mainWinMu.Unlock()

		nsApp := appkit.Application_SharedApplication()

		if !mainWinCreated {
			a.createMainWindow(route)
		} else if route != "" {
			// Routes are passed as JSON so they are always a valid JS string.
			if quoted, err := json.Marshal(route); err == nil {
				mainWebView.EvaluateJavaScriptCompletionHandler(
					"location.hash = "+string(quoted),
					func(objc.Object, foundation.Error) {},
				)
			}
		}

		// A menu bar app has no Dock icon; show one while the window is open
		// so it can be found again with Cmd-Tab.
		nsApp.SetActivationPolicy(appkit.ApplicationActivationPolicyRegular)
		nsApp.ActivateIgnoringOtherApps(true)
		mainWin.MakeKeyAndOrderFront(nil)
	})
}

// createMainWindow builds the window and loads the UI. Main thread only,
// with mainWinMu held.
func (a *App) createMainWindow(route string) {
	frame := foundation.Rect{
		Origin: foundation.Point{X: 0, Y: 0},
		Size:   foundation.Size{Width: 1120, Height: 740},
	}

	win := appkit.NewWindowWithContentRectStyleMaskBackingDefer(
		frame,
		appkit.WindowStyleMaskTitled|
			appkit.WindowStyleMaskClosable|
			appkit.WindowStyleMaskMiniaturizable|
			appkit.WindowStyleMaskResizable,
		appkit.BackingStoreBuffered,
		false,
	)
	win.SetTitle("AgentManager")
	win.SetReleasedWhenClosed(false)
	win.SetMinSize(foundation.Size{Width: 760, Height: 520})

	webView := webkit.NewWebViewWithFrame(frame)
	webView.SetAutoresizingMask(appkit.ViewWidthSizable | appkit.ViewHeightSizable)
	webView.SetAllowsBackForwardNavigationGestures(false)
	win.SetContentView(webView)

	delegate := &appkit.WindowDelegate{}
	delegate.SetWindowWillClose(func(foundation.Notification) {
		// Back to a menu bar only app once the window is gone.
		appkit.Application_SharedApplication().SetActivationPolicy(appkit.ApplicationActivationPolicyAccessory)
	})
	win.SetDelegate(delegate)

	win.Center()
	win.SetFrameAutosaveName("AgentManagerMainWindow")

	installMainMenu()

	webView.LoadRequest(foundation.NewURLRequestWithURL(foundation.URL_URLWithString(a.ui.entryURL(route))))

	mainWin = win
	mainWebView = webView
	mainWinKeep = append(mainWinKeep, delegate)
	mainWinCreated = true
}

// installMainMenu gives the app the standard Edit and Window shortcuts.
// Without a main menu, Cmd-C, Cmd-V, Cmd-A and Cmd-W do nothing in the window.
func installMainMenu() {
	item := func(title, selector, key string) appkit.MenuItem {
		return appkit.NewMenuItemWithTitleActionKeyEquivalent(title, objc.Sel(selector), key)
	}

	mainMenu := appkit.NewMenuWithTitle("Main")

	appItem := appkit.NewMenuItem()
	appMenu := appkit.NewMenuWithTitle("AgentManager")
	appMenu.AddItem(item("Hide AgentManager", "hide:", "h"))
	appMenu.AddItem(item("Close Window", "performClose:", "w"))
	appItem.SetSubmenu(appMenu)
	mainMenu.AddItem(appItem)

	editItem := appkit.NewMenuItem()
	editMenu := appkit.NewMenuWithTitle("Edit")
	editMenu.AddItem(item("Undo", "undo:", "z"))
	editMenu.AddItem(item("Redo", "redo:", "Z"))
	editMenu.AddItem(appkit.MenuItem_SeparatorItem())
	editMenu.AddItem(item("Cut", "cut:", "x"))
	editMenu.AddItem(item("Copy", "copy:", "c"))
	editMenu.AddItem(item("Paste", "paste:", "v"))
	editMenu.AddItem(item("Select All", "selectAll:", "a"))
	editItem.SetSubmenu(editMenu)
	mainMenu.AddItem(editItem)

	appkit.Application_SharedApplication().SetMainMenu(mainMenu)
}

// closeMainWindow closes the window when the helper is quitting.
func closeMainWindow() {
	mainWinMu.Lock()
	defer mainWinMu.Unlock()
	if mainWinCreated {
		mainWin.Close()
	}
}
