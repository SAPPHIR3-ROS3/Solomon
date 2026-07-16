package config

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"
)

func PrintRolesTableCatalog(out io.Writer) {
	if out == nil {
		return
	}
	fmt.Fprintln(out, "Choose up to 5 characteristics for your subagent table:")
	max := len(roles.AllCharacteristics)
	idxW := len(strconv.Itoa(max))
	for i, ch := range roles.AllCharacteristics {
		fmt.Fprintf(out, "  %*d  %s %s\n", idxW, i+1, roles.CharacteristicSymbol(ch), roles.CharacteristicLabel(ch))
	}
}

func RunRolesTableWizard(pio PromptIO, existing *Root) ([]string, error) {
	out := pio.promptOut()
	if existing != nil && len(existing.Roles.Table.Characteristics) > 0 {
		line, err := ReadPromptLine(pio, fmt.Sprintf("Roles table already set (%s). Reconfigure? [y/N]: ", strings.Join(existing.Roles.Table.Characteristics, ", ")))
		if err != nil {
			return nil, err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
		default:
			return existing.Roles.Table.Characteristics, nil
		}
	}
	PrintRolesTableCatalog(out)
	fmt.Fprintf(out, "Enter numbers separated by spaces or commas (1-%d), or skip to defer: ", len(roles.AllCharacteristics))
	line, err := readOnboardLine(pio, "")
	if err != nil {
		return nil, err
	}
	if isSkipInput(line) {
		PrintConfigSkipHint(out, "roles_table")
		return nil, ErrRolesTableSkipped
	}
	parts := parseSelectionNumbers(line)
	if len(parts) == 0 {
		return nil, fmt.Errorf("select at least one characteristic")
	}
	if len(parts) > roles.MaxTableCharacteristics {
		return nil, fmt.Errorf("at most %d characteristics", roles.MaxTableCharacteristics)
	}
	chosen := make([]string, 0, len(parts))
	seen := map[int]bool{}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > len(roles.AllCharacteristics) {
			return nil, fmt.Errorf("invalid selection %q", p)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		chosen = append(chosen, roles.AllCharacteristics[n-1])
	}
	if err := roles.ValidateTableCharacteristics(chosen); err != nil {
		return nil, err
	}
	return chosen, nil
}

func parseSelectionNumbers(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	line = strings.NewReplacer(",", " ", ";", " ").Replace(line)
	return strings.Fields(line)
}

func EnsureRolesTable(pio PromptIO, cfg *Root) error {
	if cfg == nil {
		return fmt.Errorf("config unavailable")
	}
	if len(cfg.Roles.Table.Characteristics) > 0 {
		return nil
	}
	chosen, err := RunRolesTableWizard(pio, cfg)
	if err != nil {
		return err
	}
	cfg.Roles.Table.Characteristics = chosen
	return nil
}
