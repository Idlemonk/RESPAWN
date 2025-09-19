package system

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	logFile     *os.File
	logLevel    LogLevel
	lastLogDate	string
}

var GlobalLogger *Logger


// Initialize creates and initializes the global logger 
func InitLogger() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	logDir := filepath.Join(homeDir, ".respawn", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logger := &Logger{
		logLevel: DEBUG,
	}

	if err := logger.rotateLogFile(logDir); err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	GlobalLogger = logger	
	return nil
}


// rotateLogFile creates a new log file for a current date
func (l *Logger) rotateLogFile(logDir string) error {
	currentDate := time.Now(). Format("2006-01-02")

	// Close existing log file if open and date has changed 
	if l.logFile != nil && l.lastLogDate != currentDate {
		l.logFile.Close()
		l.logFile = nil
	}

	// Create new log file if needed
	if l.logFile == nil {
		logPath := filepath.Join(logDir, "respawn.log")


		//if it's a new day, backup the old log file 
		if l.lastLogDate != "" && l.lastLogDate != currentDate {
			backupPath := filepath.Join(logDir, fmt.Sprintf("respawn-%s.log", l.lastLogDate))
			os.Rename(logPath, backupPath)
		}

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}

		l.logFile = file											
		l.lastLogDate = currentDate

		// Initialize loggers
		l.debugLogger = log.New(file, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
        l.infoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
        l.warnLogger = log.New(file, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
        l.errorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	}
	return nil 
}


// Debug logs debug messages
func Debug(v ...interface{}) {
	if GlobalLogger != nil && GlobalLogger.logLevel <= DEBUG {
		GlobalLogger.debugLogger.Println(v...)
	}
}

// Info logs info messages  
func Info(v ...interface{}) {
	if GlobalLogger != nil && GlobalLogger.logLevel <= INFO {
		GlobalLogger.infoLogger.Println(v...)
	}
}

// Warn logs warning messages
func Warn(v ...interface{}) {
	if GlobalLogger != nil && GlobalLogger.logLevel <= WARN {
		GlobalLogger.warnLogger.Println(v...)
	}
}

// Error logs error messages
func Error(v ...interface{}) {
	if GlobalLogger != nil && GlobalLogger.logLevel <= ERROR {
		GlobalLogger.errorLogger.Println(v...)	
	}
}

// Close closes the log file
func Close() {
	if GlobalLogger != nil && GlobalLogger.logFile != nil {	
		GlobalLogger.logFile.Close()
	}
}