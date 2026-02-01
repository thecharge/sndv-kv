package logger

import (
	"os"
	"testing"
	"time"
)

func TestLoggerInitialization(t *testing.T) {
	testDir := "./test_logs_init"
	os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	if err := InitializeLogger(testDir, "INFO"); err != nil {
		t.Fatalf("InitializeLogger failed: %v", err)
	}

	if !IsLoggerInitialized() {
		t.Error("Logger should be initialized")
	}

	LogInfoEvent("Test Message %d", 1)

	// Allow async write
	time.Sleep(100 * time.Millisecond)

	logPath := testDir + "/system.log"
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("system.log not created")
	}
}

func TestManualLogRotationTrigger(t *testing.T) {
	testDir := "./test_logs_manual_rot"
	os.RemoveAll(testDir)
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	InitializeLogger(testDir, "INFO")
	LogInfoEvent("Pre-rotation")
	time.Sleep(50 * time.Millisecond)

	stat1, _ := os.Stat(testDir + "/system.log")

	// Explicit shutdown to release lock for test manipulation on Windows
	ShutdownLogger()

	// Force file size condition by writing junk
	f, _ := os.OpenFile(testDir+"/system.log", os.O_APPEND|os.O_WRONLY, 0644)
	bigData := make([]byte, 11*1024*1024)
	f.Write(bigData)
	f.Close()

	InitializeLogger(testDir, "INFO")
	CheckAndRotateLogFile() // Manual trigger

	stat2, _ := os.Stat(testDir + "/system.log")

	fi, _ := os.Stat(testDir + "/system.log")
	if fi.Size() > 10*1024*1024 {
		t.Error("File did not rotate, size is still large")
	}

	// On successful rotation, the new file (stat2) should be a different file instance than the old one (stat1)
	// or at minimum, the size is reset.
	if os.SameFile(stat1, stat2) && fi.Size() > 1024 {
		t.Error("Log file handle/identity did not change appropriately")
	}
}
