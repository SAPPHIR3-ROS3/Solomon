package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
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
