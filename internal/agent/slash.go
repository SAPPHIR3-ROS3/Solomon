package agent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
)

var ErrExitChat = errors.New("exit chat")

func splitSlashArgs(line string) []string {
	line = strings.TrimSpace(line)
	var fields []string
	for len(line) > 0 {
		if line[0] == '"' {
			line = line[1:]
			var b strings.Builder
			for len(line) > 0 {
				if line[0] == '\\' && len(line) > 1 {
					b.WriteByte(line[1])
					line = line[2:]
					continue
				}
				if line[0] == '"' {
					line = line[1:]
					break
				}
				b.WriteByte(line[0])
				line = line[1:]
			}
			fields = append(fields, b.String())
			line = strings.TrimLeft(line, " \t")
			continue
		}
		i := strings.IndexAny(line, " \t")
		if i < 0 {
			fields = append(fields, line)
			break
		}
		fields = append(fields, line[:i])
		line = strings.TrimLeft(line[i:], " \t")
	}
	return fields
}

func slashCommandName(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	t := strings.TrimSpace(parts[0])
	t = strings.TrimPrefix(t, "/")
	t = strings.TrimSpace(t)
	t = strings.Trim(t, "\ufeff\u200b\u200c\u200d")
	return strings.ToLower(t)
}

func SlashDispatch(d commands.Deps, line string) error {
	parts := splitSlashArgs(line)
	if len(parts) == 0 {
		return nil
	}
	name := slashCommandName(parts)
	if name != "" {
		logging.Log(logging.INFO_LOG_LEVEL, "slash dispatch", logging.LogOptions{Params: map[string]any{"command": name}})
	}
	if strings.HasPrefix(name, "skill:") {
		if err := commands.RunForcedSkillSlash(d, strings.TrimSpace(line)); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "forced skill command failed", logging.LogOptions{Params: map[string]any{"command": name, "err": err.Error()}})
			return err
		}
		return nil
	}
	ok, err := commands.DispatchBuiltinSlash(d, parts, name)
	if ok {
		if errors.Is(err, commands.ErrBuiltinExitChat) {
			return ErrExitChat
		}
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "slash command failed", logging.LogOptions{Params: map[string]any{"command": name, "err": err.Error()}})
		}
		return err
	}
	e, skillErr := skills.LookupSkillBySlashCommand(name, d.ProjHex, d.ProjRoot)
	if skillErr != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "slash skill lookup failed", logging.LogOptions{Params: map[string]any{"command": name, "err": skillErr.Error()}})
		return skillErr
	}
	if e != nil {
		if err := commands.RunSkillSlash(d, *e); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "slash skill command failed", logging.LogOptions{Params: map[string]any{"command": name, "skill": e.Name, "err": err.Error()}})
			return err
		}
		return nil
	}
	err = fmt.Errorf("unknown command /%s (try /help)", name)
	logging.Log(logging.WARNING_LOG_LEVEL, "unknown slash command", logging.LogOptions{Params: map[string]any{"command": name}})
	return err
}
