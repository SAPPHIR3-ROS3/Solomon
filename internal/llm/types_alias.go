package llm

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"

type Protocol = apitype.Protocol

const (
	ProtocolOpenAI    = apitype.ProtocolOpenAI
	ProtocolAnthropic = apitype.ProtocolAnthropic
)

type ToolDef = apitype.ToolDef
type TurnRequest = apitype.TurnRequest
type SimpleCompletionRequest = apitype.SimpleCompletionRequest
type AssistantToolCall = apitype.AssistantToolCall
type UsageStats = apitype.UsageStats
type AssistantTurnResult = apitype.AssistantTurnResult
type StreamOpts = apitype.StreamOpts
type CompletionBackend = apitype.CompletionBackend
