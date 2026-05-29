package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func Skills(d Deps) error {
	return skills.ListInstalledSkills(d.Out, d.ProjHex, d.ProjRoot)
}

func RunSkillSlash(d Deps, e skills.SkillEntry) error {
	text, err := skills.SkillInputPrefillText(e)
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
	msg, err := skills.SkillUserMessagePayload(e)
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
	apiMsg, err := skills.ForcedSkillUserMessagePayload(*e, remainder)
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
