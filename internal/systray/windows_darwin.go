//go:build darwin

package systray

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/progrium/darwinkit/dispatch"
	"github.com/progrium/darwinkit/helper/action"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
	"github.com/progrium/darwinkit/objc"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
)

// Apple HIG-compliant design constants
const (
	windowPadding   = 20.0 // Standard macOS window margin
	sectionGap      = 24.0 // Gap between major sections
	itemGap         = 8.0  // Gap between items within a section
	boxRadius       = 8.0  // Rounded corner radius for content boxes
	boxInnerPadding = 16.0 // Padding inside content boxes
	rowHeight       = 22.0 // Standard row height for info rows

	// Action type constants
	actionInstall   = "install"
	actionUninstall = "uninstall"
	actionUpdate    = "update"
)

var (
	windowsMu          sync.Mutex
	activeWindows      []appkit.Window
	settingsWindow     appkit.Window
	settingsWindowOpen bool
)

// showNativeSettingsWindow displays a polished native macOS settings window.
func (a *App) showNativeSettingsWindow() {
	app := a

	dispatch.MainQueue().DispatchAsync(func() {
		windowsMu.Lock()
		if settingsWindowOpen {
			// Bring existing window to foreground
			nsApp := appkit.Application_SharedApplication()
			nsApp.SetActivationPolicy(appkit.ApplicationActivationPolicyRegular)
			nsApp.ActivateIgnoringOtherApps(true)
			settingsWindow.MakeKeyAndOrderFront(nil)
			windowsMu.Unlock()
			return
		}
		windowsMu.Unlock()

		windowWidth := 440.0
		windowHeight := 460.0

		win := appkit.NewWindowWithContentRectStyleMaskBackingDefer(
			foundation.Rect{
				Origin: foundation.Point{X: 200, Y: 200},
				Size:   foundation.Size{Width: windowWidth, Height: windowHeight},
			},
			appkit.WindowStyleMaskTitled|
				appkit.WindowStyleMaskClosable|
				appkit.WindowStyleMaskMiniaturizable,
			appkit.BackingStoreBuffered,
			false,
		)
		win.SetTitle("Settings")
		win.SetReleasedWhenClosed(false)

		contentView := appkit.NewView()
		contentView.SetFrameSize(foundation.Size{Width: windowWidth, Height: windowHeight})

		contentWidth := windowWidth - (windowPadding * 2)
		y := windowHeight - windowPadding

		// ═══════════════════════════════════════════════════════════════
		// HEADER - App icon and title
		// ═══════════════════════════════════════════════════════════════
		iconSize := 64.0

		// App icon (rounded square like native macOS app icons)
		iconView := appkit.NewBox()
		iconView.SetBoxType(appkit.BoxCustom)
		iconView.SetCornerRadius(14)
		iconView.SetFillColor(appkit.Color_ControlAccentColor())
		iconView.SetBorderWidth(0)
		iconView.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y - iconSize},
			Size:   foundation.Size{Width: iconSize, Height: iconSize},
		})
		contentView.AddSubview(iconView)

		// Icon letter centered
		iconLabel := appkit.NewTextField()
		iconLabel.SetStringValue("A")
		iconLabel.SetEditable(false)
		iconLabel.SetBordered(false)
		iconLabel.SetDrawsBackground(false)
		iconLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(28, appkit.FontWeightMedium))
		iconLabel.SetTextColor(appkit.Color_WhiteColor())
		iconLabel.SetAlignment(appkit.TextAlignmentCenter)
		iconLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y - iconSize + 18},
			Size:   foundation.Size{Width: iconSize, Height: 32},
		})
		contentView.AddSubview(iconLabel)

		// Title
		titleX := windowPadding + iconSize + 16
		titleLabel := appkit.NewTextField()
		titleLabel.SetStringValue("AgentManager")
		titleLabel.SetEditable(false)
		titleLabel.SetBordered(false)
		titleLabel.SetDrawsBackground(false)
		titleLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(20, appkit.FontWeightSemibold))
		titleLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: titleX, Y: y - 28},
			Size:   foundation.Size{Width: contentWidth - iconSize - 16, Height: 26},
		})
		contentView.AddSubview(titleLabel)

		// Subtitle
		subtitleLabel := appkit.NewTextField()
		subtitleLabel.SetStringValue("Manage your AI development agents")
		subtitleLabel.SetEditable(false)
		subtitleLabel.SetBordered(false)
		subtitleLabel.SetDrawsBackground(false)
		subtitleLabel.SetFont(appkit.Font_SystemFontOfSize(13))
		subtitleLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		subtitleLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: titleX, Y: y - 50},
			Size:   foundation.Size{Width: contentWidth - iconSize - 16, Height: 18},
		})
		contentView.AddSubview(subtitleLabel)

		// Version
		versionStr := app.version
		if versionStr == "" {
			versionStr = "dev"
		}
		versionLabel := appkit.NewTextField()
		versionLabel.SetStringValue("Version " + versionStr)
		versionLabel.SetEditable(false)
		versionLabel.SetBordered(false)
		versionLabel.SetDrawsBackground(false)
		versionLabel.SetFont(appkit.Font_SystemFontOfSize(11))
		versionLabel.SetTextColor(appkit.Color_TertiaryLabelColor())
		versionLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: titleX, Y: y - 68},
			Size:   foundation.Size{Width: contentWidth - iconSize - 16, Height: 14},
		})
		contentView.AddSubview(versionLabel)

		y -= iconSize + sectionGap + 16 // Extra padding before first section

		// ═══════════════════════════════════════════════════════════════
		// CLI SECTION
		// ═══════════════════════════════════════════════════════════════

		// Section label
		cliSectionLabel := appkit.NewTextField()
		cliSectionLabel.SetStringValue("COMMAND LINE TOOL")
		cliSectionLabel.SetEditable(false)
		cliSectionLabel.SetBordered(false)
		cliSectionLabel.SetDrawsBackground(false)
		cliSectionLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(11, appkit.FontWeightMedium))
		cliSectionLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		cliSectionLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y},
			Size:   foundation.Size{Width: contentWidth, Height: 14},
		})
		contentView.AddSubview(cliSectionLabel)
		y -= 22

		// Determine CLI status
		isInstalled := false
		currentPath := app.config.Helper.CLIPath
		if currentPath == "" {
			if path, err := findAgentMgrBinary(); err == nil {
				currentPath = path
				isInstalled = true
			} else {
				currentPath = "Not installed"
			}
		} else {
			isInstalled = true
		}

		// CLI content box
		cliBoxHeight := 98.0 // Extra bottom padding
		cliBox := appkit.NewBox()
		cliBox.SetBoxType(appkit.BoxCustom)
		cliBox.SetCornerRadius(boxRadius)
		cliBox.SetFillColor(appkit.Color_QuaternaryLabelColor().ColorWithAlphaComponent(0.1))
		cliBox.SetBorderColor(appkit.Color_SeparatorColor())
		cliBox.SetBorderWidth(0.5)
		cliBox.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y - cliBoxHeight},
			Size:   foundation.Size{Width: contentWidth, Height: cliBoxHeight},
		})
		contentView.AddSubview(cliBox)

		boxTop := y - boxInnerPadding

		// Status row: dot + "Installed" text
		statusDot := appkit.NewBox()
		statusDot.SetBoxType(appkit.BoxCustom)
		statusDot.SetCornerRadius(4)
		if isInstalled {
			statusDot.SetFillColor(appkit.Color_SystemGreenColor())
		} else {
			statusDot.SetFillColor(appkit.Color_SystemOrangeColor())
		}
		statusDot.SetBorderWidth(0)
		statusDot.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: boxTop - 12},
			Size:   foundation.Size{Width: 8, Height: 8},
		})
		contentView.AddSubview(statusDot)

		statusText := "Installed"
		if !isInstalled {
			statusText = "Not Installed"
		}
		statusLabel := appkit.NewTextField()
		statusLabel.SetStringValue(statusText)
		statusLabel.SetEditable(false)
		statusLabel.SetBordered(false)
		statusLabel.SetDrawsBackground(false)
		statusLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(13, appkit.FontWeightMedium))
		statusLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + boxInnerPadding + 16, Y: boxTop - 16},
			Size:   foundation.Size{Width: 120, Height: 18},
		})
		contentView.AddSubview(statusLabel)

		// Path row
		pathValueLabel := appkit.NewTextField()
		pathValueLabel.SetStringValue(currentPath)
		pathValueLabel.SetEditable(false)
		pathValueLabel.SetBordered(false)
		pathValueLabel.SetDrawsBackground(false)
		pathValueLabel.SetFont(appkit.Font_MonospacedSystemFontOfSizeWeight(11, appkit.FontWeightRegular))
		pathValueLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		pathValueLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: boxTop - 38},
			Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2, Height: 16},
		})
		contentView.AddSubview(pathValueLabel)

		// Buttons row with extra top margin
		buttonY := boxTop - 72

		// Install/Reinstall button
		installBtn := appkit.NewButton()
		if isInstalled {
			installBtn.SetTitle("Reinstall")
		} else {
			installBtn.SetTitle("Install")
		}
		installBtn.SetBezelStyle(appkit.BezelStyleRounded)
		installBtn.SetControlSize(appkit.ControlSizeSmall)
		installBtn.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: buttonY},
			Size:   foundation.Size{Width: 80, Height: 24},
		})

		// Uninstall button (always created, hidden if not installed)
		uninstallBtnX := windowPadding + boxInnerPadding + 88
		uninstallBtn := appkit.NewButton()
		uninstallBtn.SetTitle("Uninstall")
		uninstallBtn.SetBezelStyle(appkit.BezelStyleRounded)
		uninstallBtn.SetControlSize(appkit.ControlSizeSmall)
		uninstallBtn.SetContentTintColor(appkit.Color_SystemRedColor())
		uninstallBtn.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: uninstallBtnX, Y: buttonY},
			Size:   foundation.Size{Width: 80, Height: 24},
		})
		if !isInstalled {
			uninstallBtn.SetHidden(true)
		}
		contentView.AddSubview(uninstallBtn)

		// Feedback label (position after uninstall button)
		feedbackX := uninstallBtnX + 88
		feedbackLabel := appkit.NewTextField()
		feedbackLabel.SetStringValue("")
		feedbackLabel.SetEditable(false)
		feedbackLabel.SetBordered(false)
		feedbackLabel.SetDrawsBackground(false)
		feedbackLabel.SetFont(appkit.Font_SystemFontOfSize(12))
		feedbackLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: feedbackX, Y: buttonY + 2},
			Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2 - (feedbackX - windowPadding - boxInnerPadding), Height: 18},
		})
		contentView.AddSubview(feedbackLabel)

		action.Set(installBtn, func(_ objc.Object) {
			feedbackLabel.SetStringValue("Installing...")
			feedbackLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
			go func() {
				success := app.installOrUpdateCLI()
				dispatch.MainQueue().DispatchAsync(func() {
					if success {
						if path, err := findAgentMgrBinary(); err == nil {
							pathValueLabel.SetStringValue(path)
						}
						feedbackLabel.SetStringValue("Installed successfully")
						feedbackLabel.SetTextColor(appkit.Color_SystemGreenColor())
						statusDot.SetFillColor(appkit.Color_SystemGreenColor())
						statusLabel.SetStringValue("Installed")
						installBtn.SetTitle("Reinstall")
						uninstallBtn.SetHidden(false) // Show uninstall button
					} else {
						feedbackLabel.SetStringValue("Installation canceled")
						feedbackLabel.SetTextColor(appkit.Color_SystemOrangeColor())
					}
				})
			}()
		})
		contentView.AddSubview(installBtn)

		// Uninstall action
		action.Set(uninstallBtn, func(_ objc.Object) {
			feedbackLabel.SetStringValue("Uninstalling...")
			feedbackLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
			go func() {
				success := app.uninstallCLI()
				dispatch.MainQueue().DispatchAsync(func() {
					if success {
						pathValueLabel.SetStringValue("Not installed")
						feedbackLabel.SetStringValue("Uninstalled")
						feedbackLabel.SetTextColor(appkit.Color_SystemOrangeColor())
						statusDot.SetFillColor(appkit.Color_SystemOrangeColor())
						statusLabel.SetStringValue("Not Installed")
						installBtn.SetTitle("Install")
						uninstallBtn.SetHidden(true)
					} else {
						feedbackLabel.SetStringValue("Uninstall canceled")
						feedbackLabel.SetTextColor(appkit.Color_SystemOrangeColor())
					}
				})
			}()
		})

		y -= cliBoxHeight + sectionGap + 8 // Extra padding before section

		// ═══════════════════════════════════════════════════════════════
		// PREFERENCES SECTION
		// ═══════════════════════════════════════════════════════════════

		prefSectionLabel := appkit.NewTextField()
		prefSectionLabel.SetStringValue("PREFERENCES")
		prefSectionLabel.SetEditable(false)
		prefSectionLabel.SetBordered(false)
		prefSectionLabel.SetDrawsBackground(false)
		prefSectionLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(11, appkit.FontWeightMedium))
		prefSectionLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		prefSectionLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y},
			Size:   foundation.Size{Width: contentWidth, Height: 14},
		})
		contentView.AddSubview(prefSectionLabel)
		y -= 22

		// Preferences box
		prefBoxHeight := 86.0 // Extra bottom padding
		prefBox := appkit.NewBox()
		prefBox.SetBoxType(appkit.BoxCustom)
		prefBox.SetCornerRadius(boxRadius)
		prefBox.SetFillColor(appkit.Color_QuaternaryLabelColor().ColorWithAlphaComponent(0.1))
		prefBox.SetBorderColor(appkit.Color_SeparatorColor())
		prefBox.SetBorderWidth(0.5)
		prefBox.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y - prefBoxHeight},
			Size:   foundation.Size{Width: contentWidth, Height: prefBoxHeight},
		})
		contentView.AddSubview(prefBox)

		prefBoxTop := y - boxInnerPadding

		// Notification checkbox
		notifyCheck := appkit.NewButton()
		notifyCheck.SetButtonType(appkit.ButtonTypeSwitch)
		notifyCheck.SetTitle("Update Notifications")
		notifyCheck.SetFont(appkit.Font_SystemFontOfSize(13))
		notifyCheck.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: prefBoxTop - 18},
			Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2, Height: 18},
		})
		if app.config.Updates.Notify {
			notifyCheck.SetState(appkit.ControlStateValueOn)
		} else {
			notifyCheck.SetState(appkit.ControlStateValueOff)
		}
		action.Set(notifyCheck, func(sender objc.Object) {
			btn := appkit.ButtonFrom(sender.Ptr())
			app.config.Updates.Notify = btn.State() == appkit.ControlStateValueOn
			if app.configLoader != nil {
				_ = app.configLoader.SetAndSave("updates.notify", app.config.Updates.Notify)
			}
		})
		contentView.AddSubview(notifyCheck)

		// Help text under checkbox - properly spaced below
		helpText := appkit.NewTextField()
		helpText.SetStringValue("Receive notifications when updates are available for your installed agents.")
		helpText.SetEditable(false)
		helpText.SetBordered(false)
		helpText.SetDrawsBackground(false)
		helpText.SetFont(appkit.Font_SystemFontOfSize(11))
		helpText.SetTextColor(appkit.Color_TertiaryLabelColor())
		helpText.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + boxInnerPadding + 22, Y: prefBoxTop - 52},
			Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2 - 22, Height: 28},
		})
		contentView.AddSubview(helpText)

		// ═══════════════════════════════════════════════════════════════
		// FOOTER
		// ═══════════════════════════════════════════════════════════════

		footerY := windowPadding + 8 // Extra padding above button
		closeBtn := appkit.NewButton()
		closeBtn.SetTitle("Done")
		closeBtn.SetBezelStyle(appkit.BezelStyleRounded)
		closeBtn.SetKeyEquivalent("\r")
		closeBtn.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowWidth - windowPadding - 80, Y: footerY},
			Size:   foundation.Size{Width: 80, Height: 28},
		})
		action.Set(closeBtn, func(_ objc.Object) {
			windowsMu.Lock()
			settingsWindowOpen = false
			windowsMu.Unlock()
			win.Close()
		})
		contentView.AddSubview(closeBtn)

		win.SetContentView(contentView)
		win.Center()

		// Bring to front
		nsApp := appkit.Application_SharedApplication()
		nsApp.SetActivationPolicy(appkit.ApplicationActivationPolicyRegular)
		nsApp.ActivateIgnoringOtherApps(true)
		win.MakeKeyAndOrderFront(nil)

		windowsMu.Lock()
		settingsWindow = win
		settingsWindowOpen = true
		activeWindows = append(activeWindows, win)
		windowsMu.Unlock()
	})
}

// showNativeAgentDetailsWindow displays a polished agent details window.
func (a *App) showNativeAgentDetailsWindow(inst agent.Installation) {
	app := a
	installation := inst

	dispatch.MainQueue().DispatchAsync(func() {
		hasUpdate := installation.HasUpdate()

		windowWidth := 480.0
		windowHeight := 340.0
		if hasUpdate {
			windowHeight = 420.0
		}

		win := appkit.NewWindowWithContentRectStyleMaskBackingDefer(
			foundation.Rect{
				Origin: foundation.Point{X: 250, Y: 250},
				Size:   foundation.Size{Width: windowWidth, Height: windowHeight},
			},
			appkit.WindowStyleMaskTitled|
				appkit.WindowStyleMaskClosable|
				appkit.WindowStyleMaskMiniaturizable,
			appkit.BackingStoreBuffered,
			false,
		)
		win.SetTitle(installation.AgentName)
		win.SetReleasedWhenClosed(false)

		contentView := appkit.NewView()
		contentView.SetFrameSize(foundation.Size{Width: windowWidth, Height: windowHeight})

		contentWidth := windowWidth - (windowPadding * 2)
		y := windowHeight - windowPadding

		// ═══════════════════════════════════════════════════════════════
		// HEADER - Agent icon and info
		// ═══════════════════════════════════════════════════════════════
		iconSize := 64.0

		// Agent icon (rounded square)
		iconView := appkit.NewBox()
		iconView.SetBoxType(appkit.BoxCustom)
		iconView.SetCornerRadius(14)
		iconView.SetFillColor(appkit.Color_ControlAccentColor())
		iconView.SetBorderWidth(0)
		iconView.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y - iconSize},
			Size:   foundation.Size{Width: iconSize, Height: iconSize},
		})
		contentView.AddSubview(iconView)

		// Icon letter
		firstLetter := "A"
		if len(installation.AgentName) > 0 {
			firstLetter = string(installation.AgentName[0])
		}
		iconLabel := appkit.NewTextField()
		iconLabel.SetStringValue(firstLetter)
		iconLabel.SetEditable(false)
		iconLabel.SetBordered(false)
		iconLabel.SetDrawsBackground(false)
		iconLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(28, appkit.FontWeightMedium))
		iconLabel.SetTextColor(appkit.Color_WhiteColor())
		iconLabel.SetAlignment(appkit.TextAlignmentCenter)
		iconLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y - iconSize + 18},
			Size:   foundation.Size{Width: iconSize, Height: 32},
		})
		contentView.AddSubview(iconLabel)

		// Agent name
		infoX := windowPadding + iconSize + 16
		nameLabel := appkit.NewTextField()
		nameLabel.SetStringValue(installation.AgentName)
		nameLabel.SetEditable(false)
		nameLabel.SetBordered(false)
		nameLabel.SetDrawsBackground(false)
		nameLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(18, appkit.FontWeightSemibold))
		nameLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: infoX, Y: y - 24},
			Size:   foundation.Size{Width: contentWidth - iconSize - 16, Height: 24},
		})
		contentView.AddSubview(nameLabel)

		// Version
		version := installation.InstalledVersion.String()
		if version == "" {
			version = "Unknown"
		}
		versionLabel := appkit.NewTextField()
		versionLabel.SetStringValue("Version " + version)
		versionLabel.SetEditable(false)
		versionLabel.SetBordered(false)
		versionLabel.SetDrawsBackground(false)
		versionLabel.SetFont(appkit.Font_SystemFontOfSize(13))
		versionLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		versionLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: infoX, Y: y - 44},
			Size:   foundation.Size{Width: contentWidth - iconSize - 16, Height: 18},
		})
		contentView.AddSubview(versionLabel)

		// Method badge
		methodText := string(installation.Method)
		badgeWidth := float64(len(methodText)*7 + 12)
		methodBadge := appkit.NewBox()
		methodBadge.SetBoxType(appkit.BoxCustom)
		methodBadge.SetCornerRadius(4)
		methodBadge.SetFillColor(appkit.Color_QuaternaryLabelColor())
		methodBadge.SetBorderWidth(0)
		methodBadge.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: infoX, Y: y - 66},
			Size:   foundation.Size{Width: badgeWidth, Height: 18},
		})
		contentView.AddSubview(methodBadge)

		methodLabel := appkit.NewTextField()
		methodLabel.SetStringValue(methodText)
		methodLabel.SetEditable(false)
		methodLabel.SetBordered(false)
		methodLabel.SetDrawsBackground(false)
		methodLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(11, appkit.FontWeightMedium))
		methodLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		methodLabel.SetAlignment(appkit.TextAlignmentCenter)
		methodLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: infoX, Y: y - 65},
			Size:   foundation.Size{Width: badgeWidth, Height: 16},
		})
		contentView.AddSubview(methodLabel)

		y -= iconSize + sectionGap + 16 // Extra space after header for visual separation

		// ═══════════════════════════════════════════════════════════════
		// UPDATE BANNER (if available)
		// ═══════════════════════════════════════════════════════════════
		if hasUpdate {
			bannerHeight := 56.0

			updateBanner := appkit.NewBox()
			updateBanner.SetBoxType(appkit.BoxCustom)
			updateBanner.SetCornerRadius(boxRadius)
			updateBanner.SetFillColor(appkit.Color_SystemGreenColor().ColorWithAlphaComponent(0.1))
			updateBanner.SetBorderColor(appkit.Color_SystemGreenColor().ColorWithAlphaComponent(0.3))
			updateBanner.SetBorderWidth(1)
			updateBanner.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: windowPadding, Y: y - bannerHeight},
				Size:   foundation.Size{Width: contentWidth, Height: bannerHeight},
			})
			contentView.AddSubview(updateBanner)

			// Update title
			updateTitle := appkit.NewTextField()
			updateTitle.SetStringValue("Update Available")
			updateTitle.SetEditable(false)
			updateTitle.SetBordered(false)
			updateTitle.SetDrawsBackground(false)
			updateTitle.SetFont(appkit.Font_SystemFontOfSizeWeight(13, appkit.FontWeightSemibold))
			updateTitle.SetTextColor(appkit.Color_SystemGreenColor())
			updateTitle.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: y - 22},
				Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2, Height: 18},
			})
			contentView.AddSubview(updateTitle)

			// Version comparison
			versionCompare := appkit.NewTextField()
			versionCompare.SetStringValue(fmt.Sprintf("%s  →  %s", version, installation.LatestVersion.String()))
			versionCompare.SetEditable(false)
			versionCompare.SetBordered(false)
			versionCompare.SetDrawsBackground(false)
			versionCompare.SetFont(appkit.Font_SystemFontOfSize(12))
			versionCompare.SetTextColor(appkit.Color_LabelColor())
			versionCompare.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: y - 42},
				Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2, Height: 16},
			})
			contentView.AddSubview(versionCompare)

			y -= bannerHeight + sectionGap
		}

		// ═══════════════════════════════════════════════════════════════
		// DETAILS SECTION
		// ═══════════════════════════════════════════════════════════════

		detailsSectionLabel := appkit.NewTextField()
		detailsSectionLabel.SetStringValue("DETAILS")
		detailsSectionLabel.SetEditable(false)
		detailsSectionLabel.SetBordered(false)
		detailsSectionLabel.SetDrawsBackground(false)
		detailsSectionLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(11, appkit.FontWeightMedium))
		detailsSectionLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		detailsSectionLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y},
			Size:   foundation.Size{Width: contentWidth, Height: 14},
		})
		contentView.AddSubview(detailsSectionLabel)
		y -= 22

		// Calculate details box height - location row is taller to allow path wrapping
		pathRowHeight := 36.0 // Taller for wrapped path
		detectedRowHeight := rowHeight
		detailsBoxHeight := boxInnerPadding*2 + detectedRowHeight
		if installation.ExecutablePath != "" {
			detailsBoxHeight += pathRowHeight + itemGap
		}

		detailsBox := appkit.NewBox()
		detailsBox.SetBoxType(appkit.BoxCustom)
		detailsBox.SetCornerRadius(boxRadius)
		detailsBox.SetFillColor(appkit.Color_QuaternaryLabelColor().ColorWithAlphaComponent(0.1))
		detailsBox.SetBorderColor(appkit.Color_SeparatorColor())
		detailsBox.SetBorderWidth(0.5)
		detailsBox.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: y - detailsBoxHeight},
			Size:   foundation.Size{Width: contentWidth, Height: detailsBoxHeight},
		})
		contentView.AddSubview(detailsBox)

		rowY := y - boxInnerPadding
		labelWidth := 70.0
		valueX := windowPadding + boxInnerPadding + labelWidth + 8

		// Location row (with wrapping path)
		if installation.ExecutablePath != "" {
			locLabel := appkit.NewTextField()
			locLabel.SetStringValue("Location")
			locLabel.SetEditable(false)
			locLabel.SetBordered(false)
			locLabel.SetDrawsBackground(false)
			locLabel.SetFont(appkit.Font_SystemFontOfSize(12))
			locLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
			locLabel.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: rowY - 16},
				Size:   foundation.Size{Width: labelWidth, Height: 16},
			})
			contentView.AddSubview(locLabel)

			// Create a text field for the path - use UsesSingleLineMode(false) to allow wrapping
			locValue := appkit.NewTextField()
			locValue.SetStringValue(installation.ExecutablePath)
			locValue.SetEditable(false)
			locValue.SetSelectable(true) // Allow selecting/copying the path
			locValue.SetBordered(false)
			locValue.SetDrawsBackground(false)
			locValue.SetFont(appkit.Font_MonospacedSystemFontOfSizeWeight(11, appkit.FontWeightRegular))
			locValue.SetUsesSingleLineMode(false)
			locValue.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: valueX, Y: rowY - pathRowHeight + 2},
				Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2 - labelWidth - 8, Height: pathRowHeight},
			})
			contentView.AddSubview(locValue)

			rowY -= pathRowHeight + itemGap
		}

		// Detected row
		detLabel := appkit.NewTextField()
		detLabel.SetStringValue("Detected")
		detLabel.SetEditable(false)
		detLabel.SetBordered(false)
		detLabel.SetDrawsBackground(false)
		detLabel.SetFont(appkit.Font_SystemFontOfSize(12))
		detLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		detLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + boxInnerPadding, Y: rowY - 16},
			Size:   foundation.Size{Width: labelWidth, Height: 16},
		})
		contentView.AddSubview(detLabel)

		detectedStr := "Unknown"
		if !installation.DetectedAt.IsZero() {
			detectedStr = installation.DetectedAt.Format("Jan 2, 2006 at 3:04 PM")
		}
		detValue := appkit.NewTextField()
		detValue.SetStringValue(detectedStr)
		detValue.SetEditable(false)
		detValue.SetBordered(false)
		detValue.SetDrawsBackground(false)
		detValue.SetFont(appkit.Font_SystemFontOfSize(12))
		detValue.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: valueX, Y: rowY - 16},
			Size:   foundation.Size{Width: contentWidth - boxInnerPadding*2 - labelWidth - 8, Height: 16},
		})
		contentView.AddSubview(detValue)

		// ═══════════════════════════════════════════════════════════════
		// FOOTER BUTTONS
		// ═══════════════════════════════════════════════════════════════

		closeBtn := appkit.NewButton()
		closeBtn.SetTitle("Close")
		closeBtn.SetBezelStyle(appkit.BezelStyleRounded)
		closeBtn.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowWidth - windowPadding - 80, Y: windowPadding},
			Size:   foundation.Size{Width: 80, Height: 28},
		})
		action.Set(closeBtn, func(_ objc.Object) {
			win.Close()
		})
		contentView.AddSubview(closeBtn)

		if hasUpdate {
			updateBtn := appkit.NewButton()
			updateBtn.SetTitle("Update Now")
			updateBtn.SetBezelStyle(appkit.BezelStyleRounded)
			updateBtn.SetKeyEquivalent("\r")
			updateBtn.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: windowWidth - windowPadding - 180, Y: windowPadding},
				Size:   foundation.Size{Width: 95, Height: 28},
			})
			action.Set(updateBtn, func(_ objc.Object) {
				win.Close()
				go app.updateSingleAgent(installation)
			})
			contentView.AddSubview(updateBtn)
		}

		win.SetContentView(contentView)
		win.Center()

		// Bring to front
		nsApp := appkit.Application_SharedApplication()
		nsApp.SetActivationPolicy(appkit.ApplicationActivationPolicyRegular)
		nsApp.ActivateIgnoringOtherApps(true)
		win.MakeKeyAndOrderFront(nil)

		windowsMu.Lock()
		activeWindows = append(activeWindows, win)
		windowsMu.Unlock()
	})
}

// closeAllNativeWindows closes all native windows.
func closeAllNativeWindows() {
	dispatch.MainQueue().DispatchAsync(func() {
		windowsMu.Lock()
		defer windowsMu.Unlock()

		for _, win := range activeWindows {
			win.Close()
		}
		activeWindows = nil
		settingsWindow = appkit.Window{}
		settingsWindowOpen = false
	})
}

// hasNativeWindowSupport returns true if native windows are available.
func hasNativeWindowSupport() bool {
	return true
}

// manageAgentRow tracks a row in the manage agents window
type manageAgentRow struct {
	agentDef         catalog.AgentDef
	installed        bool
	hasUpdate        bool
	version          string
	latestVer        string
	checkbox         appkit.Button
	selected         bool
	statusLabel      appkit.TextField
	actionBtn        appkit.Button
	actionPopup      appkit.PopUpButton
	installedMethods []agent.Installation       // All installed methods for this agent
	availableMethods []catalog.InstallMethodDef // Available install methods for platform
}

var (
	manageWindowOpen bool
	manageWindow     appkit.Window
	manageRows       []*manageAgentRow
)

// showNativeManageAgentsWindow displays the agent management window.
//
//nolint:gocyclo // Complex native UI window construction
func (a *App) showNativeManageAgentsWindow(agentDefs []catalog.AgentDef, installedAgents []agent.Installation) {
	app := a
	defs := agentDefs
	installed := installedAgents

	dispatch.MainQueue().DispatchAsync(func() {
		windowsMu.Lock()
		if manageWindowOpen {
			// Bring existing window to foreground
			nsApp := appkit.Application_SharedApplication()
			nsApp.SetActivationPolicy(appkit.ApplicationActivationPolicyRegular)
			nsApp.ActivateIgnoringOtherApps(true)
			manageWindow.MakeKeyAndOrderFront(nil)
			windowsMu.Unlock()
			return
		}
		windowsMu.Unlock()

		// Calculate minimum width based on content
		// Checkbox(32) + Name(200) + Description gap + Status(120) + Button(90) + margins
		minContentWidth := 32.0 + 200.0 + 150.0 + 120.0 + 90.0 + 40.0 // ~632
		minWindowWidth := minContentWidth + (windowPadding * 2)
		if minWindowWidth < 650 {
			minWindowWidth = 650
		}

		windowWidth := minWindowWidth
		windowHeight := 500.0

		win := appkit.NewWindowWithContentRectStyleMaskBackingDefer(
			foundation.Rect{
				Origin: foundation.Point{X: 150, Y: 150},
				Size:   foundation.Size{Width: windowWidth, Height: windowHeight},
			},
			appkit.WindowStyleMaskTitled|
				appkit.WindowStyleMaskClosable|
				appkit.WindowStyleMaskMiniaturizable|
				appkit.WindowStyleMaskResizable,
			appkit.BackingStoreBuffered,
			false,
		)
		win.SetTitle("Manage Agents")
		win.SetReleasedWhenClosed(false)
		win.SetMinSize(foundation.Size{Width: minWindowWidth, Height: 400})

		contentView := appkit.NewView()
		contentView.SetFrameSize(foundation.Size{Width: windowWidth, Height: windowHeight})
		contentView.SetAutoresizingMask(appkit.ViewWidthSizable | appkit.ViewHeightSizable)

		contentWidth := windowWidth - (windowPadding * 2)

		// ═══════════════════════════════════════════════════════════════
		// HEADER
		// ═══════════════════════════════════════════════════════════════
		headerY := windowHeight - windowPadding

		titleLabel := appkit.NewTextField()
		titleLabel.SetStringValue("Manage Agents")
		titleLabel.SetEditable(false)
		titleLabel.SetBordered(false)
		titleLabel.SetDrawsBackground(false)
		titleLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(20, appkit.FontWeightSemibold))
		titleLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: headerY - 28},
			Size:   foundation.Size{Width: contentWidth, Height: 28},
		})
		titleLabel.SetAutoresizingMask(appkit.ViewWidthSizable | appkit.ViewMinYMargin)
		contentView.AddSubview(titleLabel)

		subtitleLabel := appkit.NewTextField()
		subtitleLabel.SetStringValue("Install, update, or remove AI development agents")
		subtitleLabel.SetEditable(false)
		subtitleLabel.SetBordered(false)
		subtitleLabel.SetDrawsBackground(false)
		subtitleLabel.SetFont(appkit.Font_SystemFontOfSize(13))
		subtitleLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
		subtitleLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: headerY - 50},
			Size:   foundation.Size{Width: contentWidth, Height: 18},
		})
		subtitleLabel.SetAutoresizingMask(appkit.ViewWidthSizable | appkit.ViewMinYMargin)
		contentView.AddSubview(subtitleLabel)

		// ═══════════════════════════════════════════════════════════════
		// AGENT LIST (Scroll View)
		// ═══════════════════════════════════════════════════════════════
		listTop := headerY - 70
		listHeight := windowHeight - 160 // Leave room for header and footer
		listY := listTop - listHeight

		// Create scroll view
		scrollView := appkit.NewScrollView()
		scrollView.SetHasVerticalScroller(true)
		scrollView.SetHasHorizontalScroller(false)
		scrollView.SetAutohidesScrollers(true)
		scrollView.SetBorderType(appkit.BezelBorder)
		scrollView.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding, Y: listY},
			Size:   foundation.Size{Width: contentWidth, Height: listHeight},
		})
		// Scroll view should resize with window
		scrollView.SetAutoresizingMask(appkit.ViewWidthSizable | appkit.ViewHeightSizable)

		// Create document view to hold rows
		rowHeight := 60.0
		actualContentHeight := float64(len(defs)) * rowHeight
		docViewHeight := actualContentHeight

		docView := appkit.NewView()
		// Document view width should resize with scroll view
		docView.SetAutoresizingMask(appkit.ViewWidthSizable)

		// Position document view: if content is smaller than visible area,
		// place doc view at TOP of scroll view (higher Y origin in non-flipped coords)
		if actualContentHeight < listHeight {
			// Push doc view up so content appears at top of visible area
			docView.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: 0, Y: listHeight - actualContentHeight},
				Size:   foundation.Size{Width: contentWidth, Height: docViewHeight},
			})
		} else {
			docView.SetFrameSize(foundation.Size{Width: contentWidth, Height: docViewHeight})
		}

		// Build lookup map for installed agents - track ALL installations per agent
		installedMap := make(map[string][]agent.Installation)
		for _, inst := range installed {
			installedMap[inst.AgentID] = append(installedMap[inst.AgentID], inst)
		}

		// Get current platform for method filtering
		platformID := string(app.platform.ID())

		// Sort agents alphabetically by name (case-insensitive)
		sort.Slice(defs, func(i, j int) bool {
			return strings.ToLower(defs[i].Name) < strings.ToLower(defs[j].Name)
		})

		// Create rows for each agent
		manageRows = make([]*manageAgentRow, 0, len(defs))
		for i, def := range defs {
			row := &manageAgentRow{
				agentDef: def,
			}

			// Get available install methods for this platform
			row.availableMethods = def.GetSupportedMethods(platformID)

			// Check if installed (may have multiple installations via different methods)
			if installations, ok := installedMap[def.ID]; ok && len(installations) > 0 {
				row.installed = true
				row.installedMethods = installations
				// Use first installation for version display
				row.version = installations[0].InstalledVersion.String()
				if installations[0].LatestVersion != nil {
					row.latestVer = installations[0].LatestVersion.String()
				}
				// Check if any installation has an update
				for _, inst := range installations {
					if inst.HasUpdate() {
						row.hasUpdate = true
						break
					}
				}
			}

			// Position rows from top (high Y) down to bottom (low Y)
			// In non-flipped coords: high Y = visual top, low Y = visual bottom
			rowY := docViewHeight - float64(i+1)*rowHeight
			rowWidth := contentWidth

			// Layout constants for clean alignment
			rowPadding := 12.0
			checkboxSize := 18.0
			textStartX := rowPadding + checkboxSize + 14.0 // After checkbox with gap
			buttonWidth := 90.0
			buttonRightMargin := 10.0
			versionAreaWidth := 150.0 // INSTALLED/CURRENT labels + values
			buttonX := rowWidth - buttonRightMargin - buttonWidth
			versionAreaX := buttonX - versionAreaWidth - 16.0 // Increased gap before button

			// Vertical centering: rowHeight=60, we want name+desc (~34px) centered
			nameY := rowY + 34
			descY := rowY + 14
			checkboxY := rowY + (rowHeight-checkboxSize)/2

			// Row background (alternating)
			if i%2 == 0 {
				rowBg := appkit.NewBox()
				rowBg.SetBoxType(appkit.BoxCustom)
				rowBg.SetFillColor(appkit.Color_QuaternaryLabelColor().ColorWithAlphaComponent(0.05))
				rowBg.SetBorderWidth(0)
				rowBg.SetFrame(foundation.Rect{
					Origin: foundation.Point{X: 0, Y: rowY},
					Size:   foundation.Size{Width: rowWidth, Height: rowHeight},
				})
				rowBg.SetAutoresizingMask(appkit.ViewWidthSizable)
				docView.AddSubview(rowBg)
			}

			// Checkbox (vertically centered)
			checkbox := appkit.NewButton()
			checkbox.SetButtonType(appkit.ButtonTypeSwitch)
			checkbox.SetTitle("")
			checkbox.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: rowPadding, Y: checkboxY},
				Size:   foundation.Size{Width: checkboxSize, Height: checkboxSize},
			})
			row.checkbox = checkbox
			docView.AddSubview(checkbox)

			// Agent name
			nameLabel := appkit.NewTextField()
			nameLabel.SetStringValue(def.Name)
			nameLabel.SetEditable(false)
			nameLabel.SetBordered(false)
			nameLabel.SetDrawsBackground(false)
			nameLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(13, appkit.FontWeightMedium))
			nameLabel.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: textStartX, Y: nameY},
				Size:   foundation.Size{Width: versionAreaX - textStartX - 8, Height: 18},
			})
			docView.AddSubview(nameLabel)

			// Description
			descLabel := appkit.NewTextField()
			descLabel.SetStringValue(def.Description)
			descLabel.SetEditable(false)
			descLabel.SetBordered(false)
			descLabel.SetDrawsBackground(false)
			descLabel.SetFont(appkit.Font_SystemFontOfSize(11))
			descLabel.SetTextColor(appkit.Color_SecondaryLabelColor())
			descLabel.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: textStartX, Y: descY},
				Size:   foundation.Size{Width: versionAreaX - textStartX - 8, Height: 16},
			})
			descLabel.SetAutoresizingMask(appkit.ViewWidthSizable)
			docView.AddSubview(descLabel)

			// Version status area - two rows with top padding
			labelColWidth := 58.0
			valueColWidth := versionAreaWidth - labelColWidth
			versionTopY := rowY + 30 // Moved down for top padding
			versionBotY := rowY + 14 // Moved down for top padding

			// INSTALLED label
			installedLabel := appkit.NewTextField()
			installedLabel.SetStringValue("INSTALLED")
			installedLabel.SetEditable(false)
			installedLabel.SetBordered(false)
			installedLabel.SetDrawsBackground(false)
			installedLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(9, appkit.FontWeightMedium))
			installedLabel.SetTextColor(appkit.Color_TertiaryLabelColor())
			installedLabel.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: versionAreaX, Y: versionTopY},
				Size:   foundation.Size{Width: labelColWidth, Height: 12},
			})
			installedLabel.SetAutoresizingMask(appkit.ViewMinXMargin)
			docView.AddSubview(installedLabel)

			// Installed version value
			installedVerLabel := appkit.NewTextField()
			if row.installed {
				installedVerLabel.SetStringValue(row.version)
				installedVerLabel.SetTextColor(appkit.Color_LabelColor())
			} else {
				installedVerLabel.SetStringValue("None")
				installedVerLabel.SetTextColor(appkit.Color_TertiaryLabelColor())
			}
			installedVerLabel.SetEditable(false)
			installedVerLabel.SetBordered(false)
			installedVerLabel.SetDrawsBackground(false)
			installedVerLabel.SetFont(appkit.Font_SystemFontOfSize(11))
			installedVerLabel.SetAlignment(appkit.TextAlignmentRight)
			installedVerLabel.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: versionAreaX + labelColWidth, Y: versionTopY - 1},
				Size:   foundation.Size{Width: valueColWidth, Height: 14},
			})
			installedVerLabel.SetAutoresizingMask(appkit.ViewMinXMargin)
			docView.AddSubview(installedVerLabel)

			// CURRENT label
			currentLabel := appkit.NewTextField()
			currentLabel.SetStringValue("CURRENT")
			currentLabel.SetEditable(false)
			currentLabel.SetBordered(false)
			currentLabel.SetDrawsBackground(false)
			currentLabel.SetFont(appkit.Font_SystemFontOfSizeWeight(9, appkit.FontWeightMedium))
			currentLabel.SetTextColor(appkit.Color_TertiaryLabelColor())
			currentLabel.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: versionAreaX, Y: versionBotY},
				Size:   foundation.Size{Width: labelColWidth, Height: 12},
			})
			currentLabel.SetAutoresizingMask(appkit.ViewMinXMargin)
			docView.AddSubview(currentLabel)

			// Current version value
			currentVerLabel := appkit.NewTextField()
			currentVer := row.latestVer
			if currentVer == "" {
				if row.installed {
					currentVer = row.version
				} else {
					currentVer = "—"
				}
			}
			currentVerLabel.SetStringValue(currentVer)
			currentVerLabel.SetEditable(false)
			currentVerLabel.SetBordered(false)
			currentVerLabel.SetDrawsBackground(false)
			currentVerLabel.SetFont(appkit.Font_SystemFontOfSize(11))
			if row.hasUpdate {
				currentVerLabel.SetTextColor(appkit.Color_SystemGreenColor())
			} else if row.installed {
				currentVerLabel.SetTextColor(appkit.Color_LabelColor())
			} else {
				currentVerLabel.SetTextColor(appkit.Color_TertiaryLabelColor())
			}
			currentVerLabel.SetAlignment(appkit.TextAlignmentRight)
			currentVerLabel.SetFrame(foundation.Rect{
				Origin: foundation.Point{X: versionAreaX + labelColWidth, Y: versionBotY - 1},
				Size:   foundation.Size{Width: valueColWidth, Height: 14},
			})
			currentVerLabel.SetAutoresizingMask(appkit.ViewMinXMargin)
			row.statusLabel = currentVerLabel
			docView.AddSubview(currentVerLabel)

			// Action button/popup (fixed right, vertically centered)
			currentRow := row

			if row.hasUpdate {
				// Update button - always regular button
				actionBtn := appkit.NewButton()
				actionBtn.SetBezelStyle(appkit.BezelStyleRounded)
				actionBtn.SetControlSize(appkit.ControlSizeSmall)
				actionBtn.SetTitle("Update")
				actionBtn.SetAlignment(appkit.TextAlignmentLeft)
				actionBtn.SetFrame(foundation.Rect{
					Origin: foundation.Point{X: buttonX, Y: rowY + 18},
					Size:   foundation.Size{Width: buttonWidth, Height: 24},
				})
				actionBtn.SetAutoresizingMask(appkit.ViewMinXMargin)
				row.actionBtn = actionBtn
				action.Set(actionBtn, func(_ objc.Object) {
					go app.performAgentAction(currentRow, win)
				})
				docView.AddSubview(actionBtn)
			} else if row.installed {
				// Uninstall - use popup if multiple installations
				if len(row.installedMethods) > 1 {
					// Popup button for multiple installed methods
					popup := appkit.NewPopUpButtonWithFramePullsDown(
						foundation.Rect{
							Origin: foundation.Point{X: buttonX, Y: rowY + 18},
							Size:   foundation.Size{Width: buttonWidth, Height: 24},
						},
						true, // pullsDown mode
					)
					popup.SetControlSize(appkit.ControlSizeSmall)
					popup.SetFont(appkit.Font_SystemFontOfSize(11)) // Match regular button text size
					popup.SetAutoresizingMask(appkit.ViewMinXMargin)

					// First item is the button title
					popup.AddItemWithTitle("Uninstall")
					// Add method options
					for _, inst := range row.installedMethods {
						popup.AddItemWithTitle(string(inst.Method))
					}

					// Set action for menu items
					for idx := 1; idx < popup.NumberOfItems(); idx++ {
						methodIdx := idx - 1 // Offset for title item
						item := popup.ItemAtIndex(idx)
						item.SetTag(methodIdx)
					}

					row.actionPopup = popup
					action.Set(popup, func(_ objc.Object) {
						selectedIdx := popup.IndexOfSelectedItem()
						if selectedIdx > 0 {
							methodIdx := selectedIdx - 1
							if methodIdx < len(currentRow.installedMethods) {
								go app.performAgentActionWithMethod(currentRow, win, actionUninstall, string(currentRow.installedMethods[methodIdx].Method))
							}
						}
					})
					docView.AddSubview(popup)
				} else {
					// Single installation - regular button
					actionBtn := appkit.NewButton()
					actionBtn.SetBezelStyle(appkit.BezelStyleRounded)
					actionBtn.SetControlSize(appkit.ControlSizeSmall)
					actionBtn.SetTitle("Uninstall")
					actionBtn.SetAlignment(appkit.TextAlignmentLeft)
					actionBtn.SetContentTintColor(appkit.Color_SystemRedColor())
					actionBtn.SetFrame(foundation.Rect{
						Origin: foundation.Point{X: buttonX, Y: rowY + 18},
						Size:   foundation.Size{Width: buttonWidth, Height: 24},
					})
					actionBtn.SetAutoresizingMask(appkit.ViewMinXMargin)
					row.actionBtn = actionBtn
					action.Set(actionBtn, func(_ objc.Object) {
						go app.performAgentAction(currentRow, win)
					})
					docView.AddSubview(actionBtn)
				}
			} else {
				// Install - use popup if multiple methods available
				if len(row.availableMethods) > 1 {
					// Popup button for multiple install methods
					popup := appkit.NewPopUpButtonWithFramePullsDown(
						foundation.Rect{
							Origin: foundation.Point{X: buttonX, Y: rowY + 18},
							Size:   foundation.Size{Width: buttonWidth, Height: 24},
						},
						true, // pullsDown mode
					)
					popup.SetControlSize(appkit.ControlSizeSmall)
					popup.SetFont(appkit.Font_SystemFontOfSize(11)) // Match regular button text size
					popup.SetAutoresizingMask(appkit.ViewMinXMargin)

					// First item is the button title
					popup.AddItemWithTitle("Install")
					// Add method options
					for _, method := range row.availableMethods {
						popup.AddItemWithTitle(method.Method)
					}

					row.actionPopup = popup
					action.Set(popup, func(_ objc.Object) {
						selectedIdx := popup.IndexOfSelectedItem()
						if selectedIdx > 0 {
							methodIdx := selectedIdx - 1
							if methodIdx < len(currentRow.availableMethods) {
								go app.performAgentActionWithMethod(currentRow, win, "install", currentRow.availableMethods[methodIdx].Method)
							}
						}
					})
					docView.AddSubview(popup)
				} else {
					// Single method - regular button
					actionBtn := appkit.NewButton()
					actionBtn.SetBezelStyle(appkit.BezelStyleRounded)
					actionBtn.SetControlSize(appkit.ControlSizeSmall)
					actionBtn.SetTitle("Install")
					actionBtn.SetAlignment(appkit.TextAlignmentLeft)
					actionBtn.SetFrame(foundation.Rect{
						Origin: foundation.Point{X: buttonX, Y: rowY + 18},
						Size:   foundation.Size{Width: buttonWidth, Height: 24},
					})
					actionBtn.SetAutoresizingMask(appkit.ViewMinXMargin)
					row.actionBtn = actionBtn
					action.Set(actionBtn, func(_ objc.Object) {
						go app.performAgentAction(currentRow, win)
					})
					docView.AddSubview(actionBtn)
				}
			}

			manageRows = append(manageRows, row)
		}

		scrollView.SetDocumentView(docView)
		contentView.AddSubview(scrollView)

		// Content is positioned from Y=0 (top in flipped clip view), so it naturally
		// appears at the top of the scroll view - no manual scrolling needed

		// ═══════════════════════════════════════════════════════════════
		// FOOTER - Bulk actions
		// ═══════════════════════════════════════════════════════════════
		footerY := windowPadding

		// Status label for results (flexible width, fixed bottom)
		resultLabel := appkit.NewTextField()
		resultLabel.SetStringValue("")
		resultLabel.SetEditable(false)
		resultLabel.SetBordered(false)
		resultLabel.SetDrawsBackground(false)
		resultLabel.SetFont(appkit.Font_SystemFontOfSize(12))
		resultLabel.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + 120, Y: footerY},
			Size:   foundation.Size{Width: 250, Height: 20},
		})
		resultLabel.SetAutoresizingMask(appkit.ViewWidthSizable | appkit.ViewMaxYMargin)
		contentView.AddSubview(resultLabel)

		// Close button (fixed right, fixed bottom)
		closeBtn := appkit.NewButton()
		closeBtn.SetTitle("Done")
		closeBtn.SetBezelStyle(appkit.BezelStyleRounded)
		closeBtn.SetKeyEquivalent("\x1b") // Escape key
		closeBtn.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowWidth - windowPadding - 80, Y: footerY},
			Size:   foundation.Size{Width: 80, Height: 28},
		})
		closeBtn.SetAutoresizingMask(appkit.ViewMinXMargin | appkit.ViewMaxYMargin)
		action.Set(closeBtn, func(_ objc.Object) {
			windowsMu.Lock()
			manageWindowOpen = false
			windowsMu.Unlock()
			win.Close()
		})
		contentView.AddSubview(closeBtn)

		// Bulk Uninstall button (fixed right, fixed bottom)
		bulkUninstallBtn := appkit.NewButton()
		bulkUninstallBtn.SetTitle("Uninstall Selected")
		bulkUninstallBtn.SetBezelStyle(appkit.BezelStyleRounded)
		bulkUninstallBtn.SetContentTintColor(appkit.Color_SystemRedColor())
		bulkUninstallBtn.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowWidth - windowPadding - 350, Y: footerY},
			Size:   foundation.Size{Width: 130, Height: 28},
		})
		bulkUninstallBtn.SetAutoresizingMask(appkit.ViewMinXMargin | appkit.ViewMaxYMargin)
		bulkUninstallBtn.SetEnabled(false) // Initially disabled
		action.Set(bulkUninstallBtn, func(_ objc.Object) {
			go app.performBulkAction("uninstall", resultLabel, win)
		})
		contentView.AddSubview(bulkUninstallBtn)

		// Bulk Install button (fixed right, fixed bottom)
		bulkInstallBtn := appkit.NewButton()
		bulkInstallBtn.SetTitle("Install Selected")
		bulkInstallBtn.SetBezelStyle(appkit.BezelStyleRounded)
		bulkInstallBtn.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowWidth - windowPadding - 210, Y: footerY},
			Size:   foundation.Size{Width: 120, Height: 28},
		})
		bulkInstallBtn.SetAutoresizingMask(appkit.ViewMinXMargin | appkit.ViewMaxYMargin)
		bulkInstallBtn.SetEnabled(false) // Initially disabled
		action.Set(bulkInstallBtn, func(_ objc.Object) {
			go app.performBulkAction("install", resultLabel, win)
		})
		contentView.AddSubview(bulkInstallBtn)

		// Helper function to update bulk button enabled state
		updateBulkButtons := func() {
			hasSelection := false
			for _, row := range manageRows {
				if row.checkbox.State() == appkit.ControlStateValueOn {
					hasSelection = true
					break
				}
			}
			bulkInstallBtn.SetEnabled(hasSelection)
			bulkUninstallBtn.SetEnabled(hasSelection)
		}

		// Select All checkbox (fixed left, fixed bottom)
		// Align with list checkboxes (scroll view X + checkbox X = windowPadding + 12)
		// Center vertically with buttons (button height 28, checkbox ~20)
		selectAllCheck := appkit.NewButton()
		selectAllCheck.SetButtonType(appkit.ButtonTypeSwitch)
		selectAllCheck.SetTitle("Select All")
		selectAllCheck.SetFont(appkit.Font_SystemFontOfSize(12))
		selectAllCheck.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: windowPadding + 12, Y: footerY + 4},
			Size:   foundation.Size{Width: 100, Height: 20},
		})
		selectAllCheck.SetAutoresizingMask(appkit.ViewMaxXMargin | appkit.ViewMaxYMargin)
		action.Set(selectAllCheck, func(sender objc.Object) {
			btn := appkit.ButtonFrom(sender.Ptr())
			selected := btn.State() == appkit.ControlStateValueOn
			for _, row := range manageRows {
				if selected {
					row.checkbox.SetState(appkit.ControlStateValueOn)
				} else {
					row.checkbox.SetState(appkit.ControlStateValueOff)
				}
				row.selected = selected
			}
			updateBulkButtons()
		})
		contentView.AddSubview(selectAllCheck)

		// Add click handlers to individual checkboxes to update bulk button state
		for _, row := range manageRows {
			currentRow := row
			action.Set(currentRow.checkbox, func(_ objc.Object) {
				updateBulkButtons()
			})
		}

		win.SetContentView(contentView)
		win.Center()

		// Bring to front
		nsApp := appkit.Application_SharedApplication()
		nsApp.SetActivationPolicy(appkit.ApplicationActivationPolicyRegular)
		nsApp.ActivateIgnoringOtherApps(true)
		win.MakeKeyAndOrderFront(nil)

		windowsMu.Lock()
		manageWindow = win
		manageWindowOpen = true
		activeWindows = append(activeWindows, win)
		windowsMu.Unlock()
	})
}

// performAgentAction performs install/update/uninstall on a single agent.
func (a *App) performAgentAction(row *manageAgentRow, parentWin appkit.Window) {
	var actionType string
	if row.hasUpdate {
		actionType = actionUpdate
	} else if row.installed {
		actionType = actionUninstall
	} else {
		actionType = actionInstall
	}

	// Show progress
	a.showProgressWindow(fmt.Sprintf("%sing %s...", actionType, row.agentDef.Name), parentWin)

	var success bool
	var err error

	switch actionType {
	case actionInstall:
		success, err = a.installAgent(row.agentDef)
	case actionUpdate:
		success, err = a.updateAgentByID(row.agentDef.ID)
	case actionUninstall:
		success, err = a.uninstallAgent(row.agentDef)
	}

	// Update UI
	dispatch.MainQueue().DispatchAsync(func() {
		closeProgressWindow()

		if success && err == nil {
			// Update row state
			switch actionType {
			case actionInstall, actionUpdate:
				row.installed = true
				row.hasUpdate = false
				row.statusLabel.SetStringValue("Installed")
				row.statusLabel.SetTextColor(appkit.Color_LabelColor())
				row.actionBtn.SetTitle("Uninstall")
				row.actionBtn.SetContentTintColor(appkit.Color_SystemRedColor())
			case actionUninstall:
				row.installed = false
				row.hasUpdate = false
				row.statusLabel.SetStringValue("Not Installed")
				row.statusLabel.SetTextColor(appkit.Color_TertiaryLabelColor())
				row.actionBtn.SetTitle("Install")
				row.actionBtn.SetContentTintColor(appkit.Color_ControlAccentColor())
			}
		}
	})

	// Refresh agent list
	a.refreshAgents(a.ctx)
}

// performAgentActionWithMethod performs install/uninstall with a specific method.
func (a *App) performAgentActionWithMethod(row *manageAgentRow, parentWin appkit.Window, actionType, method string) {
	// Show progress
	a.showProgressWindow(fmt.Sprintf("%sing %s via %s...", actionType, row.agentDef.Name, method), parentWin)

	var success bool
	var err error

	switch actionType {
	case actionInstall:
		success, err = a.installAgentWithMethod(row.agentDef, method)
	case actionUninstall:
		success, err = a.uninstallAgentWithMethod(row.agentDef, method)
	}

	// Update UI
	dispatch.MainQueue().DispatchAsync(func() {
		closeProgressWindow()

		if success && err == nil {
			switch actionType {
			case actionInstall:
				row.installed = true
				row.hasUpdate = false
				if row.statusLabel.Ptr() != nil {
					row.statusLabel.SetStringValue("Installed")
					row.statusLabel.SetTextColor(appkit.Color_LabelColor())
				}
			case actionUninstall:
				// Only mark as not installed if no other methods remain installed
				row.installed = false
				row.hasUpdate = false
				if row.statusLabel.Ptr() != nil {
					row.statusLabel.SetStringValue("Not Installed")
					row.statusLabel.SetTextColor(appkit.Color_TertiaryLabelColor())
				}
			}
		}
	})

	// Refresh agent list
	a.refreshAgents(a.ctx)
}

// performBulkAction performs an action on all selected agents.
func (a *App) performBulkAction(actionType string, resultLabel appkit.TextField, parentWin appkit.Window) {
	// Get selected rows
	var selected []*manageAgentRow
	for _, row := range manageRows {
		if row.checkbox.State() == appkit.ControlStateValueOn {
			selected = append(selected, row)
		}
	}

	if len(selected) == 0 {
		dispatch.MainQueue().DispatchAsync(func() {
			resultLabel.SetStringValue("No agents selected")
			resultLabel.SetTextColor(appkit.Color_SystemOrangeColor())
		})
		return
	}

	// Show progress
	a.showProgressWindow(fmt.Sprintf("%sing %d agents...", actionType, len(selected)), parentWin)

	successCount := 0
	for _, row := range selected {
		var success bool
		var err error

		switch actionType {
		case actionInstall:
			if !row.installed {
				success, err = a.installAgent(row.agentDef)
			} else {
				continue
			}
		case actionUpdate:
			if row.hasUpdate {
				success, err = a.updateAgentByID(row.agentDef.ID)
			} else {
				continue
			}
		case actionUninstall:
			if row.installed {
				success, err = a.uninstallAgent(row.agentDef)
			} else {
				continue
			}
		}

		if success && err == nil {
			successCount++
		}
	}

	// Update UI
	dispatch.MainQueue().DispatchAsync(func() {
		closeProgressWindow()
		resultLabel.SetStringValue(fmt.Sprintf("%d agent(s) processed", successCount))
		resultLabel.SetTextColor(appkit.Color_SystemGreenColor())
	})

	// Refresh agent list
	a.refreshAgents(a.ctx)
}

// Progress window
var (
	progressWindow     appkit.Window
	progressWindowOpen bool
	progressLabel      appkit.TextField
)

// showProgressWindow displays a progress indicator window.
func (a *App) showProgressWindow(message string, parentWin appkit.Window) {
	dispatch.MainQueue().DispatchAsync(func() {
		if progressWindowOpen {
			progressLabel.SetStringValue(message)
			return
		}

		windowWidth := 280.0
		windowHeight := 90.0

		win := appkit.NewWindowWithContentRectStyleMaskBackingDefer(
			foundation.Rect{
				Origin: foundation.Point{X: 300, Y: 300},
				Size:   foundation.Size{Width: windowWidth, Height: windowHeight},
			},
			appkit.WindowStyleMaskTitled,
			appkit.BackingStoreBuffered,
			false,
		)
		win.SetTitle("Working...")
		win.SetReleasedWhenClosed(false)

		contentView := appkit.NewView()
		contentView.SetFrameSize(foundation.Size{Width: windowWidth, Height: windowHeight})

		// Progress indicator (left side)
		progress := appkit.NewProgressIndicator()
		progress.SetStyle(appkit.ProgressIndicatorStyleSpinning)
		progress.SetControlSize(appkit.ControlSizeSmall)
		progress.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: 20, Y: (windowHeight - 20) / 2},
			Size:   foundation.Size{Width: 20, Height: 20},
		})
		progress.StartAnimation(nil)
		contentView.AddSubview(progress)

		// Message label (right of spinner)
		label := appkit.NewTextField()
		label.SetStringValue(message)
		label.SetEditable(false)
		label.SetBordered(false)
		label.SetDrawsBackground(false)
		label.SetFont(appkit.Font_SystemFontOfSize(13))
		label.SetFrame(foundation.Rect{
			Origin: foundation.Point{X: 50, Y: (windowHeight - 18) / 2},
			Size:   foundation.Size{Width: windowWidth - 70, Height: 18},
		})
		progressLabel = label
		contentView.AddSubview(label)

		win.SetContentView(contentView)
		win.Center()

		// Show the progress window
		win.MakeKeyAndOrderFront(nil)

		progressWindow = win
		progressWindowOpen = true
	})
}

// closeProgressWindow closes the progress window.
func closeProgressWindow() {
	dispatch.MainQueue().DispatchAsync(func() {
		if progressWindowOpen && progressWindow.Ptr() != nil {
			progressWindow.Close()
			progressWindowOpen = false
		}
	})
}

// installAgent installs an agent using the first available method.
func (a *App) installAgent(def catalog.AgentDef) (bool, error) {
	// Find the first available install method
	for methodName, methodDef := range def.InstallMethods {
		methodDef.Method = methodName // Ensure method name is set
		_, err := a.installer.Install(a.ctx, def, methodDef, false)
		if err == nil {
			return true, nil
		}
	}
	return false, fmt.Errorf("no suitable install method found")
}

// updateAgentByID updates an agent by its ID.
func (a *App) updateAgentByID(agentID string) (bool, error) {
	// Find the installed agent
	a.agentsMu.RLock()
	var target *agent.Installation
	for i := range a.agents {
		if a.agents[i].AgentID == agentID {
			inst := a.agents[i]
			target = &inst
			break
		}
	}
	a.agentsMu.RUnlock()

	if target == nil {
		return false, fmt.Errorf("agent not found")
	}

	// Get the agent definition from catalog
	agentDef, err := a.catalog.GetAgent(a.ctx, agentID)
	if err != nil {
		return false, err
	}

	// Get the method definition
	methodDef, ok := agentDef.InstallMethods[string(target.Method)]
	if !ok {
		return false, fmt.Errorf("install method not found")
	}
	methodDef.Method = string(target.Method)

	_, err = a.installer.Update(a.ctx, target, *agentDef, methodDef)
	return err == nil, err
}

// uninstallAgent uninstalls an agent.
func (a *App) uninstallAgent(def catalog.AgentDef) (bool, error) {
	// Find the installed agent
	a.agentsMu.RLock()
	var target *agent.Installation
	var methodName string
	for i := range a.agents {
		if a.agents[i].AgentID == def.ID {
			inst := a.agents[i]
			target = &inst
			methodName = string(inst.Method)
			break
		}
	}
	a.agentsMu.RUnlock()

	if target == nil {
		return false, fmt.Errorf("agent not installed")
	}

	// Get the method definition
	methodDef, ok := def.InstallMethods[methodName]
	if !ok {
		return false, fmt.Errorf("install method not found")
	}
	methodDef.Method = methodName

	err := a.installer.Uninstall(a.ctx, target, methodDef)
	return err == nil, err
}

// installAgentWithMethod installs an agent using a specific method.
func (a *App) installAgentWithMethod(def catalog.AgentDef, method string) (bool, error) {
	methodDef, ok := def.InstallMethods[method]
	if !ok {
		return false, fmt.Errorf("install method %s not found", method)
	}
	methodDef.Method = method

	_, err := a.installer.Install(a.ctx, def, methodDef, false)
	return err == nil, err
}

// uninstallAgentWithMethod uninstalls an agent using a specific method.
func (a *App) uninstallAgentWithMethod(def catalog.AgentDef, method string) (bool, error) {
	// Find the installed agent with the specific method
	a.agentsMu.RLock()
	var target *agent.Installation
	for i := range a.agents {
		if a.agents[i].AgentID == def.ID && string(a.agents[i].Method) == method {
			inst := a.agents[i]
			target = &inst
			break
		}
	}
	a.agentsMu.RUnlock()

	if target == nil {
		return false, fmt.Errorf("agent not installed via method %s", method)
	}

	// Get the method definition
	methodDef, ok := def.InstallMethods[method]
	if !ok {
		return false, fmt.Errorf("install method %s not found", method)
	}
	methodDef.Method = method

	err := a.installer.Uninstall(a.ctx, target, methodDef)
	return err == nil, err
}
