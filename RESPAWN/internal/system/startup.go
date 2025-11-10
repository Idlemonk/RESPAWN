package system

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"RESPAWN/internal/types"
	"RESPAWN/pkg/config"
)

// StartupManager handles application lifecycle and auto-start
type StartupManager struct {
    autoStart      *MacOSAutoStart
    instanceLock   *InstanceLock
    crashTracker   *CrashTracker
    baseDir        string
    executablePath string
}

// InstanceLock ensures single instance of RESPAWN
type InstanceLock struct {
    lockFile string
    pidFile  string
    pid      int
}

// CrashTracker monitors crash patterns
type CrashTracker struct {
    crashes      []time.Time
    maxCrashes   int
    windowPeriod time.Duration
    isDisabled   bool
    stateFile    string
}

// RestartPolicy defines restart behavior
type RestartPolicy struct {
    MaxRetries      int
    BackoffIntervals []time.Duration
    CurrentRetry    int
    LastCrashTime   time.Time
}

//NewStartupManager creates a new startup manager 
func NewStartupManager() (*StartupManager, error) {
    // Get the executable path
    execPath, err := os.Executable()
    if err != nil {
        return nil, fmt.Errorf("failed to get executable path: %w", err)
    }
    
    // Get the base directory (where the executable lives)
    baseDir := filepath.Dir(execPath)

    // create macOS auto-start manager
    autoStart := NewMacOSAutoStart(execPath)

    // Initialize instance lock
    instanceLock := &InstanceLock{
        lockFile: filepath.Join(baseDir, "respawn.lock"),
        pidFile:  filepath.Join(baseDir, "respawn.pid"),
        pid:      os.Getpid(),
    }

    // Initialize crash tracker
    crashTracker := &CrashTracker{
        crashes:      make([]time.Time, 0),
        maxCrashes:   3, // Disable after 3 crashes
        windowPeriod: 1 * time.Hour,
        stateFile:    filepath.Join(baseDir, "crash_state.json"),	
    }

    if err := crashTracker.Load(); err != nil {
        Debug("No previous crash state found, starting fresh")
    }

    sm := &StartupManager{
        autoStart:      autoStart,
        instanceLock:   instanceLock,
        crashTracker:   crashTracker,
        baseDir:        baseDir,
        executablePath: execPath,
    }

    return sm, nil 
}

// EnsureSingleInstance checks if another instance is running
func (sm *StartupManager) EnsureSingleInstance() error {
	Debug("Checking for existing RESPAWN instance")

	// Check if lock file exists
	if _, err := os.Stat(sm.instanceLock.lockFile); err == nil {
	// Lock file exists, check if process is still running
	pidData, err := os.ReadFile(sm.instanceLock.pidFile)
		if err == nil {
			oldPID, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
			if err == nil && sm.isProcessRunning(oldPID) {
				return fmt.Errorf("RESPAWN is already running (PID: %d)", oldPID)
			}
		}

		// Stale lock file, remove it
		Debug("Removing stale lock file")
		os.Remove(sm.instanceLock.lockFile)
		os.Remove(sm.instanceLock.pidFile)
	}

	// Create Lock file
	if err := os.WriteFile(sm.instanceLock.lockFile, []byte(fmt.Sprintf("%d", sm.instanceLock.pid)), 0644); err != nil {
		return fmt.Errorf("Failed to create lock file: %w", err)
	}

	// Create PID file
	if err := os.WriteFile(sm.instanceLock.pidFile, []byte(fmt.Sprintf("%d", sm.instanceLock.pid)), 0644); err != nil {
		return fmt.Errorf("Failed to create PID file: %w", err)
	}

	Info("Single instance lock acquired")
	return nil 
}

// Install sets up auto-start for RESPAWN
func (sm *StartupManager) Install() error {
	Info("Installing RESPAWN auto-start for macOS")

	// Check if already installed
	if sm.autoStart.IsInstalled() {
		Info("RESPAWN auto-start already installed")
		return nil 
	}

	// Install auto-start
	if err := sm.autoStart.Install(); err != nil {
		return fmt.Errorf("Failed to install auto-start: %w", err)
	}

	// Enable auto-start
	if err := sm.autoStart.Enable(); err != nil {
		return fmt.Errorf("Failed to enable auto-start: %w", err)
	}

	Info("RESPAWN auto-start installed successfully")
	fmt.Println("✅ RESPAWN auto-start configured")
    fmt.Println("✅ Will start automatically on system login")
    fmt.Println("✅ Target startup time: 7-8 seconds")
    
    return nil
}

// Uninstall removes auto-start for RESPAWN
func (sm *StartupManager) Uninstall() error {
	Info("Uninstalling RESPAWN auto-start")

	if !sm.autoStart.IsInstalled() {
		Info("RESPAWN auto-start not installed")
		return nil
	}

	if err := sm.autoStart.Uninstall(); err != nil {
		return fmt.Errorf("Failed to uninstall auto-start: %w", &err)
	}

	Info("RESPAWN auto-start uninstalled successfully")
	fmt.Println("✅ RESPAWN auto-start removed")

	return nil 
}

// EnableAutoStart enables automatic startup
func (sm *StartupManager) EnableAutoStart() error {
	Info("Enabling RESPAWN auto-start")

	if !sm.autoStart.IsInstalled() {
		return fmt.Errorf("auto-start not installed, run: respawn --install")
	}

	if err := sm.autoStart.Enable(); err != nil {
		return fmt.Errorf("Failed to enable auto-start: %w", err)
	}

	// Reset crash tracker
	sm.crashTracker.isDisabled = false
	sm.crashTracker.Save()

	Info("RESPAWN auto-start enabled")
	fmt.Println("✅ Auto-start enabled ")

	return nil 
}

//DisableAutoStart disables automatic startup
func (sm *StartupManager) DisableAutoStart() error {
	Info("Disabling RESPAWN auto-start")

	if !sm.autoStart.IsInstalled() {
		return fmt.Errorf("auto-start not installed")
	}

	if err := sm.autoStart.Disable(); err != nil {
		return fmt.Errorf("Failed to disable auto-start: %w", &err)
	}

	Info("RESPAWN auto-start disabled")
	fmt.Println("✅ Auto-start disabled ")

	return nil
}

// IsEnabled returns whether auto-start is currently enabled
func (sm *StartupManager) IsEnabled() bool {
    if sm.autoStart == nil {
        return false
    }
    return sm.autoStart.IsEnabled()
}

// StartWithPolicy starts RESPAWN with restart policy  
func (sm *StartupManager) StartWithPolicy() error {
	startTime := time.Now()
	Info("Starting RESPAWN with restart policy")

	// Check crash history
	if sm.crashTracker.ShouldDisableAutoStart() {
		Warn("RESPAWN has crashed too many times, auto-start disabled")
		sm.showCrashNotification()
		return fmt.Errorf("auto-start disabled due to repeated crashes")
	}

	// Ensure single instance
	if err := sm.EnsureSingleInstance(); err != nil {
		return err 
	}

	//Initialize with timeout (7-8 seconds target)
	initTimeout := 8 * time.Second
	initChan := make(chan error, 1)

	go func ()  {
		initChan <- sm.initialize()
	}()

	select {
	case err := <-initChan:
		if err != nil {
			Error("Initialization failed:", err)
			sm.recordCrash()
			sm.showErrorDialog("RESPAWN Initialization Failed", err.Error())
			return err 
		}
	case <-time.After(initTimeout):
		Error("Initialization timeout exceeded")
		return fmt.Errorf("initialization timeout (>8 seconds)")
	}

	duration := time.Since(startTime)
	Info("RESPAWN started successfully in", duration)

	if duration.Seconds() > 8 {
		Warn("Startup time exceeded target:", duration)
	} else if duration.Seconds() <= 7 {
		Info("Startup time excellent:", duration)
	}
	return nil
}

// initialize performs RESPAWN initialization
func (sm *StartupManager) initialize() error {
	Debug("Initializing RESPAWN components")

	// Load configuration
	if err := config.LoadConfig(); err != nil {
		return fmt.Errorf("Failed to load configuration: %w", err)
	}
	
	// Check permissions
	if err := sm.checkMacOSPermissions(); err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	// Initialize logger
	if err := InitLogger(); err != nil {
		return fmt.Errorf("Failed to initialize logger: %w", err)
	}

	Info("RESPAWN components initialized successfully")
	return nil
}

// checkMacOSPermissions checks macOS-specific permissions
func (sm *StartupManager) checkMacOSPermissions() error {
	Debug("Checking macOS permissions")

	// Check Accessibility permission (CRITICAL)
	hasAccessibility := sm.hasAccessibilityPermission()
	if !hasAccessibility {
		Warn("Accessibility permission not granted")
		sm.showPermissionDialog(
			"Accessibility Access Required",
			"RESPAWN needs Accessibility access to detect window states. \n\n"+
				"Please grant permission in:\nSystem Preferences -> Security & Privacy -> Privacy -> Accessibility",
		)
		return fmt.Errorf("Accessibility permission required")
	}

	Info("Accessibility permission granted")

	// Check full Disk Access (OPTIONAL)
	hasFullDisk := sm.hasFullDiskAccess()
	if !hasFullDisk {
		Debug("Full Disk Access not granted - deep integration disabled")
		Debug("Basic functionality will work, deep app integration unavailable")
	} else {
		Info("Full Disk Access granted - deep integration enabled")
	}

	return nil
}

//hasAccessibilityPermission checks if accessibility permission is granted
func (sm *StartupManager) hasAccessibilityPermission() bool {
	// Use AppleScript to check accessibility permission
	script := `
        tell application "System Events"
            try
                set x to every window
                return true
            on error
                return false
            end try
        end tell
    `

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}

// hasFullDiskAccess checks if Full Disk Access is granted
func (sm *StartupManager) hasFullDiskAccess() bool {
	// Try to access a protected location
	testPath := filepath.Join(os.Getenv("HOME"), "Library/Safari/Bookmarks.plist")
	_, err := os.Stat(testPath)
	return err == nil 
}

//recordCrash records a crash event
func (sm *StartupManager) recordCrash() {
	sm.crashTracker.RecordCrash()

	if sm.crashTracker.ShouldDisableAutoStart() {
		Error("Crash threshold exceeded (3 crashes in 1 hour) - disabling auto-start")
		sm.DisableAutoStart()
	}
}

//RestartWithBackoff restarts RESPAWN with exponential backoff
func (sm *StartupManager) RestartWithBackoff(policy *types.RestartPolicy) error {
	if policy.CurrentRetry >= policy.MaxRetries {
		Error("Max restart retries exceeded")
		return fmt.Errorf("max restart retries (%d) exceeded", policy.MaxRetries)
	}

	// Get backoff interval
	backoffIndex := policy.CurrentRetry
	if backoffIndex >= len(policy.BackoffIntervals) {
		backoffIndex = len(policy.BackoffIntervals) - 1
	}

	backoff := policy.BackoffIntervals[backoffIndex]

	Info("Restarting RESPAWN after", backoff, "(attempt", policy.CurrentRetry+1, "of", policy.MaxRetries,")")
	time.Sleep(backoff)

	policy.CurrentRetry++
	policy.LastCrashTime = time.Now()

	// Attempt restart
	cmd := exec.Command(sm.executablePath, "--start")
	if err := cmd.Start(); err != nil {
		Error ("Failed to restart RESPAWN:", err)
		return sm.RestartWithBackoff(policy)
	}

	Info("RESPAWN restart initiated successfully")
	return nil
}

// GetDefaultRestartPolicy returns the default restart policy
func GetDefaultRestartPolicy() *types.RestartPolicy {
	return &types.RestartPolicy{
		MaxRetries: 3,
		BackoffIntervals: []time.Duration{
			5 * time.Second,  // First retry: 5 seconds 
			10 * time.Second, // Second retry: 10 seconds
			30 * time.Second, // Third retry: 30 seconds
		},
		CurrentRetry: 0,
	}
}

// CrashTracker methods

// RecordCrash records a new crash
func (ct *CrashTracker) RecordCrash() {
	now := time.Now()
	ct.crashes = append(ct.crashes, now)

	// Remove crashes outside the window period
	ct.cleanOldCrashes()

	ct.Save()

	Warn("Crash recorded. Total crashes in last hour:", len(ct.crashes))
}

// ShouldDisableAutoStart checks if auto-start should be disabled 
func (ct *CrashTracker) ShouldDisableAutoStart() bool {
	if ct.isDisabled {
		return true
	}

	ct.cleanOldCrashes()

	if len(ct.crashes) >= ct.maxCrashes {
		ct.isDisabled = true
		ct.Save()
		Error("Crash threshold reached:", len(ct.crashes), "crashes in last hour")
		return true
	}

	return false
}

// cleanOldCrashes removes crashes outside the window period
func (ct *CrashTracker) cleanOldCrashes() {
	cutoff := time.Now().Add(-ct.windowPeriod)
	validCrashes := make([]time.Time, 0)

	for _, crashTime := range ct.crashes {
		if crashTime.After(cutoff) {
			validCrashes = append(validCrashes, crashTime)
		}
	}

	ct.crashes = validCrashes
}

// Save saves crash tracker state
func (ct *CrashTracker) Save() error {
	data, err := json.MarshalIndent(ct, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(ct.stateFile, data, 0644)
}

// Load loads crash tracker state
func (ct *CrashTracker) Load() error {
	data, err := os.ReadFile(ct.stateFile)
	if err != nil {
		return err 
	}
	return json.Unmarshal(data, ct)
}

// Helper methods

// isProcessRunning checks if a process with given PID is running
func (sm *StartupManager) isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, sending signal 0 checks if process exists
	err = process.Signal(os.Signal(nil))
	return err == nil 
}

//showPermissionDialog shows a permission 
func (sm *StartupManager) showPermissionDialog(title, message string) {
	script := fmt.Sprintf(`
        display dialog "%s" with title "%s" buttons {"OK"} default button "OK" with icon caution
    `, strings.ReplaceAll(message, `"`, `\"`), title)

	cmd:= exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		Warn("Failed to show permission dialog:", err)
	}
}

// showErrorDialog shows an error dialog and exits
func (sm *StartupManager) showErrorDialog(title, message string) {
	Error(title, ":", message)

	script := fmt.Sprintf(`
        display dialog "%s" with title "%s" buttons {"OK"} default button "OK" with icon stop
    `, strings.ReplaceAll(message, `"`, `\"`), title)

	cmd := exec.Command("osascript", "-e", script)
	cmd.Run()
}

// showCrashNotification shows crash notification to user
func (sm *StartupManager) showCrashNotification() {
	message := fmt.Sprintf(
        "RESPAWN has crashed %d times in the last hour.\n\n"+
            "Auto-start has been disabled for safety.\n\n"+
            "To re-enable:\nOpen Terminal and run: respawn --enable-autostart",
        sm.crashTracker.maxCrashes,
    )

	sm.showPermissionDialog("RESPAWN Auto-start disabled", message)
}

// ReleaseLock releases the instance lock
func (sm *StartupManager) ReleaseLock() {
	Debug("Releasing instance lock")
	os.Remove(sm.instanceLock.lockFile)
	os.Remove(sm.instanceLock.pidFile)
}

// Cleanup performs cleanup on shutdown
func (sm *StartupManager) Cleanup() {
	Info("Performing startup manager cleanup")
	sm.ReleaseLock()
}

