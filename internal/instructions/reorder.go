package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func ReorderRules(scope, projHex string, numbers []int) ([]RuleEntry, error) {
	dir, err := rulesDir(scope, projHex)
	if err != nil {
		return nil, err
	}
	entries, err := ListRules(scope, projHex)
	if err != nil {
		return nil, err
	}
	if len(entries) != len(numbers) {
		return nil, fmt.Errorf("rule order does not match the current rules")
	}
	byNumber := make(map[int]RuleEntry, len(entries))
	for _, entry := range entries {
		byNumber[entry.Number] = entry
	}
	seen := make(map[int]struct{}, len(numbers))
	for _, number := range numbers {
		if _, ok := byNumber[number]; !ok {
			return nil, fmt.Errorf("rule order does not match the current rules")
		}
		if _, ok := seen[number]; ok {
			return nil, fmt.Errorf("duplicate rule number %d", number)
		}
		seen[number] = struct{}{}
	}
	suffix := fmt.Sprintf(".reorder-%d-%d", os.Getpid(), time.Now().UnixNano())
	temporary := make(map[int]string, len(entries))
	for _, entry := range entries {
		path := entry.FilePath + suffix
		if err := os.Rename(entry.FilePath, path); err != nil {
			return nil, err
		}
		temporary[entry.Number] = path
	}
	for index, number := range numbers {
		entry := byNumber[number]
		destination := filepath.Join(dir, ruleFileName(index+1, entry.Enabled))
		if err := os.Rename(temporary[number], destination); err != nil {
			return nil, err
		}
	}
	return ListRules(scope, projHex)
}
