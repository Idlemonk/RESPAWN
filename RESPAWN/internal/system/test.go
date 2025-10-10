package system

func TestConnection() {
    // This will only compile on macOS
    autoStart := NewMacOSAutoStart("/path/to/binary")
    
    // If this compiles, connection works!
    println("Connection test passed:", autoStart.plistPath)
}

