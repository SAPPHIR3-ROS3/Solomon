package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/docs"
)

func RunDocsSlash(d Deps, line string) error {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToLower(line), "/docs") {
		return fmt.Errorf("invalid /docs command")
	}
	rest := strings.TrimSpace(line[len("/docs"):])
	if rest == "" {
		return fmt.Errorf("usage: /docs <query>")
	}
	opts := docs.Options{
		MinNormalizedScore: config.EffectiveDocSearchMinNorm(d.Cfg),
		FullArticleScore:   config.EffectiveDocSearchFullArticleScore(d.Cfg),
	}
	res, err := docs.Retrieve(rest, opts)
	if err != nil {
		return err
	}
	apiMsg := docs.FormatSlashPayload(rest, res)
	visible := strings.TrimSpace(line)
	if d.SubmitVisibleUserMessage != nil {
		return d.SubmitVisibleUserMessage(visible, apiMsg)
	}
	if d.SubmitUserMessage == nil {
		return fmt.Errorf("/docs unavailable in this context")
	}
	return d.SubmitUserMessage(apiMsg)
}
