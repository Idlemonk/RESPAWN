package process

import (

	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"RESPAWN/internal/system"
	"RESPAWN/internal/types"
	"RESPAWN/pkg/config"
	
)

type ProcessDetector struct {
	enabledApps []config.AppConfig
}

// NewProcessDetector creates a new process detector
func NewProcessDetector() *ProcessDetector {
	return &ProcessDetector{
		enabledApps: config.GlobalConfig.GetEnabledApplications(),
	}
}

// DetectRunningProcesses finds all enabled applications that are currently running
func (pd *ProcessDetector) DetectRunningProcesses() ([]types.ProcessInfo, error) {
    system.Debug("Starting process detection")

	var runningProcesses []types.ProcessInfo


	for _, app := range pd.enabledApps {
		processInfo, err := pd.getProcessInfo(app)
		if err != nil {
			system.Warn("Failed to get process info for", app.Name, ":", err)
			continue
		}

		if processInfo.IsRunning {
			runningProcesses = append(runningProcesses, processInfo)
			system.Debug("Found running process:", app.Name, "PID:", processInfo.PID, "Memory:", processInfo.MemoryMB, "MB")
		} 
	}
	system.Info("Detected", len(runningProcesses), "running processes")
	return runningProcesses, nil 
}

// GetRunningApplications returns list of all running GUI applications
<<<<<<< HEAD
func (pd *ProcessDetector) GetRunningApplications() ([]types.ApplicationInfo, error) {
=======
func (pd *ProcessDetector) GetRunningApplications() ([]ApplicationInfo, error) {
>>>>>>> 0dcb4dc2d24cd8117198084eb96667f1b9bfff7b
    // Use AppleScript to get running applications
    script := `
        tell application "System Events"
            set appList to name of every application process whose background only is false
            return appList
        end tell
    `
    
    cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(" Failed to get applications: %w", err)
	}

	// Parse output
	appNames := strings.Split(strings.TrimSpace(string(output)), ", ")

<<<<<<< HEAD
	var apps []types.ApplicationInfo
=======
	var apps []ApplicationInfo
>>>>>>> 0dcb4dc2d24cd8117198084eb96667f1b9bfff7b
	for _, name := range appNames {
		// Skip system Apps
		if isSystemApp(name) {
			continue
		}

		appInfo, err := pd.getApplicationInfo(name)
		if err != nil {
			continue // Skip apps we can't get info for
		}

		apps = append(apps, appInfo)
	}

	return apps, nil
}

// getProcessInfo gets detailed information about a specific application
func (pd *ProcessDetector) getProcessInfo(app config.AppConfig) (types.ProcessInfo, error) {
	ProcessInfo := types.ProcessInfo{
		Name:        app.Name,
		ProcessName: app.ProcessName,
		IsRunning:   false,
	}

	// Use macOS 'ps' command to find process 
	cmd := exec.Command("ps", "axo", "pid,comm,rss", "-c")
	output, err := cmd.Output()
	if err != nil {
		return ProcessInfo, fmt.Errorf("failed to execute ps command: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] { // Skip header line
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		processName := fields[1]
		if processName == app.ProcessName {
			// Parse PID
			pid, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}

			// Parse memory (RSS is in kb on macOS, convert to MB)
			rssKB, err := strconv.ParseInt(fields[2], 10, 64)
			if err != nil {
				continue
			}
			memoryMB := rssKB / 1024

			ProcessInfo.PID = pid 
			ProcessInfo.MemoryMB = memoryMB 
			ProcessInfo.IsRunning = true 

			// get window state (simplified for now)
			windowState, err := pd.getWindowState(pid)
			if err != nil {
				system.Debug("Could not get window state for", app.Name, ":", err)
				windowState = "normal" // default
			}
			ProcessInfo.WindowState = windowState 

			break
		}
	}

	return ProcessInfo, nil 
}

// getWindowState determines if the application window is minimized, maximized, or normal
func (pd *ProcessDetector) getWindowState(pid int) (string, error) {
	// Use AppleScript to check window state.
	script := fmt.Sprintf(`
	tell application "Sysytem Events"
            set appName to name of first application process whose unix id is %d
            tell application process appName
                if exists window 1 then
                    set windowProps to properties of window 1
                    return windowProps as string
                else
                    return "no_window"
                end if
            end tell
        end tell
    `, pid)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "normal", err
	}

	outputStr := strings.TrimSpace(string(output))

	// Simple parsing - in real implementation you'd parse the properties more carefully
	if strings.Contains(outputStr, "minimized:true") {
		return "minimized", nil
	} else if strings.Contains(outputStr, "zoomed:true") {
		return "maximized", nil
	}

	return "normal", nil
}

// getApplicationInfo gets detailed info for an application
func (pd *ProcessDetector) getApplicationInfo(appName string) (types.ApplicationInfo, error) {
	var info types.ApplicationInfo

	// get bundle ID
	script := fmt.Sprintf(`
        tell application "System Events"
            set appProcess to first application process whose name is "%s"
            set bundleID to bundle identifier of appProcess
            return bundleID
        end tell
    `, appName)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return info, err 
	}

	info.Name = appName	
	info.BundleID = strings.TrimSpace(string(output))
	info.ExecutablePath = fmt.Sprintf("/Applications/%s.app", appName)

	// Get window information
	windows, err := pd.getWindowInfo(appName)
	if err == nil {
		info.Windows = windowsFromAppleScript
	}

	return info, nil
}

// getWindowInfo gets window positions for an application
func (pd *ProcessDetector) getWindowInfo(appName string) ([]types.WindowInfo, error) {
	script := fmt.Sprintf(`
        tell application "System Events"
            tell process "%s"
                set windowList to {}
                repeat with w in windows
                    set windowInfo to {name of w, position of w, size of w}
                    set end of windowList to windowInfo
                end repeat
                return windowList
            end tell
        end tell
    `, appName)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse window data (simplified)
	// TODO: Proper parsing of AppleScript output
	outputStr := strings.TrimSpace(string(output))
	var windows []types.WindowInfo

	// Example simple parsing: Split by app-specific delimiters (e.g., assume output like "window1:{x,y},size{w,h}; ...")
	// For now, return empty if not parsable-expand as needed
	if !strings.Contains(outputStr, "no windows") {  // Basic check like getWindowState
		// Placeholder: Add real split/logic here, e.g., strings.Split(outputStr, ";")
		// windows = append(windows, types.WindowInfo{Title: "Example", ...})  // Stub for testing
	}

	return window, nil
}

// isSystemApp checks if app should be excluded
func isSystemApp(appName string) bool {
    systemApps := []string{
        "Finder",
        "Dock",
        "SystemUIServer",
        "loginwindow",
        "NotificationCenter",
    }
    
    for _, sys := range systemApps {
        if appName == sys {
            return true
        }
    }
    
    return false
}

func SortByMemoryUsage(processes []types.ProcessInfo) []types.ProcessInfo {
	// Simple bubble sort for demonstration purposes. (one could use sort.Slice for better performance)
	sorted := make([]types.ProcessInfo, len(processes))
	copy(sorted, processes)

	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].MemoryMB < sorted[j+1].MemoryMB {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	return sorted
}

