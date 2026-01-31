package logger

import (
	"os"
	"testing"
	"time"
)

func TestLoggerInit(t *testing.T) {
	dir := "./test_logs"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	if err := Init(dir, "INFO"); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !IsInitialized() {
		t.Error("Logger should be initialized")
	}

	Info("Test Log Message %d", 1)
	time.Sleep(10 * time.Millisecond)

	if _, err := os.Stat(dir + "/system.log"); os.IsNotExist(err) {
		t.Error("system.log not created")
	}
}

func TestManualRotationTrigger(t *testing.T) {
	dir := "./test_logs_manual_rot"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	Init(dir, "INFO")
	Info("Pre-rotation")
	time.Sleep(10 * time.Millisecond)

	stat1, _ := os.Stat(dir + "/system.log")

	// Close to allow manipulation on Windows
	Shutdown()

	// Force large file condition
	f, _ := os.OpenFile(dir+"/system.log", os.O_APPEND|os.O_WRONLY, 0644)
	bigData := make([]byte, 11*1024*1024)
	f.Write(bigData)
	f.Close()

	Init(dir, "INFO")
	CheckRotation() // Manual trigger

	stat2, _ := os.Stat(dir + "/system.log")

	// If rotated, the new system.log will be small (empty), stat1 was small.
	// If FAILED, system.log would still be huge (same file).
	// We check if it is NOT the huge file we just wrote.
	fi, _ := os.Stat(dir + "/system.log")
	if fi.Size() > 10*1024*1024 {
		t.Error("File did not rotate")
	}

	if os.SameFile(stat1, stat2) {
		// Technically different inodes usually, but size check is robust
	}
}
