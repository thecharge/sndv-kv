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
	logFile    *os.File
	fileWriter *bufio.Writer
	logQueue   chan string
	isInit     atomic.Bool
	minLevel   int
	baseDir    string
	mu         sync.Mutex
	stopChan   chan struct{}
	wg         sync.WaitGroup
)

const (
	DEBUG      = 0
	INFO       = 1
	ERROR      = 2
	MaxLogSize = 10 * 1024 * 1024
)

func Init(dir string, levelStr string) error {
	mu.Lock()
	defer mu.Unlock()

	if isInit.Load() {
		closeLoggers()
	}

	baseDir = dir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := openLogFile(); err != nil {
		return err
	}

	logQueue = make(chan string, 10000)
	stopChan = make(chan struct{})

	switch strings.ToUpper(levelStr) {
	case "DEBUG": minLevel = DEBUG
	case "INFO":  minLevel = INFO
	case "ERROR": minLevel = ERROR
	default:      minLevel = INFO
	}

	isInit.Store(true)
	wg.Add(1)
	go processLogs()

	return nil
}

func Shutdown() {
	mu.Lock()
	defer mu.Unlock()
	closeLoggers()
}

func closeLoggers() {
	if !isInit.Load() { return }
	
	close(stopChan)
	mu.Unlock()
	wg.Wait()
	mu.Lock()

	if fileWriter != nil { fileWriter.Flush() }
	if logFile != nil { logFile.Close() }
	
	isInit.Store(false)
}

func openLogFile() error {
	f, err := os.OpenFile(filepath.Join(baseDir, "system.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	logFile = f
	fileWriter = bufio.NewWriter(f)
	return nil
}

func IsInitialized() bool { return isInit.Load() }

func processLogs() {
	defer wg.Done()
	ticker := time.NewTicker(500 * time.Millisecond)
	byteCount := 0
	console := os.Stdout

	for {
		select {
		case msg := <-logQueue:
			if fileWriter != nil {
				n, _ := fileWriter.WriteString(msg + "\n")
				byteCount += n
			}
			fmt.Fprintln(console, msg)

			if byteCount > 1024*10 { 
				CheckRotation()
				byteCount = 0
			}
		case <-ticker.C:
			mu.Lock()
			if fileWriter != nil { fileWriter.Flush() }
			mu.Unlock()
		case <-stopChan:
			return
		}
	}
}

func CheckRotation() {
	mu.Lock()
	defer mu.Unlock()
	
	if logFile == nil { return }
	
	info, err := logFile.Stat()
	if err == nil && info.Size() > MaxLogSize {
		fileWriter.Flush()
		logFile.Close()
		
		oldName := filepath.Join(baseDir, "system.log")
		newName := oldName + "." + fmt.Sprint(time.Now().UnixNano())
		os.Rename(oldName, newName)
		
		openLogFile()
	}
}

func tryLog(prefix, format string, v ...interface{}) {
	if !isInit.Load() { return }
	ts := time.Now().Format("2006/01/02 15:04:05")
	msg := fmt.Sprintf("%s %s "+format, append([]interface{}{ts, prefix}, v...)...)
	select {
	case logQueue <- msg:
	default:
	}
}

func Access(format string, v ...interface{}) { tryLog("[ACC]", format, v...) }
func Info(format string, v ...interface{})   { if minLevel <= INFO { tryLog("[INF]", format, v...) } }
func Error(format string, v ...interface{})  { if minLevel <= ERROR { tryLog("[ERR]", format, v...) } }
func Debug(format string, v ...interface{})  { if minLevel <= DEBUG { tryLog("[DBG]", format, v...) } }