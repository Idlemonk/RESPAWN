package main 
import (
	"fmt"
	"os"
	"RESPAWN/internal/system"
	"RESPAWN/pkg/config"
	"RESPAWN/internal/process"

)

func main() {
	// Initialize the logger
	if err := system.InitLogger(); err != nil {
		fmt.Printf("Failed to iitialize logger: %v\n", err)
		os.Exit(1) 
	}
	defer system.Close()

	// Test the logging system
	system.Debug("This is a debug message - testing logging system")
    system.Info("Logger initialized successfully")

	// Add config loading
	if err := config.LoadConfig(); err != nil {
		system.Error("Failed to load config:", err)
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}
	system.Info("Config lodaded successfully")

	// Test process detection
	detector := process.NewProcessDetector()
	runningProcesses, err := detector.DetectRunningProcesses()
	if err != nil {
		system.Error("Failed to detect running processes:", err)
		fmt.Printf("Failed to detect running processes: %v\n", err)
		os.Exit(1)
	}

	// Display results
	fmt.Printf("\n=== RUNNING PROCESSES ===\n")
    fmt.Printf("Found %d running applications:\n\n", len(runningProcesses))
	
	if len(runningProcesses) > 0 {
		// Sort by memory usage
		sortedProcesses := process.SortByMemoryUsage(runningProcesses)

		for i, proc := range sortedProcesses {
			fmt.Printf("%d. %s (PID: %d)\n", i+1, proc.Name, proc.PID)
			fmt.Printf("   Memory: %d MB\n", proc.MemoryMB)
			fmt.Printf("   Window State: %s\n\n", proc.WindowState)
		}
	} else {
		fmt.Println("No target applications are currently running. ")
		fmt.Println("Try opening Chrome, Safari, or any of the configured apps and run again. ")
	}

	system.Info("Process detection test completed")

	// Add config testing
	system.Debug("Enabled applications:", len(config.GlobalConfig.GetEnabledApplications()))
	fmt.Printf("Config loaded successfully!\n")
	fmt.Printf("Data directory: %s\n", config.GlobalConfig.DataDir)
    system.Warn("This is a warning message test")
    system.Error("This is an error message test (not a real error)")
    fmt.Println("Logging test completed. Check ~/.respawn/logs/respawn.log")

}
