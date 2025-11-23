package config

import (
	"fmt"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

)


type AppConfig struct {
	Name        string `json:"name"`
	ProcessName string `json:"process_name"`
	Enabled     bool   `json:"enabled"`
}

type Config struct {
	// Application Monitoring 
	Applications []AppConfig `json:"applications"`

	// checkpoint settings
	CheckpointInterval time.Duration	`json:"checkpoint_interval"`
	DataRetentionDays  int 		`json:"data_rentention_days"`

	// System settings
	AutoRestore bool `json:"auto_restore"`
	MaxRetryAttempts int `json:"max_retry_attempts"`
	LaunchDelayMs int `json:"launch_delay_ms"`

	// Paths
	DataDir string `json:"data_dir"`
	LogDir  string `json:"log_dir"`
	ConfigPath string `json:"config_path"`
}

var GlobalConfig *Config 

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".respawn")


	return &Config{	
		Applications: []AppConfig{
			{Name: "Google Chrome", ProcessName: "Google Chrome", Enabled: true},
            {Name: "Safari", ProcessName: "Safari", Enabled: true},
            {Name: "Brave Browser", ProcessName: "Brave Browser", Enabled: true},
            {Name: "TextEdit", ProcessName: "TextEdit", Enabled: true},
            {Name: "Firefox", ProcessName: "Firefox", Enabled: true},
            {Name: "Claude", ProcessName: "Claude", Enabled: true},
            {Name: "Preview", ProcessName: "Preview", Enabled: true},

		},

		CheckpointInterval: 15 * time.Minute, // 15 minutes 
		DataRetentionDays: 7, // 7 days
		AutoRestore: true,
		MaxRetryAttempts: 3,
		LaunchDelayMs: 7000, // 7 seconds
		DataDir: dataDir,
		LogDir: filepath.Join(dataDir, "logs"),
		ConfigPath: filepath.Join(dataDir, "config.json"),
	}
}

// LoadConfig loads configuration from file or creates default
func LoadConfig() error {
    config := DefaultConfig()
    
    // Create data directory if it doesn't exist
    if err := os.MkdirAll(config.DataDir, 0755); err != nil {
        return fmt.Errorf("failed to create data directory: %w", err)
    }
    
    // Try to load existing config
    if _, err := os.Stat(config.ConfigPath); err == nil {
        data, err := os.ReadFile(config.ConfigPath)
        if err != nil {
            return fmt.Errorf("failed to read config file: %w", err)
        }
        
        if err := json.Unmarshal(data, config); err != nil {
            return fmt.Errorf("failed to parse config file: %w", err)
        }
    }
    
    // Set the config path (not saved to JSON)
    config.ConfigPath = filepath.Join(config.DataDir, "config.json")
    
    // Validate configuration
    if err := config.Validate(); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }
    
    // Save config (creates file if it doesn't exist or updates if validation fixed something)
    if err := config.Save(); err != nil {
        return fmt.Errorf("failed to save config: %w", err)
    }
    
    GlobalConfig = config
    return nil
}
// Save writes the configuration to file
func (c *Config) Save() error {
    data, err := json.MarshalIndent(c, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal config: %w", err)
    }
    
    if err := os.WriteFile(c.ConfigPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write config file: %w", err)
    }
    
    return nil
}

// Validate checks if configuration values are valid
func (c *Config) Validate() error {
    // Validate data retention
    if c.DataRetentionDays <= 0 {
        return fmt.Errorf("data_retention_days must be greater than 0, got %d", c.DataRetentionDays)
    }
    
    // Validate checkpoint interval
    if c.CheckpointInterval <= 0 {
        return fmt.Errorf("checkpoint_interval must be greater than 0")
    }
    
    // Validate retry attempts
    if c.MaxRetryAttempts < 1 {
        c.MaxRetryAttempts = 3 // Fix with default
    }
    
    // Validate launch delay
    if c.LaunchDelayMs < 0 {
        c.LaunchDelayMs = 2000 // Fix with default
    }
    
    // Validate applications list
    if len(c.Applications) == 0 {
        return fmt.Errorf("applications list cannot be empty")
    }
    
    // Validate each application config
    for i, app := range c.Applications {
        if app.Name == "" {
            return fmt.Errorf("application at index %d has empty name", i)
        }
        if app.ProcessName == "" {
            return fmt.Errorf("application '%s' has empty process_name", app.Name)
        }
    }
    
    // Validate and create directories
    if err := os.MkdirAll(c.DataDir, 0755); err != nil {
        return fmt.Errorf("failed to create data directory: %w", err)
    }
    
    if err := os.MkdirAll(c.LogDir, 0755); err != nil {
        return fmt.Errorf("failed to create log directory: %w", err)
    }
    
    return nil
}
// GetEnabledApplications returns only enabled applications
func (c *Config) GetEnabledApplications() []AppConfig {
    var enabled []AppConfig
    for _, app := range c.Applications {
        if app.Enabled {
            enabled = append(enabled, app)
        }
    }
    return enabled
}

// IsApplicationEnabled checks if a specific application is enabled
func (c *Config) IsApplicationEnabled(processName string) bool {
    for _, app := range c.Applications {
        if app.ProcessName == processName && app.Enabled {
            return true
        }
    }
    return false
}


