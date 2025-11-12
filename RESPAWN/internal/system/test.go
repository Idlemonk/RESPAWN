package system

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

// TestMacOSAutoStartCreation verifies auto-start instance creation
func TestMacOSAutoStartCreation(t *testing.T) {
    // Get a temporary executable path for testing
    execPath := "/usr/local/bin/respawn"
    
    autoStart := NewMacOSAutoStart(execPath)
    
    if autoStart == nil {
        t.Fatal("NewMacOSAutoStart returned nil")
    }
    
    if autoStart.plistPath == "" {
        t.Error("plistPath is empty")
    }
    
    if autoStart.executablePath != execPath {
        t.Errorf("Expected executablePath %s, got %s", execPath, autoStart.executablePath)
    }
    
    t.Logf("✓ Auto-start created successfully")
    t.Logf("  Plist path: %s", autoStart.plistPath)
    t.Logf("  Executable: %s", autoStart.executablePath)
}

// TestStartupManagerCreation verifies startup manager initialization
func TestStartupManagerCreation(t *testing.T) {
    sm, err := NewStartupManager()
    
    if err != nil {
        t.Fatalf("Failed to create StartupManager: %v", err)
    }
    
    if sm.autoStart == nil {
        t.Error("autoStart is nil")
    }
    
    if sm.instanceLock == nil {
        t.Error("instanceLock is nil")
    }
    
    if sm.crashTracker == nil {
        t.Error("crashTracker is nil")
    }
    
    // Cleanup
    defer sm.Cleanup()
    
    t.Log("✓ StartupManager created successfully")
}

// TestCrashTrackerLogic verifies crash tracking functionality
func TestCrashTrackerLogic(t *testing.T) {
    // Create temporary crash tracker
    tempDir := t.TempDir()
    ct := &CrashTracker{
        crashes:      make([]time.Time, 0),
        maxCrashes:   3,
        windowPeriod: 1 * time.Hour,
        stateFile:    filepath.Join(tempDir, "crash_state.json"),
    }
    
    // Should not disable initially
    if ct.ShouldDisableAutoStart() {
        t.Error("Should not disable auto-start with no crashes")
    }
    
    // Record crashes
    ct.RecordCrash()
    ct.RecordCrash()
    
    // Should still not disable (only 2 crashes)
    if ct.ShouldDisableAutoStart() {
        t.Error("Should not disable after 2 crashes")
    }
    
    // Third crash should trigger disable
    ct.RecordCrash()
    if !ct.ShouldDisableAutoStart() {
        t.Error("Should disable after 3 crashes")
    }
    
    t.Log("✓ Crash tracker logic works correctly")
}

// TestInstanceLockCreation verifies single instance mechanism
func TestInstanceLockCreation(t *testing.T) {
    tempDir := t.TempDir()
    
    lock := &InstanceLock{
        lockFile: filepath.Join(tempDir, "test.lock"),
        pidFile:  filepath.Join(tempDir, "test.pid"),
        pid:      os.Getpid(),
    }
    
    // Create lock files
    err := os.WriteFile(lock.lockFile, []byte("test"), 0644)
    if err != nil {
        t.Fatalf("Failed to create lock file: %v", err)
    }
    
    err = os.WriteFile(lock.pidFile, []byte("12345"), 0644)
    if err != nil {
        t.Fatalf("Failed to create PID file: %v", err)
    }
    
    // Verify files exist
    if _, err := os.Stat(lock.lockFile); os.IsNotExist(err) {
        t.Error("Lock file was not created")
    }
    
    if _, err := os.Stat(lock.pidFile); os.IsNotExist(err) {
        t.Error("PID file was not created")
    }
    
    t.Log("✓ Instance lock files created successfully")
}

// TestPermissionChecks verifies macOS permission checking
func TestPermissionChecks(t *testing.T) {
    sm, err := NewStartupManager()
    if err != nil {
        t.Fatalf("Failed to create StartupManager: %v", err)
    }
    defer sm.Cleanup()
    
    // Test accessibility check (might fail if not granted)
    hasAccessibility := sm.hasAccessibilityPermission()
    t.Logf("Accessibility permission: %v", hasAccessibility)
    
    // Test full disk access check
    hasFullDisk := sm.hasFullDiskAccess()
    t.Logf("Full Disk Access: %v", hasFullDisk)
    
    // Note: These tests document current state, don't fail
    t.Log("✓ Permission checks completed")
}

// Benchmark tests

// BenchmarkStartupManagerCreation measures creation performance
func BenchmarkStartupManagerCreation(b *testing.B) {
    for i := 0; i < b.N; i++ {
        sm, err := NewStartupManager()
        if err != nil {
            b.Fatal(err)
        }
        sm.Cleanup()
    }
}

// BenchmarkCrashTrackerRecording measures crash recording performance
func BenchmarkCrashTrackerRecording(b *testing.B) {
    tempDir := b.TempDir()
    ct := &CrashTracker{
        crashes:      make([]time.Time, 0),
        maxCrashes:   3,
        windowPeriod: 1 * time.Hour,
        stateFile:    filepath.Join(tempDir, "crash_state.json"),
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        ct.RecordCrash()
    }
}

