package process

import (
	"fmt"
	"os/exec"
	"time"

	"RESPAWN/internal/system"
	"RESPAWN/internal/types"
	"RESPAWN/pkg/config"	

)



type ApplicationLauncher struct {
	detector *ProcessDetector
	results  []types.LaunchResult
}

// NewApplicationLauncher creates a new application launcher
func NewApplicationLauncher()  *ApplicationLauncher {
	return &ApplicationLauncher{
		detector: NewProcessDetector(),
		results: make([]types.LaunchResult, 0),
	}
}

// RestoreApplications launches applications in memory order with full state restoration
func (al *ApplicationLauncher) RestoreApplications(processes []types.ProcessInfo) ([]types.LaunchResult, error) {
	system.Info("Starting application restoration")

	// Sort by memory usage (highest first)
	sortedProcesses := SortByMemoryUsage(processes)

	for _, proc := range sortedProcesses {
		// Check if app is already running
		if al.isApplicationRunning(proc.ProcessName) {
			system.Debug("Skipping", proc.Name, "- already running")
			continue
		}

		// Launch application with retry logic
		result := al.launchWithRetry(proc)
		al.results = append(al.results, result)

		if result.Success {
			// Restore window state immediately after successful launch
			al.restoreWindowState(proc, result.PID)

			// Show success notification
			al.showSuccessNotification(proc.Name)

			// Wait a bit before launching the next app to avoid overload
			time.Sleep(time.Duration(config.GlobalConfig.LaunchDelayMs) * time.Millisecond)
		}
	}

	system.Info("Application restoration completed")
	return al.results, nil
}

// launchWithRetry attempts to launch an application with retry logic
func (al *ApplicationLauncher) launchWithRetry(proc types.ProcessInfo) types.LaunchResult {
	maxRetries := config.GlobalConfig.MaxRetryAttempts

	for attempt := 1; attempt <= maxRetries; attempt++ {
		system.Debug("Launching", proc.Name, "- attempt", attempt)

		result := al.launchApplication(proc)
		result.RetryCount = attempt

		if result.Success {
			system.Info("Successfully launched", proc.Name, "on attempt", attempt)
			return result
		}

		system.Warn("Failed to launch", proc.Name, "on attempt", attempt, ":", result.ErrorMsg)

		if attempt < maxRetries {
			time.Sleep(1 * time.Second) // Wait before retrying
		} 
	}

	// All Retries Attempt Failed
	system.Error("Failed to launch", proc.Name, "after", maxRetries, "attempts")
	return types.LaunchResult{
		AppName: proc.Name,
		Success: false,
		LaunchTime: time.Now(),
		RetryCount: maxRetries,
		ErrorMsg: fmt.Sprintf("Failed after %d attempts", maxRetries),
	}
}

// launchApplication launches a single application
func (al *ApplicationLauncher) launchApplication(proc types.ProcessInfo) types.LaunchResult {
	startTime  := time.Now()

	// Use 'open -a' command for fast, reliable launching
	cmd := exec.Command("open", "-a", proc.ProcessName)

	err := cmd.Start()
	if err != nil {
		return types.LaunchResult{
			AppName: proc.Name,
			Success: false,
			LaunchTime: startTime,
			ErrorMsg: fmt.Sprintf("Failed to start process: %v", err),
		}
	}
	// Wait for the command to complete
	err = cmd.Wait()
	if err != nil {
		return types.LaunchResult{
			AppName: proc.Name,
			Success: false,
			LaunchTime: startTime,
			ErrorMsg: fmt.Sprintf("Process execution failed: %v", err),
		}
	}
	// Wait a moment for the process to fully initialize
	time.Sleep(500 * time.Millisecond)

	// Verify the application actually started
	pid, isRunning := al.verifyApplicationLaunched(proc.ProcessName)
	if !isRunning {
		return types.LaunchResult{
			AppName: proc.Name,
			Success: false,
			LaunchTime: startTime,
			ErrorMsg: "Process Not Found After Launch",
		}
	}


	return types.LaunchResult{
		AppName: proc.Name,
		Success: true,		
		PID: 	 pid,	
		LaunchTime: startTime,
	}
}

// verifyApplicationLaunched checks if the application is actuallyy running
func (al *ApplicationLauncher) verifyApplicationLaunched(processName string) (int, bool) {
	cmd := exec.Command("pgrep", "-f", processName)
	output, err := cmd.Output()	

	if err != nil {
		return 0, false
	}

	if len(output) > 0 {
		// Parse PID from pgrep output
		pidStr := string(output[:len(output)-1]) // Remove newline
		if len(pidStr) > 0 {
			// For simplicity, we'll just return that it's running
			// In a more robust implementation, you'd parse the actual PID
			return 1, true
		}
	}
	return 0, false
}

// isApplicationRunning checks if an application is currently running
func (al *ApplicationLauncher) isApplicationRunning(processName string) bool {
	_, isRunning := al.verifyApplicationLaunched(processName)
	return isRunning			
}


// restoreWindowState restores the window state for a launched application
func (al *ApplicationLauncher) restoreWindowState(proc types.ProcessInfo, pid int) {
	system.Debug("Restoring window state for", proc.Name, "to", proc.WindowState)

	var script string

	switch proc.WindowState {
	case "minimized":
		script = fmt.Sprintf(`
            tell application "System Events"
                tell application process "%s"
                    if exists window 1 then
                        set minimized of window 1 to true
                    end if
                end tell
            end tell
        `, proc.ProcessName)

	case "maximized":
		script = fmt.Sprintf(`
            tell application "System Events"
                tell application process "%s"
                    if exists window 1 then
                        set zoomed of window 1 to true
                    end if
                end tell
            end tell
        `, proc.ProcessName)

	case "normal":
		// For normal windows, we do not need to do anything special
		// The application should open in it's default state
		system.Debug("Window state is normal, no restoration needed for", proc.Name)
		return
	}

	if script != "" {
		cmd := exec.Command("osascript", "-e", script)
		err := cmd.Run()
		if err != nil {
			system.Warn("Failed to restore window state for", proc.Name, ":", err)
		} else {
			system.Debug("Successfully restored window state for", proc.Name)
		}
	}
}

// showSuccessNotification displays the success indicator
func (al *ApplicationLauncher) showSuccessNotification(appName string) {
	// Print to stdout so user sees it immediately
	fmt.Printf("%s ✅\n", appName)

	// Log the success
	system.Info("Application resrored:", appName)

	// Waits for 2 seconds 
	time.Sleep(2 * time.Second)
}

// GetFailedApplications returns applications that failed to launch
func (al *ApplicationLauncher) GetFailedApplications() []types.LaunchResult {
	var failed []types.LaunchResult
	for _, result := range al.results {
		if !result.Success {
			failed = append(failed, result)
		}
	}
	return failed
}

// GetSuccessfulApplications returns application that launched successfully
func (al *ApplicationLauncher) GetSuccessfulApplications() []types.LaunchResult {
	var successful []types.LaunchResult
	for _, result := range al.results {
		if result.Success {
			successful = append(successful, result)
		}
	}
	return successful
}

// GetLaunchSummary returns a summary of the launch operation
func (al *ApplicationLauncher) GetLaunchSummary() (int, int, []string) {
	successful := 0
	failed := 0
	var failedApps []string

	for _, result := range al.results {
		if result.Success {
			successful++
		} else {
			failed++
			failedApps = append(failedApps, result.AppName)
		}
	}

	return successful, failed, failedApps
}
