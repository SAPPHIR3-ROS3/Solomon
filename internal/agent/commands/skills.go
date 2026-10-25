package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func Skills(d Deps) error {
	return skills.ListInstalledSkills(d.Out, d.ProjHex, d.ProjRoot)
}

// RunSkillSlash runs "/<skill> [request]"; a request typed after the command is sent with the skill.
func RunSkillSlash(d Deps, e skills.SkillEntry, line string) error {
	var notify *atmention.Notifier
	if d.AtIncludeNotifier != nil {
		notify = d.AtIncludeNotifier()
	}
	line = strings.TrimSpace(line)
	if i := strings.IndexAny(line, " \t\n"); i >= 0 && strings.TrimSpace(line[i:]) != "" {
		request := line[i:]
		apiMsg, err := skills.ForcedSkillUserMessagePayload(e, request, d.ProjRoot, notify)
		if err != nil {
			return err
		}
		if d.SubmitVisibleUserMessage != nil {
			return d.SubmitVisibleUserMessage(line, apiMsg)
		}
		if d.SubmitUserMessage == nil {
			return fmt.Errorf("skill command unavailable in this context")
		}
		return d.SubmitUserMessage(apiMsg)
	}
	text, err := skills.SkillUserMessagePayload(e, d.ProjRoot, notify)
	if err != nil {
		return err
	}
	if d.PrefillInput != nil {
		d.PrefillInput(text)
		return nil
	}
	if d.SubmitUserMessage == nil {
		return fmt.Errorf("skill command unavailable in this context")
	}
	msg, err := skills.SkillUserMessagePayload(e, d.ProjRoot, notify)
	if err != nil {
		return err
	}
	return d.SubmitUserMessage(msg)
}

func RunForcedSkillSlash(d Deps, line string) error {
	e, _, remainder, err := skills.ResolveForcedSkillCommand(line, d.ProjHex, d.ProjRoot)
	if err != nil {
		return err
	}
	var notify *atmention.Notifier
	if d.AtIncludeNotifier != nil {
		notify = d.AtIncludeNotifier()
	}
	apiMsg, err := skills.ForcedSkillUserMessagePayload(*e, remainder, d.ProjRoot, notify)
	if err != nil {
		return err
	}
	visible := strings.TrimSpace(line)
	if d.SubmitVisibleUserMessage != nil {
		return d.SubmitVisibleUserMessage(visible, apiMsg)
	}
	if d.SubmitUserMessage == nil {
		return fmt.Errorf("skill command unavailable in this context")
	}
	return d.SubmitUserMessage(apiMsg)
}
