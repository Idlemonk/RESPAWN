//go:build darwin

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

type MacOSAutoStart struct {
    executablePath string
    plistPath      string
}

const launchAgentPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.respawn.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExecutablePath}}</string>
        <string>--start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
        <key>Crashed</key>
        <true/>
    </dict>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}/respawn_stdout.log</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}/respawn_stderr.log</string>
</dict>
</plist>`

func NewMacOSAutoStart(execPath string) *MacOSAutoStart {
	homeDir, _ := os.UserHomeDir()
	plistPath := filepath.Join(homeDir, "Library/LaunchAgents/com.respawn.agent.plist")

	return &MacOSAutoStart{
		executablePath: execPath,
		plistPath:  	plistPath,
	}
}

func (m *MacOSAutoStart) Install() error {
	Debug("Installing macOS LaunchAgent")

	//Ensure LaunchAgents directory exists
	launchAgentsDir := filepath.Dir(m.plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("Failed to create LaunchAgents directory: %w", err)
	}

	// Create plist file from template
	tmpl, err := template.New("plist").Parse(launchAgentPlistTemplate)
	if err != nil {
		return fmt.Errorf("Failed to parse plist template: %w", err)
	}

	file, err := os.Create(m.plistPath)
	if err != nil {
		return fmt.Errorf("Failed to create plist file: %w", err)
	}
	defer file.Close()

	homeDir, _ := os.UserHomeDir()
	logPath := filepath.Join(homeDir, ".respawn/logs")

	data := struct {
		ExecutablePath  string
		LogPath         string
	}{
		ExecutablePath: m.executablePath,
		LogPath: 		logPath,
	}

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("Failed to write plist file: %w", err)
	}

	Debug("LaunchAgent plist created at:", m.plistPath)
	return nil
}

func (m *MacOSAutoStart) Uninstall() error {
	Debug("Uninstalling macOS LunchAgent")

	// Unload first if loaded
	m.Disable()

	// Remove plist file
	if err := os.Remove(m.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to remover plist file: %w", err)
	}

	Debug("LaunchAgent plist removed")
	return nil
}

func (m *MacOSAutoStart) Enable() error {
	Debug("Enabling macOS LaunchAgent")

	// Load the LaunchAgent
	cmd := exec.Command("launchctl", "load", m.plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to load LaunchAgent: %w (output: %s)", err, string(output))
	}

	Debug("LaunchAgent loaded successfully")
	return nil
}

func (m *MacOSAutoStart) Disable() error {
	Debug("Disabling macOS LaunchAgent")

	// Unload the LaunchAgent
	cmd := exec.Command("launchctl", "unload", m.plistPath)
	cmd.Run() //Ignore errors - might not be loaded

	Debug("LaunchAgent unloaded")
	return nil
}

func (m *MacOSAutoStart) IsInstalled() bool {
	_, err := os.Stat(m.plistPath)
	return err == nil
}

func (m *MacOSAutoStart) IsEnabled() bool {
	// Check if LaunchAgent is loaded
	cmd := exec.Command("launchctl", "list", "com.respawn.agent")
	err := cmd.Run()
	return err == nil
}




































































































































