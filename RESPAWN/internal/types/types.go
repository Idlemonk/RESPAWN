package types

import "time"

// ProcessInfo represents a running process with it's state
type ProcessInfo struct {
	PID         int    `json:"pid"`
    Name        string `json:"name"`
    ProcessName string `json:"process_name"`
    MemoryMB    int64  `json:"memory_mb"`
    WindowState string `json:"window_state"` // "normal", "minimized", "maximized"
    IsRunning   bool   `json:"is_running"`
}

// LaunchResult represents the result of launching an application
type LaunchResult struct {
    AppName     string    `json:"app_name"`
    Success     bool      `json:"success"`
    PID         int       `json:"pid"`
    LaunchTime  time.Time `json:"launch_time"`
    RetryCount  int       `json:"retry_count"`
    ErrorMsg    string    `json:"error_msg,omitempty"`
}

// Checkpoint represents a system checkpoint
type Checkpoint struct {
    ID           string        `json:"id"`
    Timestamp    time.Time     `json:"timestamp"`
    Processes    []ProcessInfo `json:"processes"`
    AppNames     []string      `json:"app_names"`
    IsCompressed bool          `json:"is_compressed"`
    FilePath     string        `json:"file_path"`
    FileSize     int64         `json:"file_size"`
}

// CheckpointList contains a list of checkpoints with metadata
type CheckpointList struct {
    Checkpoints     []Checkpoint `json:"checkpoints"`
	LastUsed        string       `json:"last_used"`
	TotalCount      int          `json:"total_count"`
	CompressedCount int          `json:"compressed_count"`
}

// RestartPolicy defines restart behavior after crashes
type RestartPolicy struct {
    MaxRetries       int
    BackoffIntervals []time.Duration
    CurrentRetry     int
    LastCrashTime    time.Time
}

// RestoreSummary contains restoration completion details
type RestoreSummary struct {
    TotalApps       int
    SuccessfulApps  int
    FailedApps      int
    SkippedApps     int
    TotalDuration   time.Duration
    FailedAppNames  []string
    StartTime       time.Time
    EndTime         time.Time
}

// CheckpointStatus contains checkpoint operation status
type CheckpointStatus struct {
    Success       bool
    CheckpointID  string
    Timestamp     time.Time
    ErrorMessage  string
    AppsCount     int
}

// StatusSummary contains RESPAWN status information
type StatusSummary struct {
    LastCheckpoint    time.Time
    TotalCheckpoints  int
    AutoStartEnabled  bool
    HealthStatus      string
}