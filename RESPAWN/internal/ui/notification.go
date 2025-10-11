package ui

import (
	"fmt"
	"os/exec"
	"respawn/internal/system"
	"strings"
	"time"
)

// NotificationManager handles user notifications
type NotificationManager struct {
	position         NotificationPosition
	respectDND       bool
	lastNotification time.Time
	isInteractive    bool
}

// NotificationPosition defines where notifications appear
type NotificationPosition int

const (
	PositionBottomRight NotificationPosition = iota
	PositionTopRight
	PositionCenter
)

// NotificationType defines notification urgency
type NotificationType int

const (
	NotificationInfo NotificationType = iota
	NotificationSuccess
	NotificationWarning
	NotificationError
)

// RestoreSummary contains restoration completion details
type RestoreSummary struct {
	TotalApps      int
	SuccessfulApps int
	FailedApps     int
	SkippedApps    int
	TotalDuration  time.Duration
	FailedAppNames []string
	StartTime      time.Time
	EndTime        time.Time
}

// CheckpointStatus contains checkpoint operation status
type CheckpointStatus struct {
	Success      bool
	CheckpointID string
	Timestamp    time.Time
	ErrorMessage string
	AppsCount    int
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		position:      PositionBottomRight,
		respectDND:    true,
		isInteractive: true,
	}
}

// ShowRestoreStart shows restoration started notification (silent in Modified Option C)
func (nm *NotificationManager) ShowRestoreStart() error {
	system.Info("Restoration started - silent notification")

	// Silent per Modified Option C - restoration happens in background
	// User will see per-app notifications instead

	return nil
}

// ShowAppRestored shows individual app restoration notification
func (nm *NotificationManager) ShowAppRestored(appName string, timestamp time.Time) error {
	system.Info("Application restored:", appName, "at", timestamp.Format("15:04:05"))

	// Check Do Not Disturb mode
	if nm.respectDND && nm.isDoNotDisturbActive() {
		system.Debug("Do Not Disturb active - notification suppressed")
		return nil
	}

	// Show minimalist notification: "App ✅"
	message := fmt.Sprintf("%s ✅", appName)

	if err := nm.showBannerNotification(message, NotificationSuccess, 2*time.Second); err != nil {
		system.Warn("Failed to show app restored notification:", err)
		return err
	}

	// Wait 2 seconds for user to see notification
	time.Sleep(2 * time.Second)

	return nil
}

// ShowRestoreComplete shows restoration completion summary
func (nm *NotificationManager) ShowRestoreComplete(summary RestoreSummary) error {
	system.Info("Restoration complete - showing summary")

	// Build summary message
	var message string

	if summary.FailedApps == 0 {
		// All successful
		message = fmt.Sprintf(
			"✅ Restored %d applications in %s",
			summary.SuccessfulApps,
			nm.formatDuration(summary.TotalDuration),
		)
	} else {
		// Some failures
		message = fmt.Sprintf(
			"⚠️ Restored %d/%d applications\n%d failed\n\nCheck: respawn --status",
			summary.SuccessfulApps,
			summary.TotalApps,
			summary.FailedApps,
		)
	}

	notificationType := NotificationSuccess
	if summary.FailedApps > 0 {
		notificationType = NotificationWarning
	}

	// Show summary for 5 seconds (longer than per-app notifications)
	if err := nm.showBannerNotification(message, notificationType, 5*time.Second); err != nil {
		system.Error("Failed to show restore complete notification:", err)
		return err
	}

	return nil
}

// ShowCheckpointFailed shows checkpoint failure alert
func (nm *NotificationManager) ShowCheckpointFailed(status CheckpointStatus) error {
	system.Error("Checkpoint failed:", status.ErrorMessage)

	// Always show checkpoint failures (Modified Option C requirement)
	// Even if DND is active

	message := fmt.Sprintf(
		"❌ Checkpoint Failed\n\n%s\n\nTime: %s",
		status.ErrorMessage,
		status.Timestamp.Format("15:04:05"),
	)

	if err := nm.showBannerNotification(message, NotificationError, 10*time.Second); err != nil {
		system.Error("Failed to show checkpoint failed notification:", err)
		return err
	}

	return nil
}

// ShowCheckpointSuccess shows checkpoint creation confirmation (silent per Modified Option C)
func (nm *NotificationManager) ShowCheckpointSuccess(status CheckpointStatus) error {
	system.Debug("Checkpoint created successfully:", status.CheckpointID)

	// Silent per Modified Option C
	// User can check status via: respawn --status

	return nil
}

// ShowError shows error notification
func (nm *NotificationManager) ShowError(title, message string) error {
	system.Error(title, ":", message)

	// Always show errors, bypass DND
	fullMessage := fmt.Sprintf("%s\n\n%s", title, message)

	if err := nm.showBannerNotification(fullMessage, NotificationError, 10*time.Second); err != nil {
		system.Error("Failed to show error notification:", err)
		return err
	}

	return nil
}

// ShowTeamCheckpointShared shows team checkpoint sharing notification
func (nm *NotificationManager) ShowTeamCheckpointShared(teamSize int, checkpointID string) error {
	system.Info("Team checkpoint shared with", teamSize, "members")

	// Check DND for team notifications
	if nm.respectDND && nm.isDoNotDisturbActive() {
		system.Debug("Do Not Disturb active - team notification suppressed")
		return nil
	}

	message := fmt.Sprintf(
		"📤 Checkpoint shared with team (%d members)\n%s",
		teamSize,
		checkpointID,
	)

	if err := nm.showBannerNotification(message, NotificationInfo, 3*time.Second); err != nil {
		system.Warn("Failed to show team checkpoint notification:", err)
		return err
	}

	return nil
}

// ShowTeamCheckpointAvailable shows team checkpoint available notification
func (nm *NotificationManager) ShowTeamCheckpointAvailable(checkpointID string, memberName string) error {
	system.Info("New team checkpoint available from", memberName)

	// Check DND for team notifications
	if nm.respectDND && nm.isDoNotDisturbActive() {
		system.Debug("Do Not Disturb active - team notification suppressed")
		return nil
	}

	message := fmt.Sprintf(
		"📥 New team checkpoint available\nFrom: %s\n%s",
		memberName,
		checkpointID,
	)

	if err := nm.showBannerNotification(message, NotificationInfo, 3*time.Second); err != nil {
		system.Warn("Failed to show team checkpoint notification:", err)
		return err
	}

	return nil
}

// showBannerNotification displays a banner notification using macOS native notifications
func (nm *NotificationManager) showBannerNotification(message string, notifType NotificationType, duration time.Duration) error {
	// Escape quotes in message for AppleScript
	escapedMessage := strings.ReplaceAll(message, `"`, `\"`)
	escapedMessage = strings.ReplaceAll(escapedMessage, "\n", "\\n")

	// Build AppleScript notification
	script := fmt.Sprintf(`
        display notification "%s" with title "RESPAWN" sound name "Glass"
    `, escapedMessage)

	// Execute AppleScript
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to show notification: %w (output: %s)", err, string(output))
	}

	system.Debug("Notification shown:", message)
	nm.lastNotification = time.Now()

	return nil
}

// isDoNotDisturbActive checks if macOS Do Not Disturb is enabled
func (nm *NotificationManager) isDoNotDisturbActive() bool {
	// Check macOS Focus mode status
	// Using plutil to read Focus preferences
	cmd := exec.Command("defaults", "read", "com.apple.ncprefs", "dnd_prefs")
	output, err := cmd.Output()
	if err != nil {
		// If we can't read DND status, assume it's not active
		system.Debug("Could not read DND status, assuming inactive")
		return false
	}

	// Simple check - if DND plist exists and contains "enabled"
	dndActive := strings.Contains(string(output), "userPref") &&
		strings.Contains(string(output), "enabled = 1")

	if dndActive {
		system.Debug("Do Not Disturb is active")
	}

	return dndActive
}

// formatDuration formats duration for user display
func (nm *NotificationManager) formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())

	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60

	if remainingSeconds == 0 {
		return fmt.Sprintf("%d minutes", minutes)
	}

	return fmt.Sprintf("%d minutes %d seconds", minutes, remainingSeconds)
}

// GetLastNotificationTime returns when the last notification was shown
func (nm *NotificationManager) GetLastNotificationTime() time.Time {
	return nm.lastNotification
}

// SetRespectDND enables or disables Do Not Disturb respect
func (nm *NotificationManager) SetRespectDND(respect bool) {
	nm.respectDND = respect
	system.Debug("Do Not Disturb respect set to:", respect)
}

// SetInteractive enables or disables interactive notifications
func (nm *NotificationManager) SetInteractive(interactive bool) {
	nm.isInteractive = interactive
	system.Debug("Interactive notifications set to:", interactive)
}

// ShowRestorationProgress shows detailed restoration progress (for interactive mode)
func (nm *NotificationManager) ShowRestorationProgress(current, total int, currentApp string) error {
	if !nm.isInteractive {
		return nil // Non-interactive mode
	}

	message := fmt.Sprintf(
		"Restoring: %s\n%d of %d applications",
		currentApp,
		current,
		total,
	)

	// Brief notification, will be replaced by next app
	if err := nm.showBannerNotification(message, NotificationInfo, 1*time.Second); err != nil {
		return err
	}

	return nil
}

// ShowStatusSummary shows current RESPAWN status (for manual status checks)
func (nm *NotificationManager) ShowStatusSummary(summary StatusSummary) error {
	system.Info("Showing status summary")

	message := fmt.Sprintf(
		"RESPAWN Status\n\n"+
			"Last Checkpoint: %s\n"+
			"Total Checkpoints: %d\n"+
			"Auto-start: %s\n"+
			"Health: %s",
		summary.LastCheckpoint.Format("15:04 PM"),
		summary.TotalCheckpoints,
		nm.boolToStatus(summary.AutoStartEnabled),
		summary.HealthStatus,
	)

	if err := nm.showBannerNotification(message, NotificationInfo, 8*time.Second); err != nil {
		return err
	}

	return nil
}

// StatusSummary contains RESPAWN status information
type StatusSummary struct {
	LastCheckpoint   time.Time
	TotalCheckpoints int
	AutoStartEnabled bool
	HealthStatus     string
}

// boolToStatus converts boolean to status string
func (nm *NotificationManager) boolToStatus(enabled bool) string {
	if enabled {
		return "✅ Enabled"
	}
	return "❌ Disabled"
}

// ShowCriticalAlert shows critical system alert (crashes, major failures)
func (nm *NotificationManager) ShowCriticalAlert(title, message string) error {
	system.Error("Critical alert:", title, "-", message)

	// Critical alerts always bypass DND
	// Use macOS dialog for critical alerts (more prominent than notifications)
	script := fmt.Sprintf(`
        display dialog "%s" with title "%s" buttons {"OK"} default button "OK" with icon stop
    `, strings.ReplaceAll(message, `"`, `\"`), title)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		// Fallback to notification if dialog fails
		return nm.showBannerNotification(
			fmt.Sprintf("%s\n\n%s", title, message),
			NotificationError,
			15*time.Second,
		)
	}

	return nil
}

// ShowPermissionRequest shows permission request dialog
func (nm *NotificationManager) ShowPermissionRequest(permissionType, instructions string) error {
	system.Info("Requesting permission:", permissionType)

	message := fmt.Sprintf(
		"RESPAWN needs %s permission.\n\n%s",
		permissionType,
		instructions,
	)

	script := fmt.Sprintf(`
        display dialog "%s" with title "Permission Required" buttons {"Grant Permission", "Quit"} default button "Grant Permission" with icon caution
    `, strings.ReplaceAll(message, `"`, `\"`))

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()

	if err != nil {
		system.Warn("User declined permission or dialog failed")
		return fmt.Errorf("permission request declined")
	}

	// Check which button was clicked
	if strings.Contains(string(output), "Grant Permission") {
		system.Info("User chose to grant permission")
		return nil
	}

	return fmt.Errorf("user chose to quit")
}

// ShowRestoreOptionsMenu shows interactive restore options (for checkpoint selection)
func (nm *NotificationManager) ShowRestoreOptionsMenu(checkpoints []string) (int, error) {
	if !nm.isInteractive {
		return 0, fmt.Errorf("interactive mode disabled")
	}

	system.Info("Showing restore options menu")

	// Build checkpoint list for dialog
	checkpointList := strings.Join(checkpoints, "\\n")

	message := fmt.Sprintf(
		"Available Checkpoints:\\n\\n%s\\n\\nEnter checkpoint number to restore:",
		checkpointList,
	)

	script := fmt.Sprintf(`
        set response to text returned of (display dialog "%s" with title "Select Checkpoint" default answer "1" buttons {"Restore", "Cancel"} default button "Restore")
        return response
    `, message)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		system.Debug("User cancelled checkpoint selection")
		return -1, fmt.Errorf("user cancelled")
	}

	// Parse selected checkpoint number
	selectedStr := strings.TrimSpace(string(output))
	var selected int
	if _, err := fmt.Sscanf(selectedStr, "%d", &selected); err != nil {
		return -1, fmt.Errorf("invalid selection: %s", selectedStr)
	}

	system.Info("User selected checkpoint:", selected)
	return selected, nil
}
