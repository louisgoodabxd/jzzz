package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	LogPath      = "/data/adb/srun/srun.log"
	MaxLogLines  = 500
)

type Logger struct {
	mu     sync.RWMutex
	lines  []string
	file   *os.File
	writer *bufio.Writer
}

var log *Logger

func InitLogger() error {
	dir := strings.TrimSuffix(LogPath, "/srun.log")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	log = &Logger{
		file:   f,
		writer: bufio.NewWriter(f),
	}

	// Load existing log lines for /api/logs
	log.loadExisting()
	return nil
}

func (l *Logger) loadExisting() {
	f, err := os.Open(LogPath)
	if err != nil {
		return
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// Keep only last MaxLogLines
	if len(lines) > MaxLogLines {
		lines = lines[len(lines)-MaxLogLines:]
	}
	l.lines = lines
}

func (l *Logger) log(level, msg string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("%s [%s] %s", now, level, msg)

	fmt.Println(line)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.lines = append(l.lines, line)
	if len(l.lines) > MaxLogLines {
		l.lines = l.lines[len(l.lines)-MaxLogLines:]
	}

	if l.writer != nil {
		l.writer.WriteString(line + "\n")
		l.writer.Flush()
	}
}

func (l *Logger) Info(msg string) {
	l.log("INFO", msg)
}

func (l *Logger) Error(msg string) {
	l.log("ERROR", msg)
}

func (l *Logger) Warn(msg string) {
	l.log("WARN", msg)
}

func (l *Logger) GetLines() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]string, len(l.lines))
	copy(result, l.lines)
	return result
}

func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = nil
	// Truncate log file
	if l.file != nil {
		l.file.Truncate(0)
		l.file.Seek(0, 0)
		l.writer.Reset(l.file)
	}
}

func LogInfo(msg string) {
	if log != nil {
		log.Info(msg)
	}
}

func LogError(msg string) {
	if log != nil {
		log.Error(msg)
	}
}

func LogWarn(msg string) {
	if log != nil {
		log.Warn(msg)
	}
}

func GetLogLines() []string {
	if log != nil {
		return log.GetLines()
	}
	return nil
}

func ClearLogs() {
	if log != nil {
		log.Clear()
	}
}
