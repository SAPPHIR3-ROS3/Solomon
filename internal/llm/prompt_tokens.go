package llm

import (
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
)

func CountTurnPrompt(req TurnRequest) int64 {
	msgs := MessageParams(req.System, req.Messages, req.ImageFiles)
	tools := openaiToolsFromDefs(req.Tools)
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = tokcount.DefaultModel
	}
	return tokcount.CountOpenAIChatCompletion(msgs, tools, model)
}
