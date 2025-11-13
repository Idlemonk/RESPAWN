package types

import "time"

// Position represents x/y coordinates
type Position struct {
	X int `json:"x,omitempty"`
	Y int `json:"y,omitempty"`
}

// Size represents width/height dimensions
type Size struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

// WindowInfo holds window data
type WindowInfo struct {
	Title       string  `json:"title,omitempty"`
	Position    Position `json:"position,omitempty"`
	Size        Size    `json:"size,omitempty"`
	IsMinimized bool    `json:"is_minimized,omitempty"`
	IsFullscreen bool   `json:"is_fullscreen,omitempty"`
}

// ApplicationInfo holds app data
type ApplicationInfo struct {
	Name         string       `json:"name,omitempty"`
	BundleID     string       `json:"bundle_id,omitempty"`
	ExecutablePath string    `json:"executable_path,omitempty"`
	Windows      []WindowInfo `json:"windows,omitempty"`
	PID          int          `json:"pid,omitempty"`
}

// ProcessInfo represents a running process with it's state
type ProcessInfo struct {
	PID         int    `json:"pid"`
	Name        string `json:"name"`
	ProcessName string `json:"process_name"`
	MemoryMB    int64  `json:"memory_mb"`
	WindowState string `json:"window_state"` // "normal", "minimized", "maximized"
	IsRunning   bool   `json:"is_running"`
}

// New embedding: Extend ProcessInfo with WindowInfo slice
type ExtendedProcessInfo struct {
    Windows []WindowInfo      // Augmented GUI window slice
}


// LaunchResult represents the result of launching an application
type LaunchResult struct {
	AppName    string    `json:"app_name"`
	Success    bool      `json:"success"`
	PID        int       `json:"pid"`
	LaunchTime time.Time `json:"launch_time"`
	RetryCount int       `json:"retry_count"`
	ErrorMsg   string    `json:"error_msg,omitempty"`
}

// Checkpoint represents a system checkpoint
type Checkpoint struct {
	ID          string        `json:"id"`
	Timestamp   time.Time     `json:"timestamp"`
	Processes   []ProcessInfo `json:"processes"`
	AppNames    []string      `json:"app_names"`
	IsCompressed bool         `json:"is_compressed"`
	FilePath    string        `json:"file_path"`
	FileSize    int64         `json:"file_size"`
}

// CheckpointList contains a list of checkpoints with metadata
type CheckpointList struct {
	Checkpoints     []Checkpoint `json:"checkpoints"`
	LastUsed        string       `json:"last_used"`
	TotalCount      int          `json:"total_count"`
	CompressedCount int          `json:"compressed_count"`
}

// CheckpointStatus contains checkpoint operation status
type CheckpointStatus struct {
	Success      bool   `json:"success"`
	CheckpointID string `json:"checkpoint_id"`
	Timestamp    time.Time `json:"timestamp"`
	ErrorMessage string `json:"error_message,omitempty"`
	AppsCount    int    `json:"apps_count"`
}

// RestartPolicy defines restart behavior after crashes
type RestartPolicy struct {
	MaxRetries     int
	BackoffIntervals []time.Duration
	CurrentRetry   int
	LastCrashTime  time.Time
}

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

// StatusSummary contains RESPAWN status information
type StatusSummary struct {
	LastCheckpoint time.Time `json:"last_checkpoint"`
	TotalCheckpoints int    `json:"total_checkpoints"`
	AutoStartEnabled bool   `json:"auto_start_enabled"`
	HealthStatus   string   `json:"health_status"`
}
