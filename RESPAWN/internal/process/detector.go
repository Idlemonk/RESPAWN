package process

import (

	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"RESPAWN/internal/system"
	"RESPAWN/pkg/config"
	
)

type ProcessInfo struct {
	PID 		int 	`json:"pid"`
	Name        string `json:"name"`
    ProcessName string `json:"process_name"`
    MemoryMB    int64  `json:"memory_mb"`
    WindowState string `json:"window_state"` // "normal", "minimized", "maximized"
    IsRunning   bool   `json:"is_running"`

}

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
func (pd *ProcessDetector) DetectRunningProcesses() ([]ProcessInfo, error) {
    system.Debug("Starting process detection")

	var runningProcesses []ProcessInfo


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

// getProcessInfo gets detailed information about a specific application
func (pd *ProcessDetector) getProcessInfo(app config.AppConfig) (ProcessInfo, error) {
	ProcessInfo := ProcessInfo{
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

func SortByMemoryUsage(processes []ProcessInfo) []ProcessInfo {
	// Simple bubble sort for demonstartion purposes. (one could use sort.Slice for better performance)
	sorted := make([]ProcessInfo, len(processes))
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

