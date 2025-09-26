package checkpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"RESPAWN/internal/process"
	"RESPAWN/internal/system"
	"RESPAWN/pkg/config"

)

type CheckpointManager struct {
	checkpointDir string
	storage       *Storage 
	detector 	  *process.ProcessDetector
}

type Checkpoint struct {
	ID          string              `json:"id"`
    Timestamp   time.Time           `json:"timestamp"`
    Processes   []process.ProcessInfo `json:"processes"`
    AppNames    []string            `json:"app_names"`
    IsCompressed bool               `json:"is_compressed"`
    FilePath    string              `json:"file_path"`
    FileSize    int64               `json:"file_size"`
}

type CheckpointList struct {
    Checkpoints    []Checkpoint `json:"checkpoints"`
    LastUsed       string       `json:"last_used"`
    TotalCount     int          `json:"total_count"`
    CompressedCount int         `json:"compressed_count"`
}

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager() (*CheckpointManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("Failed to get home directory: %w", err)
	}

	checkpointDir := filepath.Join(homeDir, ".respawn", "checkpoints")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return nil, fmt.Errorf("Failed to create checkpoint directory: %w", err)
	}

	storage, err := NewStorage(checkpointDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize storage: %w", err)
	}

	return &CheckpointManager{
		checkpointDir: checkpointDir,
        storage:       storage,
        detector:      process.NewProcessDetector(),
    }, nil
}

// Creates a new system checkpoint
func (cm *CheckpointManager) CreateCheckpoint() (*Checkpoint, error) {
	system.Info("Creating new checkpoint")

	// Detect running processes
	processes, err := cm.detector.DetectRunningProcesses()
	if err != nil {
		return nil, fmt.Errorf("Failed to detect running processes: %w", err)
	}

	if len(processes) == 0 {
		system.Warn ("No target application running, Empty checkpoint created")
	}

	// Create Checkpoint
	timestamp := time.Now()
	checkpointID := timestamp.Format("2006-01-15_15-04-05")

	// Extract app names for descriptive naming 
	appNames := make([]string, len(processes))
	for i, proc := range processes {
		appNames[i] = proc.Name
	}

	checkpoint := &Checkpoint{
        ID:          checkpointID,
        Timestamp:   timestamp,
        Processes:   processes,
        AppNames:    appNames,
        IsCompressed: false,	
	}

	// Save checkpoint to storage
	filePath, fileSize, err := cm.storage.SaveCheckpoint(checkpoint) 
	if err != nil {
		return nil, fmt.Errorf("Failed to save checkpoint: %w", err)
	}

	checkpoint.FilePath = filePath
	checkpoint.FileSize = fileSize

	system.Info("Created checkpoint:", cm.formatCheckpointName(checkpoint))
	system.Debug("Checkpoint saved to:", filePath, "Size:", fileSize, "bytes")
	return checkpoint, nil
}

// GetAvailableCheckpoints returns all available checkpoints with descriptive names 
func (cm *CheckpointManager) GetAvailableCheckpoints() (*CheckpointList, error) {
	system.Debug("Loading available checkpoints")

	checkpoints, err := cm.storage.LoadAllCheckpoints()
	if err != nil {
		return nil, fmt.Errorf("Failed to load checkpoints: %w", err)
	}

	// Sort by  timestamp (newest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.After(checkpoints[j].Timestamp)
	})

	// Count compressed checkpoints
	compressedCount := 0
	for _, cp := range checkpoints {
		if cp.IsCompressed {
            compressedCount++
		}
	}

	return &CheckpointList{
		Checkpoints:     checkpoints,
        LastUsed:        cm.getLastUsedCheckpoint(checkpoints),
        TotalCount:      len(checkpoints),
        CompressedCount: compressedCount,
    }, nil
}

// RestoreFromCheckpoint restores system state from a specific checkpoint
func (cm *CheckpointManager) RestoreFromCheckpoint(checkpointID string) ([]process.LaunchResult, error) {
	system.Info("Restoring from checkpoint:", checkpointID)

	// Load the specific checkpoint
	checkpoint, err := cm.storage.LoadCheckpointByID(checkpointID)
	if err != nil {
		return nil, fmt.Errorf("Failed to load checkpoint %s: %w", checkpointID, err)
	}

	system.Info("Loaded checkpoint:", cm.formatCheckpointName(checkpoint))
	system.Debug("Checkpoint contains", len(checkpoint.Processes), "applications")

	// Update last used checkpoint
	cm.updateLastUsedCheckpoint(checkpointID)

	// Launch applications
	launcher := process.NewApplicationLauncher()
	results, err := launcher.RestoreApplications(checkpoint.Processes)
	if err != nil {
		return results, fmt.Errorf("Failed to restore applications: %w", err)
	}

	successful, failed, failedApps := launcher.GetLaunchSummary()
	system.Info ("Restoration completed - Success:", successful, "Failed:", failed)

	if failed > 0 {
		system.Warn("Failed applications:", strings.Join(failedApps, ", "))
	} 

	return results, nil
} 

// RestoreLatestCheckpoint restores from the most recent checkpoint
func (cm *CheckpointManager) RestoreLatestCheckpoint() ([]process.LaunchResult, error) {
	system.Info("Restoring from latest checkpoint")

	checkpointList, err := cm.GetAvailableCheckpoints()
	if err != nil {
		return nil, fmt.Errorf("Failed to get checkpoints: %w", err)
	}

	if len(checkpointList.Checkpoints) == 0 {
		return nil, fmt.Errorf("No checkpoints available for restoration")
	}

	latestCheckpoint := checkpointList.Checkpoints[0] // Already sorted by newest first
	return cm.RestoreFromCheckpoint(latestCheckpoint.ID)
}

// DisplayCheckpointMenu shows available checkpoints with descriptive names and success icons
func (cm *CheckpointManager) DisplayCheckpointMenu() error {
	checkpointList, err := cm.GetAvailableCheckpoints()
	if err != nil {
		return fmt.Errorf("Failed to load checkpoints: %w", err)
	}

	if len(checkpointList.Checkpoints) == 0 {
		fmt.Println("No checkpoints available.")
		return nil
	}

	fmt.Printf("\n=== AVAILABLE CHECKPOINTS ===\n")
	fmt.Printf("Total: %d | Compressed: %d\n\n", checkpointList.TotalCount, checkpointList.CompressedCount)

	for i, checkpoint := range checkpointList.Checkpoints {
		status := "✅"
		if checkpoint.IsCompressed {
			status += " 📦" // Add compression indicator
		}
		fmt.Printf("%d. CP: [%s] %s\n", i+1, cm.formatCheckpointName(&checkpoint), status)  
	}

	if checkpointList.LastUsed != "" {
		fmt.Printf("\nLast used: %s\n", checkpointList.LastUsed)
	}
	return nil 
}

// PerformMaintenanceTasks runs background maintenance
func (cm *CheckpointManager) PerformMaintenanceTasks() error {
	system.Debug("Starting maintenance tasks")

	// Check disk space
	if err := cm.checkDiskSpace(); err != nil {
		system.Warn("Disk space check failed:", err)
	}

	// Clean old checkpoints based on retention policy
	if err := cm.cleanOldCheckpoints(); err != nil {
		system.Warn("Cleanup failed:", err)
	}

	// Compress eligible checkpoints (after 24 hours)
	if err := cm.compressOldCheckpoints(); err != nil {
		system.Warn("Compression failed:", err)
	}

	system.Debug("Maintenance tasks completed")
	return nil
}

// Helper functions

//formatCheckpointName creates descriptive checkpoint name 
func (cm *CheckpointManager) formatCheckpointName(checkpoint *Checkpoint) string {
	appList := strings.Join(checkpoint.AppNames, ", ")
	if appList == "" {
		appList = "No applications"
	}
	return fmt.Sprintf("%s (%s)", checkpoint.ID, appList)
}

// getLastUsedCheckpoint determines which checkpoit was last used for restoration
func (cm *CheckpointManager) getLastUsedCheckpoint(checkpoints []Checkpoint) string {
	// For now, we'll implement this as a simple file-based tracking
	// in a more sophisticated version, this would be stored in metadata
	return ""
}

//updateLastUsedCheckpoint updates the last used checkpoint record
func (cm *CheckpointManager) updateLastUsedCheckpoint(checkpointID string) {
	system.Debug("Updating last used checkpoint to:", checkpointID)
	// Implementation would store this information persistently 
}

//checkDiskSpace monitors disk space and triggers cleanup if needed
func (cm *CheckpointManager) checkDiskSpace() error {
	// Implementation for disk space checking
	// This would check if we're above 75% threshold
	return nil
}

// cleanOldCheckpoints removes checkpoints older than retention period
// This function `cleanOldCheckpoints` in the `CheckpointManager` struct is responsible for removing
// checkpoints that are older than a specified retention period.
func (cm *CheckpointManager) cleanOldCheckpoints() error {
	retentionDays := config.GlobalConfig.DataRetentionDays
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	system.Debug("Cleaning checkpoints older than", retentionDays, "days")

	return cm.storage.cleanOldCheckpoints(cutoffTime)
}

// compressOldCheckpoints compresses checkpoints older than 24 hours from last used 
func (cm *CheckpointManager) compressOldCheckpoints() error {
	system.Debug("Starting checkpoint compression")

	checkpointList, err := cm.GetAvailableCheckpoints()
	if err != nil {
		return err
	}

	if len(checkpointList.Checkpoints) == 0 {
		return nil
	}
	// Find last used checkpoint or use latest as reference
	var lastUsedTime time.Time 
	if checkpointList.LastUsed != "" {
		// Find the last used checkpoint's timestamp
		for _, cp := range checkpointList.Checkpoints {
			if cp.ID == checkpointList.LastUsed {
				lastUsedTime = cp.Timestamp
				break
			}
		}
	}

	// if no last used found, use the latest checkpoint
	if lastUsedTime.IsZero() && len(checkpointList.Checkpoints) > 0 {
		lastUsedTime = checkpointList.Checkpoints[0].Timestamp
	}

	// Compress checkpoints older than 24 hours from last used
	compressionThreshold := lastUsedTime.Add(-24 * time.Hour)

	for _, checkpoint := range checkpointList.Checkpoints {
		if !checkpoint.IsCompressed && checkpoint.Timestamp.Before(compressionThreshold) {
			system.Debug("Compessing checkpoint:", checkpoint.ID)
			if err := cm.storage.CompressCheckpoint(&checkpoint); err != nil {
				system.Warn("Failed to compress checkpoint", checkpoint.ID, ":", err)
			}
		}
	}
	return nil 
}










