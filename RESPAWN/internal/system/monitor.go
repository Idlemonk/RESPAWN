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

    "RESPAWN/pkg/config"
)

type SystemState int

const (
    StateUnknown SystemState = iota
    StateFirstRun
    StateNormal
    StateSleep
    StateRestart
    StateCrash
    StateHighCPU
    StateLowBattery
    StateAboutToSleep
)

type UserActivity int

const (
    ActivityIdle UserActivity = iota
    ActivityLight
    ActivityWorking
    ActivityIntensive
)

type WorkPattern struct {
	StartHour           int             `json:"start_hour"`
	EndHour             int             `json:"end_hour"`
	ActiveAppThreshold  int             `json:"active_app_threshold"`
	IdleTimeBeforeSleep time.Duration   `json:"idle_time_before_sleep"`
	CPUPatterns         map[int]float64 `json:"cpu_patterns"`                               // Hour -> Average CPU
	AppUsageFrequency   map[string]int  `json:"app_usage_frequency"`                    // App Name -> Usage Count
	TopThreeApps        []string        `json:"top_three_apps"`
	LearningStartDate   time.Time       `json:"learning_start_date"`
	IsLearningComplete  bool            `json:"is_learning_complete"`
}


type OptimizationMetrics struct {
    CheckpointDurations []time.Duration `json:"checkpoint_durations"`
    RestoreSuccessRate  float64         `json:"restore_success_rate"`
    DiskGrowthRate      float64         `json:"disk_growth_rate_mb_per_week"`
    LastOptimization    time.Time       `json:"last_optimization"`
}

type SystemMonitor struct {
    workPattern       *WorkPattern
    metrics           *OptimizationMetrics
    isRunning         bool
    lastHeartbeat     time.Time
    lastCheckpoint    time.Time
    processID         int
    baseDir           string
}

// NewSystemMonitor Creates a new system monitor
func NewSystemMonitor() (*SystemMonitor, error) {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("Failed to get home directory: %w", err)
    }

    baseDir := filepath.Join(homeDir, ".respawn")

    monitor := &SystemMonitor{
		processID:     os.Getpid(),
		baseDir:       baseDir,
		lastHeartbeat: time.Now(),
	}

    // Load or create work pattern
    if err := monitor.loadWorkPattern(); err != nil {
        Info("Creating new work pattern learning profile")
        monitor.workPattern = &WorkPattern{
            StartHour:           21, // Default 9 PM
            EndHour:             5,  // Default 5 AM  
            ActiveAppThreshold:  3,
            IdleTimeBeforeSleep: 15 * time.Minute,
            CPUPatterns:         make(map[int]float64),
            AppUsageFrequency:   make(map[string]int),
            TopThreeApps:        []string{},
            LearningStartDate:   time.Now(),
            IsLearningComplete:  false,
        }
        monitor.saveWorkPattern()
    }

    // Load optimization metrics
    if err := monitor.loadMetrics(); err != nil {
        monitor.metrics = &OptimizationMetrics{
            CheckpointDurations: make([]time.Duration, 0),
            RestoreSuccessRate:  1.0,
            DiskGrowthRate:      0.0,
            LastOptimization:    time.Now(),
        }
    }
    return monitor, nil
}

// Start begins the monitoring process
func (sm *SystemMonitor) Start() error {
    Info("Starting RESPAWN system monitor")
    sm.isRunning = true

    // Check system state on startup
    state := sm.DetectSystemState()
    Info("System state detected:", sm.stateToString(state))

    //Handle system state
    if err := sm.handleSystemState(state); err != nil {
        Error("Failed to handle system state:", err)
        return err 
    }

    // Start monitoring loop
    go sm.monitoringLoop()
    go sm.heartbeatLoop()
    go sm.learningLoop()

    Info("System monitor started successfully")
    return nil 
}

// DetectSystemState determines current system state using hybrid detection
func (sm *SystemMonitor) DetectSystemState() SystemState {
    Debug ("Detecting system state")

    // Check if first run
    if sm.isFirstRun() {
        return StateFirstRun
    }

    // Get system uptime
    uptime, err := sm.getSystemUptime()
    if err != nil {
        Warn("Failed to get system uptime:", err)
        return StateUnknown
    }

    // Get last heartbeat time
    lastHeartbeat := sm.getLastHeartbeatTime()
    if lastHeartbeat.IsZero() {
        Debug("No previous heartbeat found")
        return StateRestart
    }

    //Calculate time since last heartbeat
    timeSinceHeartbeat := time.Since(lastHeartbeat)

    Debug("System uptime:", uptime, "Time since last heartbeat:", timeSinceHeartbeat)

    // Hybrid detection logic
    if uptime < timeSinceHeartbeat {
        // System uptime is less than time since last heartbeat = RESTART
        Info("Restart detected - uptime:", uptime, "<heartbeat gap:", timeSinceHeartbeat)
        return StateRestart
    }

    if timeSinceHeartbeat > 2*time.Hour && uptime > timeSinceHeartbeat {
        // Long gap but uptime matches = SLEEP
        Info("Sleep cycle detected - long heartbeat gap but matching uptime")
        return StateSleep
    }

    // Check for RESPAWN crash
    if !sm.wasProcessRunning() && timeSinceHeartbeat > 5*time.Minute {
        Info("RESPAWN crash detected - process not found but system uptime matches")
        return StateCrash
    }

    return StateNormal
}

// handleSystemState responds appropriately to detected system state
func (sm *SystemMonitor) handleSystemState(state SystemState) error {
    switch state {
    case StateFirstRun:
        Info("First run detected - creating initial checkpoint")
        return sm.createInitialCheckpoint()

    case StateRestart:
        Info("System restart detected - initiating restoration")
        return sm.handleSystemRestart()

    case StateSleep:
        Info("Sleep cycle detected - no restoration needed")
        return sm.updateAfterSleep()

    case StateCrash:
        Info("RESPAWN crash detected - showing recovery options")
        return sm.handleCrashRecovery()

    case StateNormal:
        Info("Normal startup - resuming monitoring")
        return sm.resumeNormalOperation()

    default:
        Warn("Unknown system state - defaulting to normal operation")
        return sm.resumeNormalOperation()
    }
}

// monitoringLoop runs the main monitoring cycle 
func (sm *SystemMonitor) monitoringLoop() {
    Debug("Starting monitoring loop")

    ticker := time.NewTicker(10 * time.Minute) // check every 10 minutes
    defer ticker.Stop()

    for sm.isRunning {
        select {
        case <-ticker.C: 
            sm.performMonitoringCycle()
        }
    }
}

//This function "performMonitoringCycle" executes one monitoring cycle
func (sm *SystemMonitor) performMonitoringCycle() {
    Debug("Performing monitoring cycle")

    // Update learning patterns
    sm.updateLearningData()

    // Check if checkpoint is needed 
    if sm.shouldCreateCheckpoint() {
        Debug("Checkpoint needed! - creating now")
        // Note: This would call checkpoint manager from main.go
        // For now, Just Log
        Info("Checkpoint creation triggered")

    }

    // CHECK FOR OPTIMIZATIONS
    if sm.shouldRunOptimizations() {
        Debug("Running optimization check")
        sm.checkAndApplyOptimizations()
    }
    // Perform maintenance
    if sm.shouldRunMaintenance() {
        Debug("Running maintenance tasks")

        // Note: This would call checkpoint manager from main.go
        Info("Maintenance tasks triggered")
        
    }
}

// shouldCreateCheckpoint determines if a checkpoint should be created
func (sm *SystemMonitor) shouldCreateCheckpoint() bool {
    // This function checks if enough time has passed
    timeSinceLastCheckpoint := time.Since(sm.lastCheckpoint)
    // This method gets optimal interval based on learned patterns
    optimalInterval := sm.getOptimalCheckpointInterval()

    if timeSinceLastCheckpoint < optimalInterval {
        return false 
    }

    //This method checks system resources
    if !sm.isSystemResourcesSafe() {
        Debug("System resources not safe for checkpointing")
        return false
    }

    //This method checks User Activity
    if sm.isUserInIntensiveWork() {
        Debug("User in intensive work - delay checkpoint processing")
        return false
    }

    return true 
}

// This method called getOptimalCheckpointInterval calculates optimal checkpoint interval based on learned pattern
func (sm *SystemMonitor) getOptimalCheckpointInterval() time.Duration {
    baseInterval := config.GlobalConfig.CheckpointInterval

    if !sm.workPattern.IsLearningComplete {
        return baseInterval // Use default during learning
    }

    currentHour := time.Now().Hour()

    // During work hours (learned pattern), use longer intervals
    if sm.isWorkHours(currentHour) {
        userActivity := sm.getCurrentUserActivity()
        switch userActivity {
        case ActivityIntensive:
            return baseInterval * 2 // 2 hours during intensive work
        case ActivityWorking:
            return baseInterval + 30*time.Minute // 1.5 hours during regular work
        default:
            return baseInterval
        }
    }

    return baseInterval
}

// isSystemResourcesSafe ia a method that checks if system resources can permit safe checkpointing
func (sm *SystemMonitor) isSystemResourcesSafe() bool {
    // Checks CPU usage
    cpuUsage, err := sm.getCPUUsage()
    if err != nil {
        Warn("Failed to get CPU usage:", err)
    } else if cpuUsage > 70.0 {
        Debug("High CPU usage detected:", cpuUsage, "% -  skipping checkpoint")
        return false
    }

    // Check battery level
    batteryLevel, err := sm.getBatteryLevel()
    if err != nil {
        Warn("Failed to get battery level:", err)
    } else if batteryLevel <= 15 && !sm.isPowerConnected() {
        Debug("Low battery detected:", batteryLevel, "% - skipping checkpoint")
        return false
    }

    return true
}

//This updateLearningData updates work pattern learning data
func (sm *SystemMonitor) updateLearningData() {
    if sm.workPattern.IsLearningComplete {
        return // Learning complete, no need to update
    }

    currentHour := time.Now().Hour()

    
    if cpuUsage, err := sm.getCPUUsage(); err == nil {
        sm.workPattern.CPUPatterns[currentHour] = cpuUsage
    }

    // Check if learning period is complete (1 month)
    if time.Since(sm.workPattern.LearningStartDate)>= 30*24*time.Hour {
        sm.completeLearning()
    }

    sm.saveWorkPattern()
}

// completeLearning finalizes the learning process and determines top 3 apps
func (sm *SystemMonitor) completeLearning() {
    Info("Completing 1-month learning period")

    // Find top 3 most used applications
    type appUsage struct {
        name  string
        count int
    }

    var usage []appUsage
    for appName, count := range sm.workPattern.AppUsageFrequency {
        usage = append(usage, appUsage{name: appName, count: count})
    }

    // Simple sort by usage count (bubble sort for simplicity)
    for i := 0; i < len(usage)-1; i++ {
        for j := 0; j < len(usage)-i-1; j++ {
            if usage[j].count < usage[j+1].count {
                usage[j], usage[j+1] = usage[j+1], usage[j]
            }
        }
    }

    // Select to 3
    topCount := 3
    if len(usage) < 3 {
        topCount = len(usage)
    }

    sm.workPattern.TopThreeApps = make ([]string, topCount)
    for i := 0; 1 < topCount; i++ {
        sm.workPattern.TopThreeApps[i] = usage[i].name
    }

    sm.workPattern.IsLearningComplete = true
    sm.saveWorkPattern()

    Info("Top 3 apps:", strings.Join(sm.workPattern.TopThreeApps, ", "))
}

// checkAndApplyOptimizations method checks for and applies performance optimizations
func (sm *SystemMonitor) checkAndApplyOptimizations() {
    optimizations := sm.generateOptimizations()

    for _, opt := range optimizations {
        if opt.ImprovementPercent > 20.0 {
            Info("Auto-applying optimizations:", opt.Description)
            if err := opt.Apply(); err != nil {
                Error("Failed to apply optimization:", err)
            } else {
                sm.metrics.LastOptimization = time.Now()
                sm.saveMetrics()
            }
        } else {
            Info("Optimization available:", opt.Description, "Improvement:", opt.ImprovementPercent, "%")
        }
    }
}
// Helper functions for system information

// getSystemUptime returns system uptime duration
func (sm *SystemMonitor) getSystemUptime() (time.Duration, error) {
    cmd := exec.Command("sysctl", "-n", "kern.boottime")
    output, err := cmd.Output()
    if err != nil {
        return 2 * time.Hour, err
    }

    outputStr := string(output)
    Debug("Boot time output:", outputStr)

    // Parse uptime output(simplified - real implementation would be more robust)
    return 2 * time.Hour, nil 
}   

// getCPUUsage returns current CPU usage percentage
func (sm *SystemMonitor) getCPUUsage() (float64, error) {
    // TODO: Real implementation needed
    cmd := exec.Command("top", "-l", "1", "-n", "0")
    output, err := cmd.Output()
    if err != nil {
        return 25.5, err
    }

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if strings.Contains(line, "CPU usage:") {
            // Parse CPU usage from top output
            Debug("CPU line:", line)
            // Simplified parsing - real implementation would be more robust
            return 25.5, nil // Placeholder
        }
    }

    return 25.5, nil
}

// getBatteryLevel returns current battery percentage
func (sm *SystemMonitor) getBatteryLevel() (int, error) {
    // TODO: Real implementation needed    
    cmd := exec.Command("pmset", "-g", "batt")
    output, err := cmd.Output()
    if err != nil {
        return 75, err
    }

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if strings.Contains(line, "%") {
            Debug("Battery line:", line)
            // Extract percentage (simplified)
            return 75, nil // placeholder
        }
    }

    return 75, nil
}

// isPowerConnected checks if power adapter is connected
func (sm *SystemMonitor) isPowerConnected() bool {
    cmd := exec.Command("pmset", "-g", "ps")
    output, err := cmd.Output()
    if err != nil {
        return false
    }

    return strings.Contains(string(output), "AC Power")
}

// Background loops
func (sm *SystemMonitor) heartbeatLoop() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for sm.isRunning {
        <-ticker.C
        sm.updateHeartbeat()
    }   
}

func (sm *SystemMonitor) learningLoop() {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    for sm.isRunning {
        <-ticker.C
        sm.updateLearningData()
    }
}

func (sm *SystemMonitor) updateHeartbeat() {
    sm.lastHeartbeat = time.Now()
    heartbeatFile := filepath.Join(sm.baseDir, "heartbeat")
    os.WriteFile(heartbeatFile, []byte(sm.lastHeartbeat.Format(time.RFC3339)), 0644)
}

func (sm *SystemMonitor) getLastHeartbeatTime() time.Time {
    heartbeatFile := filepath.Join(sm.baseDir, "heartbeat")
    data, err := os.ReadFile(heartbeatFile)
    if err != nil {
        return time.Time{}    
    }

    t, err := time.Parse(time.RFC3339, string(data))
    if err != nil {
        return time.Time{}
    }

    return t
}

func (sm *SystemMonitor) isFirstRun() bool {
    heartbeatFile := filepath.Join(sm.baseDir, "heartbeat")
    _, err := os.Stat(heartbeatFile)
    return os.IsNotExist(err)
}

func (sm *SystemMonitor) wasProcessRunning() bool {
    pidFile := filepath.Join(sm.baseDir, "monitor.pid")
    data, err := os.ReadFile(pidFile)
    if err != nil {
        return false
    }

    oldPID, _ := strconv.Atoi(strings.TrimSpace(string(data)))
    process, err := os.FindProcess(oldPID)
    if err != nil {
        return false
    }

    err = process.Signal(os.Signal(nil))    
    return err == nil
}

func (sm *SystemMonitor) isWorkHours(hour int) bool {
    if sm.workPattern.StartHour <= sm.workPattern.EndHour {
        return hour >= sm.workPattern.StartHour && hour <= sm.workPattern.EndHour
    }
    return hour >= sm.workPattern.StartHour || hour <= sm.workPattern.EndHour
}

func (sm *SystemMonitor) getCurrentUserActivity() UserActivity {
    return ActivityWorking
}

func (sm *SystemMonitor) isUserInIntensiveWork() bool {
    return sm.getCurrentUserActivity() == ActivityIntensive
}

func (sm *SystemMonitor) shouldRunOptimizations() bool {
    return time.Since(sm.metrics.LastOptimization) > 24*time.Hour
}

func (sm *SystemMonitor) shouldRunMaintenance() bool {
    return time.Since(sm.lastCheckpoint) > 6*time.Hour
}
// State handlers

func (sm *SystemMonitor) createInitialCheckpoint() error {
    // Placeholder for initial checkpoint creation logic}
    Info("Creating initial checkpoint...")
    return nil
}

func (sm *SystemMonitor) handleSystemRestart() error {
    // Placeholder for system restart handling logic
    Info("Handling system restart...")
    return nil
}

func (sm *SystemMonitor) updateAfterSleep() error {
    // Placeholder for updating after sleep logic
    Info("Updating after sleep...")
    sm.updateHeartbeat()
    return nil 
}

func (sm *SystemMonitor) handleCrashRecovery() error {
    Warn("Resuming normal operation")
    return nil
}

func (sm *SystemMonitor) resumeNormalOperation() error {
    Info("Resuming normal operation...")
    sm.updateHeartbeat()
    return nil
}

func (sm *SystemMonitor) stateToString(state SystemState) string {
	states := map[SystemState]string{
		StateUnknown:      "Unknown",
		StateFirstRun:     "First Run",
		StateNormal:       "Normal",
		StateSleep:        "Sleep",
		StateRestart:      "Restart",
		StateCrash:        "Crash",
		StateHighCPU:      "High CPU",
		StateLowBattery:   "Low Battery",
		StateAboutToSleep: "About to Sleep",
	}
	return states[state]
}

type Optimization struct {
    Description         string
    ImprovementPercent  float64
    Apply           func() error                                   
}

func (sm *SystemMonitor) generateOptimizations() []Optimization {
    // Implementation for optimization generation
    return []Optimization{}
}

// Persistence functions

// saveWorkPattern saves work pattern to file
func (sm *SystemMonitor) saveWorkPattern() error {
    filePath := filepath.Join(sm.baseDir, "work-pattern.json")
    data, err := json.MarshalIndent(sm.workPattern, "", " ")
    if err != nil {
        return err
    }
    return os.WriteFile(filePath, data, 0644)
}

// loadWorkPattern loads work pattern from file
func (sm *SystemMonitor) loadWorkPattern() error {
    filePath := filepath.Join(sm.baseDir, "work-pattern.json")
    data, err := os.ReadFile(filePath)
    if err != nil {
        return err 
    }

    sm.workPattern = &WorkPattern{}
    return json.Unmarshal(data, sm.workPattern)
}

func (sm *SystemMonitor) saveMetrics() error {
	filePath := filepath.Join(sm.baseDir, "metrics.json")
	data, err := json.MarshalIndent(sm.metrics, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func (sm *SystemMonitor) loadMetrics() error {
	filePath := filepath.Join(sm.baseDir, "metrics.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	sm.metrics = &OptimizationMetrics{}
	return json.Unmarshal(data, sm.metrics)
}

// Stop stops the monitoring process
func (sm *SystemMonitor) Stop() {
    Info("Stopping system monitor")
    sm.isRunning = false
}


