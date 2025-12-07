

package main

import (
	"fmt"
	"os"
	"os/exec"
    "os/signal"
    "syscall"
    "strconv"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

    "RESPAWN/internal/checkpoint"
	"RESPAWN/internal/process"
	"RESPAWN/internal/system"
    "RESPAWN/internal/types"
	"RESPAWN/internal/ui"
	"RESPAWN/pkg/config"
)


const (
	Version = "v1.0.0-beta"
	Copyright = "© 2024 NINSCO GLOBAL RESOURCES LTD. All rights reserved."
	Website =  "https://github.com/ninsco/respawn"
	SupportMail  = "verifiedbusinessmail@gmail.com" 
)

//RESPAWNApp holds all application components
type RESPAWNApp struct {
	startupManager      *system.StartupManager
    monitor            *system.SystemMonitor
    checkpointManager  *checkpoint.CheckpointManager
    notificationManager *ui.NotificationManager
    launcher           *process.ApplicationLauncher
    detector           *process.ProcessDetector
    
    startTime          time.Time
    lastCheckpointTime time.Time
    isRunning          bool
}

var (
    app *RESPAWNApp
    
    // Command flags
    silentMode   bool
    forceMode    bool
    checkpointID string
)

// Root command
var rootCmd = &cobra.Command{
    Use:     "respawn",
    Short:   "RESPAWN - Automatic workspace restoration",
    Long:    buildWelcomeMessage(),
    Version: Version,
}

// Install command
var installCmd = &cobra.Command{
    Use:   "install",
    Short: "Install RESPAWN auto-start",
    Long:  "Sets up RESPAWN to start automatically on system login",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleInstall(); err != nil {
            fmt.Printf("❌ Installation failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Uninstall command
var uninstallCmd = &cobra.Command{
    Use:   "uninstall",
    Short: "Uninstall RESPAWN auto-start",
    Long:  "Removes RESPAWN from auto-start",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleUninstall(); err != nil {
            fmt.Printf("❌ Uninstall failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Start command
var startCmd = &cobra.Command{
    Use:   "start",
    Short: "Start RESPAWN monitoring",
    Long:  "Starts RESPAWN in background monitoring mode",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleStart(); err != nil {
            fmt.Printf("❌ Start failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Restore command
var restoreCmd = &cobra.Command{
    Use:   "restore",
    Short: "Restore workspace from checkpoint",
    Long:  "Restores applications from the latest or specified checkpoint",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleRestore(); err != nil {
            fmt.Printf("❌ Restore failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Checkpoint command
var checkpointCmd = &cobra.Command{
    Use:   "checkpoint",
    Short: "Create immediate checkpoint",
    Long:  "Forces creation of a checkpoint now",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleCheckpoint(); err != nil {
            fmt.Printf("❌ Checkpoint failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Status command
var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show RESPAWN status",
    Long:  "Displays current RESPAWN status and statistics",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleStatus(); err != nil {
            fmt.Printf("❌ Status check failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Enable auto-start command
var enableCmd = &cobra.Command{
    Use:   "enable-autostart",
    Short: "Enable auto-start",
    Long:  "Re-enables RESPAWN auto-start on system login",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleEnableAutoStart(); err != nil {
            fmt.Printf("❌ Enable failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Disable auto-start command
var disableCmd = &cobra.Command{
    Use:   "disable-autostart",
    Short: "Disable auto-start",
    Long:  "Disables RESPAWN auto-start without uninstalling",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleDisableAutoStart(); err != nil {
            fmt.Printf("❌ Disable failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Pause command
var pauseCmd = &cobra.Command{
    Use:   "pause",
    Short: "Pause monitoring",
    Long:  "Temporarily pauses checkpoint creation",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handlePause(); err != nil {
            fmt.Printf("❌ Pause failed: %v\n", err)
            os.Exit(1)
        }
    },
}

// Resume command
var resumeCmd = &cobra.Command{
    Use:   "resume",
    Short: "Resume monitoring",
    Long:  "Resumes checkpoint creation after pause",
    Run: func(cmd *cobra.Command, args []string) {
        if err := handleResume(); err != nil {
            fmt.Printf("❌ Resume failed: %v\n", err)
            os.Exit(1)
        }
    },
}

func init() {
	// Add flags to restore command
	restoreCmd.Flags().BoolVarP(&silentMode, "silent", "s", false, "Restore silently without progress display")
	restoreCmd.Flags().StringVarP(&checkpointID, "checkpoint", "c", "", "Restore from specific checkpoint ID")

	// Add flags to checkpoint command 
	checkpointCmd.Flags().BoolVarP(&forceMode, "force", "f", false, "Force checkpoint even under high CPU/low battery")



	// Add all commands to root
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(checkpointCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(resumeCmd)
}


func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// buildWelcomeMessage creates the welcome/help message
func buildWelcomeMessage() string {
    return fmt.Sprintf(`
┌─────────────────────────────────────┐
│         Welcome to RESPAWN          │
│            By NINSCO                │
│                                     │
│   Automatic workspace restoration   │
│   Simple. Powerful. Invisible.      │
│                                     │
│   %s                  │
│   %s     │
└─────────────────────────────────────┘

Website: %s
Support: %s

RESPAWN automatically saves your workspace every hour
and restores it after system restarts or crashes.
`, Version, Copyright, Website, SupportMail)
}

// initializeComponents starts all RESPAWN components in correct order
func initializeComponents() error {
    system.Info("Initializing RESPAWN components...")
    initStart := time.Now()

    // Phase 1: Logger (already initialized by system.Info call above)
    system.Debug("Logger initialized ✓")

    // Phase 2: Configuration
    if err := config.LoadConfig(); err != nil {

        // Tryto auto-fix
        system.Warn("Config load failed, attempting auto-fix:", err)
        if err := autoFixConfig(err); err != nil {
            return fmt.Errorf("Config initialization failed: %w", err)
        }
        system.Info("Config auto-fixed successfully ✓")

        // Show notification about auto-fix 
        if app.notificationManager != nil {
            app.notificationManager.ShowError("Configuration Reset", "Config was reset to defaults")
        }
    }
    system.Debug("Configuration loaded ✓")

    // Phase 3: Startup Manager and permissions
    startupMgr, err := system.NewStartupManager()
    if err != nil {
        return fmt.Errorf("Startup manager initialization failed: %w", err)
    }
    app.startupManager = startupMgr
    system.Debug("Startup manager initialized ✓")

    // Phase 4: Storage and Checkpoint Manager
    checkpointMgr, err := checkpoint.NewCheckpointManager()
    if err != nil {
        return fmt.Errorf("Checkpoint manager initialization failed: %w", err)
    }
    app.checkpointManager = checkpointMgr
    system.Debug("Checkpoint manager initialized ✓")

    // Phase 5: Process Detection
    app.detector = process.NewProcessDetector()
    system.Debug("Process detector initialized ✓")

    // Phase 6: Application Launcher
    app.launcher = process.NewApplicationLauncher()
    system.Debug("Application launcher initialized ✓")

    // Phase 7: System Monitor
    monitor, err := system.NewSystemMonitor()
    if err != nil {
        return fmt.Errorf("System monitor initialization failed: %w", err)
    }
    app.monitor = monitor
    system.Debug("System monitor initialized ✓")

    // Phase 8: Notification Manager
    app.notificationManager = ui.NewNotificationManager()
    system.Debug("Notification manager initialized ✓")

    duration := time.Since(initStart)
    system.Info("All components initialized in", duration)

    // Log warning if initialization took too long, but continue
    if duration.Seconds() > 8 {
        system.Warn("Initialization exceeded 8-seconds target:", duration)
    }
    return nil
}
// autoFixConfig attempts to automatically fix configuration issues
func autoFixConfig(origErr error) error {
    system.Info("Attempting to auto-fix configuration...")
    
    // Backup current config if it exists
    homeDir, _ := os.UserHomeDir()
    configPath := filepath.Join(homeDir,".respawn", "config.json")

    if _, err := os.Stat(configPath); err == nil {
        backupPath := configPath + ".broken"
        if err := os.Rename(configPath, backupPath); err != nil {
            system.Warn("Could not backup broken config:", err)
        } else {
            system.Info("Backed up broken config to", backupPath)
        }
    }

    // Create fresh default config
    defaultCfg := config.DefaultConfig()

    // Validate default config
    if err := defaultCfg.Validate(); err != nil {
        return fmt.Errorf("Default config validation failed: %w", err)
    }

    // Save default config
    if err := defaultCfg.Save(); err != nil {
        return fmt.Errorf("failed to save default config: %w", err)
    }

    // Reload config
    if err := config.LoadConfig(); err != nil {
        return fmt.Errorf("Failed to reload config after auto-fix: %w", err)
    }

    system.Info("Configuration auto-fixed successfully")
    return nil
}

// handleInstall processes the install command     
func handleInstall() error {
    system.Info("Starting RESPAWN installation")

    // Check if first run
    if isFirstRun() {
        if err := showFirstTimeExperience(); err != nil {
            return fmt.Errorf("First-time setup failed: %w", err)
        }
    }

    // Initialize minimal components for installation
    app = &RESPAWNApp{}

    startupMgr, err := system.NewStartupManager()
    if err != nil {
        return fmt.Errorf("Startup manager creation failed: %w", err)
    }
    app.startupManager = startupMgr

    // Install auto-start
    if err := app.startupManager.Install(); err != nil {
        return fmt.Errorf("Installation failed: %w", err)
    }

    fmt.Println("✅ RESPAWN installed successfully!")
    fmt.Println("✅ Auto-start configured")
    fmt.Println("✅ Will start on next login")
    fmt.Println("\nRun 'respawn start' to start now, or restart your system.")
    
    return nil
}

//handleUninstall processes the uninstall command
func handleUninstall() error {
    system.Info("Starting RESPAWN uninstall....")

    app = &RESPAWNApp{}

    startupMgr, err := system.NewStartupManager()
    if err != nil {
        return fmt.Errorf("Startup manager creation failed: %w", err)
    }

    app.startupManager = startupMgr

    if err := app.startupManager.Uninstall(); err != nil {
        return fmt.Errorf("uninstall failed: %w", err)
    }

    fmt.Println("✅ RESPAWN uninstalled successfully")
    fmt.Println("Note: Checkpoint data preserved in ~/.respawn/")
    
    return nil
}

// handleStart processes the start command 
func handleStart() error {
    system.Info("Starting RESPAWN")

    // Always  daemonize on start
    if err := daemonize(); err != nil {
        return fmt.Errorf("Failed to daemonize: %w", err)
    }
    app = &RESPAWNApp{
        startTime: time.Now(),
        isRunning: true,
    }

    // Initialize all components 
    if err := initializeComponents(); err != nil {
        return fmt.Errorf("Component initialization failed: %w", err)
    }

    // Wait 10seconds for system stabilization
    system.Info("Waiting 10 seconds for system stabilization....")
    time.Sleep(10 * time.Second)

    // Show RESPAWN ACTIVE notification (regardless of init time)
    system.Info("System stabilized, showing active notification")
    if err := app.notificationManager.ShowError("RESPAWN Active", "Monitoring workspace"); err != nil {
        system.Warn("Failed to show active notification:", err)
    }

    // Start monitoring 
    if err := app.monitor.Start(); err != nil {
        return fmt.Errorf("monitor start failed: %w", err)
    }

    // Setup graceful shutdown
    setupGracefulShutdown()

    system.Info("RESPAWN is now running...")
    system.Info("Next checkpoint in:", config.GlobalConfig.CheckpointInterval)

    // Keep running until interrupted
    select{}
}

// daemonize forks the process and exits the parent
func daemonize() error {
    // Check if already a daemon
    if os.Getppid() == 1 {
        return nil // Already daemonized
    }
    // Fork the process
    cmd := exec.Command(os.Args[0], os.Args[1:]...)
    cmd.Stdout = nil
    cmd.Stderr = nil
    cmd.Stdin = nil

    if err := cmd.Start(); err != nil {
        return err
    }
    // Parent exits, child continues
    fmt.Printf("RESPAWN started in background (PID: %d)\n", cmd.Process.Pid)
    os.Exit(0)

    return nil
}

// Helper to check if running in background
func isBackgroundMode() bool {
    // Checks if parent process is launchd (PID 1)
    return os.Getppid() == 1
}

// Start process in background
func startInBackground() error {
    cmd := exec.Command(os.Args[0], "start", "--background")
    cmd.Stdout = nil
    cmd.Stderr = nil

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("Failed to start in background: %w", err)
    }

    fmt.Printf("✅ RESPAWN started in background (PID: %d)\n", cmd.Process.Pid)
    os.Exit(0)
    return nil
}

// handleRestore processes the restore command
func handleRestore() error {
    system.Info("Starting workspace restoration")

    app = &RESPAWNApp{}

    // Initialize necessary components
    if err := system.InitLogger(); err != nil {
        return fmt.Errorf("Logger initialization failed: %w", err)
    }

    if err := config.LoadConfig(); err != nil {
        return fmt.Errorf("Config load failed: %w", err)
    }

    checkpointMgr, err := checkpoint.NewCheckpointManager()
    if err != nil {
        return fmt.Errorf("Checkpoint manager creation failed: %w", err)
    }
    app.checkpointManager = checkpointMgr

    app.launcher = process.NewApplicationLauncher()
    app.notificationManager = ui.NewNotificationManager()

    var results []types.LaunchResult

    // Restore from specific checkpoint or latest
    if checkpointID != "" {
        system.Info("Restoring from checkpoint:", checkpointID)
        results, err = app.checkpointManager.RestoreFromCheckpoint(checkpointID)
    } else {
        system.Info("Restoring from latest checkpoint")
        results, err = app.checkpointManager.RestoreLatestCheckpoint()
    }

    if err != nil {
        return fmt.Errorf("Restoration failed: %w", err)
    }

    // Show progress (unless silent mode)
    if !silentMode {
        for _, result := range results {
            if result.Success {
                app.notificationManager.ShowAppRestored(result.AppName, result.LaunchTime)
            }
        }
    }

    // Show summary
    successful, failed, failedApps := app.launcher.GetLaunchSummary()

    if !silentMode {
        summary := types.RestoreSummary{
            TotalApps:      successful + failed,
            SuccessfulApps: successful,
            FailedApps:     failed,
            FailedAppNames: failedApps,
        }
        app.notificationManager.ShowRestoreComplete(summary)
    }

    fmt.Printf("✅ Restored %d applications\n", successful)
    if failed > 0 {
        fmt.Printf("⚠️  %d applications failed to restore\n", failed)
    }

    return nil
}

// handleCheckpoint processes the checkpoint command
func handleCheckpoint() error {
    system.Info("Creating forced checkpoint")

    app = &RESPAWNApp{}

    // Initialize necessary components
    if err := config.LoadConfig(); err != nil {
        return fmt.Errorf("Coonfig load failed: %w", err)
    }

    checkpointMgr, err := checkpoint.NewCheckpointManager()
    if err != nil {
        return fmt.Errorf("Checkpoint manager creation failed: %w", err)
    }
    app.checkpointManager = checkpointMgr

    // Create checkpoint
    cp, err := app.checkpointManager.CreateCheckpoint()
    if err != nil {
        return fmt.Errorf("Checkpoint creation failed: %w", err)
    }

    fmt.Printf("✅ Checkpoint created: %s\n", cp.ID)
    fmt.Printf("   Applications saved: %d\n", len(cp.Processes))
    fmt.Printf("   Size: %d bytes\n", cp.FileSize)
    
    return nil
}

// handleStatus processes the status command 
func handleStatus() error {
    system.Info("Checking RESPAWN status")

    //Initialize minimal component
    if err := system.InitLogger(); err != nil {
        return fmt.Errorf("Logger initialization failed: %w",err)
    }

    if err := config.LoadConfig(); err != nil {
        return fmt.Errorf("Config load failed: %w", err)
    }

    checkpointMgr, err := checkpoint.NewCheckpointManager()
    if err != nil {
        return fmt.Errorf("Checkpoint manager creation failed: %w", err)
    }

    startupMgr, err := system.NewStartupManager()
    if err != nil {
        return fmt.Errorf("Startup manager creation failed: %w", err)
    }

    // Check if RESPAWN is running
    isRunning := false
    pidFile := filepath.Join(os.Getenv("HOME"), ".respawn", "respawn.pid")
    if pidData, err := os.ReadFile(pidFile); err == nil {
        if pid, err := strconv.Atoi(strings.TrimSpace(string(pidData))); err == nil {
            if process, err := os.FindProcess(pid); err == nil {
                if err := process.Signal(syscall.Signal(0)); err == nil {
                    isRunning = true
                }
            }
        }
    }

    // Get checkpoint list
    checkpointList, err := checkpointMgr.GetAvailableCheckpoints()
    if err != nil {
        return fmt.Errorf("Failed to get checkpoints: %w", err)
    }

    //Display Status
    fmt.Println("\n=== RESPAWN STATUS ===")
    fmt.Printf("Version: %s\n", Version)
    fmt.Printf("Running: %s\n", boolToStatus(isRunning))
    fmt.Printf("Auto-start: %s\n", boolToStatus(startupMgr.IsEnabled()))
    
    // Show pause state
    pauseFile := filepath.Join(os.Getenv("HOME"), ".respawn", "paused")
    if _, err := os.Stat(pauseFile); err == nil {
        fmt.Printf("Status: ⏸️  PAUSED\n")
    } else if isRunning {
        fmt.Printf("Status: ✅ ACTIVE - Monitoring\n")
    } else {
        fmt.Printf("Status: ❌ STOPPED\n")
    }
    
    fmt.Printf("\nCheckpoints:\n")
    fmt.Printf("  Total: %d\n", checkpointList.TotalCount)    

    if len(checkpointList.Checkpoints) > 0 {
        latest := checkpointList.Checkpoints[0]
        fmt.Printf("  Latest: %s\n", latest.ID)
        fmt.Printf("  Created: %s\n", latest.Timestamp.Format("2006-01-02 15:04:05"))
        fmt.Printf("  Apps in latest: %d\n", len(latest.AppNames))
        
        if len(latest.AppNames) > 0 {
            fmt.Printf("  Applications:\n")
            for i, app := range latest.AppNames {
                if i >= 10 {
                    fmt.Printf("    ... and %d more\n", len(latest.AppNames)-10)
                    break
                }
                fmt.Printf("    - %s\n", app)
            }
        }
        
        // Show next checkpoint time
        if isRunning {
            nextCheckpoint := latest.Timestamp.Add(config.GlobalConfig.CheckpointInterval)
            timeUntil := time.Until(nextCheckpoint)
            if timeUntil > 0 {
                fmt.Printf("\n  Next checkpoint in: %s\n", timeUntil.Round(time.Minute))
            } else {
                fmt.Printf("\n  Next checkpoint: Overdue (should create soon)\n")
            }
        }
    } else {
        fmt.Printf("  No checkpoints yet\n")
    }
    
    fmt.Printf("\nConfiguration:\n")
    fmt.Printf("  Checkpoint interval: %v\n", config.GlobalConfig.CheckpointInterval)
    fmt.Printf("  Data retention: %d days\n", config.GlobalConfig.DataRetentionDays)
    
    return nil
}
// handleEnableAutoStart processes the enable-autostart command
func handleEnableAutoStart() error {
    app = &RESPAWNApp{}

    startupMgr, err := system.NewStartupManager()
    if err != nil {
        return err
    }
    app.startupManager = startupMgr

    return app.startupManager.EnableAutoStart()
}

// handleDisableAutoStart runs the diable-autostart command 
func handleDisableAutoStart() error {
    app = &RESPAWNApp{}

    startupMgr, err := system.NewStartupManager()
    if err != nil {
        return err
    }
    app.startupManager = startupMgr

    return app.startupManager.DisableAutoStart()
}

// handlePause runs the pause command 
func handlePause() error {
    // Create pause marker file
    homeDir, _ := os.UserHomeDir()
    pauseFile := filepath.Join(homeDir, ".respawn", "paused")

    if err := os.WriteFile(pauseFile, []byte(time.Now().String()), 0644); err != nil {
        return fmt.Errorf("Failed to create pause marker: %w", err)
    }

    fmt.Println("✅ RESPAWN monitoring paused")
    fmt.Println("Run 'respawn resume' to resume monitoring")
    
    return nil
}

// handleResume runs the resume command 
func handleResume() error {
    // Remove pause marker file
    homeDir, _ := os.UserHomeDir()
    pauseFile := filepath.Join(homeDir, ".respawn", "paused")

    if err := os.Remove(pauseFile); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("Failed to remove pause marker: %w", err)
    }

    fmt.Println("✅ RESPAWN monitoring resumed")

    return nil
}

// setupGracefulShutdown handles graceful shutdown or signals 
func setupGracefulShutdown() {
    sigChan :=  make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        sig := <-sigChan
        system.Info("Received signal:", sig)

        if err := gracefulShutdown(); err != nil {
            system.Error("Graceful shutdown failed:", err)
            os.Exit(1)
        }

        os.Exit(0)
    }()
}

// gracefulShutdown performs graceful shutdown with checkpoint logic
func gracefulShutdown() error {
    system.Info("Starting graceful shutdown")

    if app == nil || !app.isRunning {
        return nil
    }

    timeSinceLastCheckpoint := time.Since(app.lastCheckpointTime)

    if timeSinceLastCheckpoint < 60*time.Minute {
        // Less than 1 hour - quit immediately
        system.Info("Recent checkpoint exists, quitting immediately")
        return cleanup()
    }

    if timeSinceLastCheckpoint >= 120*time.Minute {
        // 2+ hours - ask user
        system.Info("Last checkpoint over 2 hours ago, asking user")

        _, err := app.notificationManager.ShowPermissionRequest(
            "Checkpoint",
            "Last checkpoint was over 2 hours ago.\nCreate checkpoint before quitting?",
        )

        if err == nil {
            // User chose to create checkpoint
            if _, err := app.checkpointManager.CreateCheckpoint(); err != nil {
                system.Error("Failed to create final checkpoint:", err)
            } else {
                system.Info("Final checkpoint created successfully")
            }
        }
    }
    return cleanup()
}
// cleanUp runs cleanup operation
func cleanup() error {
    system.Info("Performing cleanup")

    if app.startupManager != nil {
        app.startupManager.Cleanup()
    }

    if app.monitor != nil {
        app.monitor.Stop()
    }

    system.Close()

    return nil 


}

// isFirstRun check if this is the first time RESPAWN is run
func isFirstRun() bool {
    homeDir, _ := os.UserHomeDir()
    firstRunMarker := filepath.Join(homeDir, ".respawn", "first_run")

    _, err := os.Stat(firstRunMarker)
    return os.IsNotExist(err)
}

// showFirstTimeExperience displays first-time setup wizard 
func showFirstTimeExperience() error {
    system.Info("Showing first-time experience")

    // Show welcome dialog using AppleScript
    welcomeScript := fmt.Sprintf(`
        display dialog "Welcome to RESPAWN
By NINSCO

Automatic workspace restoration
Simple. Powerful. Invisible.

%s
%s

Ready to begin setup?" with title "Welcome to RESPAWN" buttons {"Begin Setup", "Learn More"} default button "Begin Setup" with icon note
    `, Version, Copyright)

    cmd := exec.Command("osascript", "-e", welcomeScript)
    output, err := cmd.Output()

    if err != nil || !strings.Contains(string(output), "Begin Setup") {
        return fmt.Errorf("User cancelled setup")
    }

    // Mark first run complete
    homeDir, _ := os.UserHomeDir()
    firstRunMarker := filepath.Join(homeDir, ".respawn", "first_run")
    os.MkdirAll(filepath.Dir(firstRunMarker), 0755)
    os.WriteFile(firstRunMarker, []byte(time.Now().String()), 0644)

    system.Info("First-time experience completed")    
    return nil
}

//boolToStatus converts boolean to status string
func boolToStatus(enabled bool) string {
    if enabled {
        return "✅ Enabled"
    }
    return "❌ Disabled"
}
