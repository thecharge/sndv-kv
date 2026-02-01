package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	globalLogFileHandle     *os.File
	globalBufferedLogWriter *bufio.Writer
	globalLogMessageQueue   chan string
	isLoggerInitialized     atomic.Bool
	minimumSeverityLevel    int
	baseLogDirectoryPath    string
	loggerMutex             sync.Mutex
	shutdownSignalChannel   chan struct{}
	backgroundWaitGroup     sync.WaitGroup
)

const (
	SeverityDebug             = 0
	SeverityInfo              = 1
	SeverityError             = 2
	MaximumLogFileSizeInBytes = 10 * 1024 * 1024 // 10 Megabytes
)

func InitializeLogger(directoryPath string, levelString string) error {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if isLoggerInitialized.Load() {
		closeAndFlushLoggerInternal()
	}

	baseLogDirectoryPath = directoryPath
	if err := os.MkdirAll(directoryPath, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	if err := openLogFileInternal(); err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	globalLogMessageQueue = make(chan string, 10000)
	shutdownSignalChannel = make(chan struct{})

	switch strings.ToUpper(levelString) {
	case "DEBUG":
		minimumSeverityLevel = SeverityDebug
	case "INFO":
		minimumSeverityLevel = SeverityInfo
	case "ERROR":
		minimumSeverityLevel = SeverityError
	default:
		minimumSeverityLevel = SeverityInfo
	}

	isLoggerInitialized.Store(true)
	backgroundWaitGroup.Add(1)
	go processLogQueueInBackground()

	return nil
}

func ShutdownLogger() {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	closeAndFlushLoggerInternal()
}

func closeAndFlushLoggerInternal() {
	if !isLoggerInitialized.Load() {
		return
	}

	close(shutdownSignalChannel)
	loggerMutex.Unlock()
	backgroundWaitGroup.Wait()
	loggerMutex.Lock()

	if globalBufferedLogWriter != nil {
		globalBufferedLogWriter.Flush()
	}
	if globalLogFileHandle != nil {
		globalLogFileHandle.Close()
	}

	isLoggerInitialized.Store(false)
}

func openLogFileInternal() error {
	filePath := filepath.Join(baseLogDirectoryPath, "system.log")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	globalLogFileHandle = file
	globalBufferedLogWriter = bufio.NewWriter(file)
	return nil
}

func IsLoggerInitialized() bool {
	return isLoggerInitialized.Load()
}

func processLogQueueInBackground() {
	defer backgroundWaitGroup.Done()
	flushTicker := time.NewTicker(500 * time.Millisecond)
	defer flushTicker.Stop()

	bytesWrittenSinceLastCheck := int64(0)
	consoleOutput := os.Stdout

	for {
		select {
		case message := <-globalLogMessageQueue:
			if globalBufferedLogWriter != nil {
				bytesWritten, _ := globalBufferedLogWriter.WriteString(message + "\n")
				bytesWrittenSinceLastCheck += int64(bytesWritten)
			}
			fmt.Fprintln(consoleOutput, message)

			if bytesWrittenSinceLastCheck > 1024*10 {
				CheckAndRotateLogFile()
				bytesWrittenSinceLastCheck = 0
			}
		case <-flushTicker.C:
			loggerMutex.Lock()
			if globalBufferedLogWriter != nil {
				globalBufferedLogWriter.Flush()
			}
			loggerMutex.Unlock()
		case <-shutdownSignalChannel:
			return
		}
	}
}

func CheckAndRotateLogFile() {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if globalLogFileHandle == nil {
		return
	}

	fileInfo, err := globalLogFileHandle.Stat()
	if err == nil && fileInfo.Size() > MaximumLogFileSizeInBytes {
		globalBufferedLogWriter.Flush()
		globalLogFileHandle.Close()

		oldFilePath := filepath.Join(baseLogDirectoryPath, "system.log")
		newFilePath := oldFilePath + "." + fmt.Sprint(time.Now().UnixNano())
		os.Rename(oldFilePath, newFilePath)

		openLogFileInternal()
	}
}

func tryQueueLogMessage(prefix string, format string, args ...interface{}) {
	if !isLoggerInitialized.Load() {
		return
	}

	timestamp := time.Now().Format("2006/01/02 15:04:05")
	formattedMessage := fmt.Sprintf("%s %s "+format, append([]interface{}{timestamp, prefix}, args...)...)

	select {
	case globalLogMessageQueue <- formattedMessage:
	default:
		// Queue full, drop message to prevent deadlock
	}
}

func LogAccessEvent(format string, args ...interface{}) {
	tryQueueLogMessage("[ACC]", format, args...)
}

func LogInfoEvent(format string, args ...interface{}) {
	if minimumSeverityLevel <= SeverityInfo {
		tryQueueLogMessage("[INF]", format, args...)
	}
}

func LogErrorEvent(format string, args ...interface{}) {
	if minimumSeverityLevel <= SeverityError {
		tryQueueLogMessage("[ERR]", format, args...)
	}
}

func LogDebugEvent(format string, args ...interface{}) {
	if minimumSeverityLevel <= SeverityDebug {
		tryQueueLogMessage("[DBG]", format, args...)
	}
}
