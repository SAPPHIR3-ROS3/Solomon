package server

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnv loads .env files relevant to the server command. Existing
// process variables always win, so a shell export remains an explicit
// override. The current directory matches Makefile behavior; dev mode also
// searches upward from the GUI directory so the command works outside the
// repository root.
func loadDotEnv(devDir string) error {
	paths := make([]string, 0, 8)
	seen := make(map[string]bool)
	addPath := func(path string) {
		path = filepath.Clean(path)
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}

	if workingDirectory, err := os.Getwd(); err == nil {
		addPath(filepath.Join(workingDirectory, ".env"))
	}
	if devDir != "" {
		for directory := filepath.Clean(devDir); ; directory = filepath.Dir(directory) {
			addPath(filepath.Join(directory, ".env"))
			parent := filepath.Dir(directory)
			if parent == directory {
				break
			}
		}
	}

	for _, path := range paths {
		if err := loadDotEnvFile(path); err != nil {
			return err
		}
	}
	return nil
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		separator := strings.IndexByte(line, '=')
		if separator <= 0 {
			return fmt.Errorf("invalid .env entry in %s at line %d", path, lineNumber)
		}
		key := strings.TrimSpace(line[:separator])
		if !validDotEnvKey(key) {
			return fmt.Errorf("invalid .env key %q in %s at line %d", key, path, lineNumber)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value := strings.TrimSpace(line[separator+1:])
		value = unquoteDotEnvValue(value)
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set .env key %q: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func validDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for index, character := range key {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func unquoteDotEnvValue(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	if comment := strings.Index(value, " #"); comment >= 0 {
		return strings.TrimSpace(value[:comment])
	}
	return value
}
