package logging

import (
	"bufio"
	f "fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const RED = "\033[91m"
const GREEN = "\033[92m"
const YELLOW = "\033[93m"
const MAGENTA = "\033[95m"
const CYAN = "\033[96m"
const GRAY = "\033[90m"
const END = "\033[0m"

const ERROR_LOG_LEVEL = 0
const WARNING_LOG_LEVEL = 1
const INFO_LOG_LEVEL = 2
const DEBUG_LOG_LEVEL = 3
const RESULT_LOG_LEVEL = 4

var GLOBAL_LOG_LEVEL = -1
var globalLogLevelInitialized = false

var globalWriteConsole = true
var globalWriteFile = false
var globalLogDir = ""
var globalRetentionDays = 7

var redText = func(text string) string { return RED + text + END }
var greenText = func(text string) string { return GREEN + text + END }
var yellowText = func(text string) string { return YELLOW + text + END }
var magentaText = func(text string) string { return MAGENTA + text + END }
var cyanText = func(text string) string { return CYAN + text + END }
var grayText = func(text string) string { return GRAY + text + END }

var logLevel = map[int]string{
	ERROR_LOG_LEVEL:   "ERROR",
	WARNING_LOG_LEVEL: "WARNING",
	INFO_LOG_LEVEL:    "INFO",
	DEBUG_LOG_LEVEL:   "DEBUG",
	RESULT_LOG_LEVEL:  "RESULT",
}

var logColor = map[int]func(string) string{
	ERROR_LOG_LEVEL:   redText,
	WARNING_LOG_LEVEL: yellowText,
	INFO_LOG_LEVEL:    greenText,
	DEBUG_LOG_LEVEL:   magentaText,
	RESULT_LOG_LEVEL:  cyanText,
}

var logMu sync.Mutex

type Config struct {
	Dir          string
	WriteConsole bool
	WriteFile    bool
	Retention    int
}

type LogOptions struct {
	Params       map[string]any
	Override     bool
	WriteConsole *bool
	WriteFile    *bool
}

func LogInit(globalLogLevel ...int) {
	var defaultLogLevel int

	if len(globalLogLevel) > 0 {
		defaultLogLevel = globalLogLevel[0]
	} else {
		defaultLogLevel = INFO_LOG_LEVEL
	}

	if defaultLogLevel < ERROR_LOG_LEVEL || defaultLogLevel > RESULT_LOG_LEVEL {
		panic("Invalid log level")
	}

	logMu.Lock()
	GLOBAL_LOG_LEVEL = defaultLogLevel
	globalLogLevelInitialized = true
	logMu.Unlock()
}

func SetGlobalLevel(level int) error {
	if level < ERROR_LOG_LEVEL || level > RESULT_LOG_LEVEL {
		return f.Errorf("invalid log level")
	}
	logMu.Lock()
	if !globalLogLevelInitialized {
		logMu.Unlock()
		return f.Errorf("global log level not initialized")
	}
	GLOBAL_LOG_LEVEL = level
	logMu.Unlock()
	return nil
}

func ParseLevel(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error":
		return ERROR_LOG_LEVEL, nil
	case "warning":
		return WARNING_LOG_LEVEL, nil
	case "info":
		return INFO_LOG_LEVEL, nil
	case "debug":
		return DEBUG_LOG_LEVEL, nil
	case "result":
		return RESULT_LOG_LEVEL, nil
	default:
		return 0, f.Errorf("unknown level %q (use error, warning, info, debug, result)", s)
	}
}

func LevelLabel(level int) string {
	if s, ok := logLevel[level]; ok {
		return s
	}
	return "UNKNOWN"
}

func Configure(config Config) error {
	retention := config.Retention
	if retention <= 0 {
		retention = 7
	}

	if config.WriteFile && config.Dir == "" {
		return f.Errorf("log directory not configured")
	}

	if config.WriteFile {
		if err := os.MkdirAll(config.Dir, 0o700); err != nil {
			return err
		}
	}

	logMu.Lock()
	globalLogDir = config.Dir
	globalWriteConsole = config.WriteConsole
	globalWriteFile = config.WriteFile
	globalRetentionDays = retention
	logMu.Unlock()

	return nil
}

func Log(currentLogLevel int, message string, options ...LogOptions) {
	var logOptions LogOptions
	var parameters map[string]any

	logMu.Lock()
	if !globalLogLevelInitialized {
		logMu.Unlock()
		panic("Global log level not initialized")
	}

	localLogLevel := GLOBAL_LOG_LEVEL
	writeConsole := globalWriteConsole
	writeFile := globalWriteFile
	logDir := globalLogDir
	retention := globalRetentionDays
	logMu.Unlock()

	if _, exists := logLevel[currentLogLevel]; !exists {
		panic("Invalid log level")
	}

	if len(options) > 0 {
		logOptions = options[0]

		if logOptions.Override {
			localLogLevel = currentLogLevel
		}
		if logOptions.Params != nil {
			parameters = logOptions.Params
		}
		if logOptions.WriteConsole != nil {
			writeConsole = *logOptions.WriteConsole
		}
		if logOptions.WriteFile != nil {
			writeFile = *logOptions.WriteFile
		}
	}

	if currentLogLevel > localLogLevel {
		return
	}

	date := time.Now().Format(time.RFC3339)
	programCounter, filename, line, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(programCounter).Name()
	file := filepath.Base(filename)

	if caller == "main.main" {
		caller = "main"
	}

	consoleMessage := renderMessage(date, currentLogLevel, file, line, caller, message, parameters, true)
	fileMessage := renderMessage(date, currentLogLevel, file, line, caller, message, parameters, false)

	logMu.Lock()
	defer logMu.Unlock()

	if writeConsole {
		f.Println(consoleMessage)
	}

	if writeFile {
		_ = writeFileLog(logDir, fileMessage, retention)
	}
}

func ReadLast(n int) ([]string, error) {
	if n <= 0 {
		n = 10
	}

	logMu.Lock()
	dir := globalLogDir
	logMu.Unlock()

	if dir == "" {
		return []string{}, nil
	}

	path := filepath.Join(dir, time.Now().Format("2006-01-02")+".log")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(lines) <= n {
		return lines, nil
	}

	return lines[len(lines)-n:], nil
}

func renderMessage(date string, currentLogLevel int, file string, line int, caller string, message string, parameters map[string]any, colored bool) string {
	color := logColor[currentLogLevel]
	if !colored {
		color = func(text string) string { return text }
	}

	level := color(logLevel[currentLogLevel])
	fileValue := color(file)
	callerValue := color(caller)
	messageValue := color(message)

	if parameters != nil {
		paramsColor := grayText
		if !colored {
			paramsColor = func(text string) string { return text }
		}
		params := paramsColor(f.Sprintf("| %v", parameters))
		return f.Sprintf("[%s][%s][%s][%s] %s %s", date, level, color(f.Sprintf("%s:%d", file, line)), callerValue, messageValue, params)
	}

	return f.Sprintf("[%s][%s][%s][%s] %s", date, level, fileValue, callerValue, messageValue)
}

func writeFileLog(dir string, message string, retention int) error {
	if dir == "" {
		return nil
	}

	path := filepath.Join(dir, time.Now().Format("2006-01-02")+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(message + "\n"); err != nil {
		return err
	}

	return rotate(dir, retention)
}

func rotate(dir string, retention int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".log" {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	if len(files) <= retention {
		return nil
	}

	sort.Strings(files)
	for _, path := range files[:len(files)-retention] {
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	return nil
}
