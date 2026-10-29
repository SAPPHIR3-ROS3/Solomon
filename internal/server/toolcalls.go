package server

import (
	"encoding/json"
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"sort"
	"strings"
)

func apiStatsFromMessage(message chatstore.Message) *apiStats {
	if message.Provider == "" && message.Model == "" && message.ReasoningEffort == "" && !message.CacheUsageTracked && !message.TurnDisplaySaved && message.PromptTokens == 0 && message.CachedPromptTokens == 0 && message.CacheCreationPromptTokens == 0 && message.UserPromptTokens == 0 && message.ReasoningTokens == 0 && message.ResponseTokens == 0 && message.TurnTotalTokens == 0 && message.TurnWallSecs == 0 {
		return nil
	}
	contextTokens := message.TurnContextTokens
	if contextTokens == 0 {
		contextTokens = message.PromptTokens
	}
	userTokens := message.TurnUserTokens
	if userTokens == 0 {
		userTokens = message.UserPromptTokens
	}
	cachedPromptTokens := chatstore.CacheTokenCount(message.CachedPromptTokens, message.CacheUsageTracked)
	if message.TurnDisplaySaved && (message.CacheUsageTracked || message.TurnCachedPromptTokens != 0) {
		cachedPromptTokens = message.TurnCachedPromptTokens
	}
	cacheCreationPromptTokens := chatstore.CacheTokenCount(message.CacheCreationPromptTokens, message.CacheUsageTracked)
	if message.TurnDisplaySaved && (message.CacheUsageTracked || message.TurnCacheCreationPromptTokens != 0) {
		cacheCreationPromptTokens = message.TurnCacheCreationPromptTokens
	}
	reasoningTokens := message.TurnReasonTokens
	if reasoningTokens == 0 {
		reasoningTokens = message.ReasoningTokens
	}
	responseTokens := message.TurnRespTokens
	if responseTokens == 0 {
		responseTokens = message.ResponseTokens
	}
	totalTokens := message.TurnTotalDisplay
	if totalTokens == 0 {
		totalTokens = message.TurnTotalTokens
	}
	outputTPS := message.TurnOutputTPS
	if outputTPS == 0 {
		outputTPS = message.OutputTPS
	}
	ttft := message.TurnTTFTSecs
	if ttft == 0 {
		ttft = message.TTFTSecs
	}
	promptTPS := message.TurnPromptTPS
	if promptTPS == 0 {
		promptTPS = message.PromptTPS
	}
	return &apiStats{
		TurnAggregate: message.TurnDisplaySaved,
		Provider:      message.Provider, Model: message.Model, ReasoningEffort: message.ReasoningEffort,
		ContextTokens: contextTokens, CachedPromptTokens: cachedPromptTokens, CacheCreationPromptTokens: cacheCreationPromptTokens,
		OutputTokensPerSecond: outputTPS, PromptTokensPerSecond: promptTPS,
		ReasoningTokens: reasoningTokens, ResponseTokens: responseTokens, TotalTokens: totalTokens,
		TTFTSeconds: ttft, UserTokens: userTokens,
	}
}

func apiStatsForMessageTurn(message chatstore.Message, source []chatstore.Message, index int) *apiStats {
	stats := apiStatsFromMessage(message)
	if stats == nil || !message.TurnDisplaySaved || index < 0 || index >= len(source) {
		return stats
	}
	start := 0
	for i := index - 1; i >= 0; i-- {
		candidate := source[i]
		if candidate.Role == "user" && !candidate.GUIHidden && !candidate.CreatedAt.IsZero() &&
			!strings.HasPrefix(strings.TrimSpace(candidate.Content), "tool_result(") {
			start = i + 1
			break
		}
	}
	contextTok, userTok, reasonTok, responseTok, totalTok, cachedTok, cacheWriteTok,
		outputTPS, ttftSecs, promptTPS, _, _, ok := chatstore.StoredUsageLineForTurnRange(source, start, index+1)
	if !ok {
		return stats
	}
	stats.ContextTokens = contextTok
	stats.UserTokens = userTok
	stats.ReasoningTokens = reasonTok
	stats.ResponseTokens = responseTok
	stats.TotalTokens = totalTok
	stats.CachedPromptTokens = cachedTok
	stats.CacheCreationPromptTokens = cacheWriteTok
	stats.OutputTokensPerSecond = outputTPS
	stats.TTFTSeconds = ttftSecs
	stats.PromptTokensPerSecond = promptTPS
	return stats
}

func apiToolCallsFromMessage(projectID, parentChatID string, messages []chatstore.Message, messageIndex int, runActive bool, includeNested bool) []apiToolCall {
	message := messages[messageIndex]
	if len(message.ToolCalls) == 0 {
		return nil
	}
	results := make(map[string]chatstore.Message)
	for _, candidate := range messages[messageIndex+1:] {
		if candidate.Role == "user" {
			break
		}
		if candidate.Role == "tool" && candidate.ToolCallID != "" {
			results[candidate.ToolCallID] = candidate
		}
	}
	var subchatSessions []*chatstore.SubSession
	subchatSessionsLoaded := false
	findSubchat := func(parentToolCallID string) *chatstore.SubSession {
		if strings.TrimSpace(parentChatID) == "" || strings.TrimSpace(parentToolCallID) == "" {
			return nil
		}
		if !subchatSessionsLoaded {
			subchatSessionsLoaded = true
			subchatSessions, _ = chatstore.ListSubSessions(projectID)
		}
		for _, subchat := range subchatSessions {
			if subchat == nil || subchat.ParentChatID != parentChatID || subchat.ParentToolCallID != parentToolCallID {
				continue
			}
			if subchat.ProjectHex != "" && subchat.ProjectHex != projectID {
				continue
			}
			return subchat
		}
		return nil
	}
	out := make([]apiToolCall, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		args := decodeArguments(call.Arguments)
		tool := apiToolCall{
			ID:         call.ID,
			Name:       call.Name,
			Input:      toolInput(call.Name, args, call.Arguments),
			Intent:     stringField(args, "intent"),
			Mode:       stringField(args, "mode"),
			NewString:  stringField(args, "newString"),
			OldString:  stringField(args, "oldString"),
			RenameTo:   stringField(args, "renameTo"),
			Delete:     boolField(args, "delete"),
			Full:       boolField(args, "full"),
			Parameters: toolParameters(call.Name, args),
		}
		if call.CpSeqSet || call.CheckpointSeq > 0 {
			checkpointSeq := call.CheckpointSeq
			tool.CheckpointSeq = &checkpointSeq
			tool.CheckpointBranch = call.CheckpointBranchKey
		}
		if call.Name == "subagent" && subagentIsSynchronous(args) {
			tool.Sync = true
		}
		var subchat *chatstore.SubSession
		if call.Name == "subagent" {
			subchat = findSubchat(call.ID)
			tool.Model = stringField(args, "model")
			tool.ReasoningEffort = stringField(args, "reasoning_effort")
		}
		if resultMessage, ok := results[call.ID]; ok {
			tool.Result = apiToolResult(resultMessage.Content)
			if resultStatus, ok := tool.Result["status"].(string); ok && resultStatus == "error" {
				tool.Status = "error"
			} else {
				tool.Status = "success"
			}
		} else if subchat != nil {
			tool.Result = map[string]any{
				"status":    "success",
				"subchatId": subchat.ID,
			}
			status := strings.TrimSpace(subchat.Status)
			if status == "" {
				status = chatstore.SubStatusRunning
			}
			tool.Result["subagentStatus"] = status
			switch status {
			case chatstore.SubStatusDone:
				tool.Status = "success"
			case chatstore.SubStatusRunning, chatstore.SubStatusQueued:
				tool.Status = "running"
			default:
				tool.Status = "error"
			}
		} else if err := tooling.ValidateToolIntent(json.RawMessage(call.Arguments)); err != nil {
			// Older interrupted/rejected turns may have persisted the assistant
			// tool call before dispatch validation. Do not resurrect those calls
			// as active work when the real chat is reopened.
			tool.Result = map[string]any{
				"error":  err.Error(),
				"status": "error",
			}
			tool.Status = "error"
		} else if !runActive {
			// A completed chat can retain an assistant tool call if the process
			// stopped after persisting the assistant turn but before persisting
			// its result. It is not live work anymore, so do not resurrect it as
			// running when the real chat is reopened.
			tool.Result = map[string]any{
				"error":  "chat run ended before the tool result was persisted",
				"status": "error",
			}
			tool.Status = "error"
		} else {
			tool.Status = "running"
		}
		if tool.Name == "subagent" && tool.Result != nil {
			if stringField(tool.Result, "subchatId") == "" && subchat != nil {
				tool.Result["subchatId"] = subchat.ID
			}
			if subchatID := stringField(tool.Result, "subchatId"); subchatID != "" {
				if subSession, subErr := chatstore.FindSubSessionByID(projectID, subchatID); subErr == nil && subSession != nil && strings.TrimSpace(subSession.Status) != "" {
					tool.Result["subagentStatus"] = strings.TrimSpace(subSession.Status)
					subchat = subSession
				}
			}
			if subStatus, ok := tool.Result["subagentStatus"].(string); ok && chatstore.SubSessionRunning(subStatus) {
				tool.Status = "running"
			}
		}
		if subchat != nil {
			tool.Model = subchat.RoleModel
			tool.Provider = subchat.RoleProvider
			tool.FastMode = subchat.FastMode
			tool.ReasoningEffort = subchat.ReasoningEffort
			if tool.Provider == "" {
				tool.Provider = message.Provider
			}
			if tool.Model == "" {
				tool.Model = message.Model
			}
			for i := len(subchat.Messages) - 1; i >= 0; i-- {
				if subchat.Messages[i].Role == "assistant" && subchat.Messages[i].Model != "" {
					tool.Model = subchat.Messages[i].Model
					tool.Provider = subchat.Messages[i].Provider
					tool.FastMode = subchat.Messages[i].FastMode
					if subchat.Messages[i].ReasoningEffort != "" {
						tool.ReasoningEffort = subchat.Messages[i].ReasoningEffort
					}
					break
				}
			}
		}
		out = append(out, tool)
		if includeNested && call.Name == "subagent" && subchat != nil {
			out = append(out, nestedSubagentToolCalls(projectID, parentChatID, subchat, map[string]bool{}, runActive)...)
		}
	}
	return out
}

func nestedSubagentToolCalls(projectID, parentChatID string, subchat *chatstore.SubSession, seen map[string]bool, runActive bool) []apiToolCall {
	if subchat == nil || seen[subchat.ID] {
		return nil
	}
	seen[subchat.ID] = true
	live := runActive || chatstore.SubSessionRunning(subchat.Status)
	var out []apiToolCall
	for index := range subchat.Messages {
		for _, call := range apiToolCallsFromMessage(projectID, parentChatID, subchat.Messages, index, live, false) {
			if call.Name != "subagent" {
				continue
			}
			out = append(out, call)
			if childID := stringField(call.Result, "subchatId"); childID != "" && !seen[childID] {
				if child, err := chatstore.FindSubSessionByID(projectID, childID); err == nil {
					out = append(out, nestedSubagentToolCalls(projectID, parentChatID, child, seen, live)...)
				}
			}
		}
	}
	return out
}

func decodeArguments(raw string) map[string]any {
	var args map[string]any
	if json.Unmarshal([]byte(raw), &args) != nil || args == nil {
		return map[string]any{}
	}
	return args
}

func toolInput(name string, args map[string]any, raw string) string {
	keys := []string{"source", "command", "path", "task", "query", "name", "pattern", "url", "id"}
	for _, key := range keys {
		if value := stringField(args, key); value != "" {
			return value
		}
	}
	if name == "listSubAgents" || name == "todoList" {
		return ""
	}
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return ""
	}
	return strings.TrimSpace(raw)
}

func toolParameters(name string, args map[string]any) []apiToolParameter {
	var keys []string
	switch name {
	case "find", "createPlan", "addTodo", "fetchWeb", "webSearch":
		for key := range args {
			if key != "intent" && key != "mode" {
				keys = append(keys, key)
			}
		}
	default:
		return nil
	}
	sort.Strings(keys)
	params := make([]apiToolParameter, 0, len(keys))
	for _, key := range keys {
		params = append(params, apiToolParameter{Label: key, Value: fmt.Sprint(args[key])})
	}
	return params
}

func apiToolResult(raw string) map[string]any {
	var decoded any
	if json.Unmarshal([]byte(raw), &decoded) != nil {
		return map[string]any{"output": raw, "status": "success"}
	}
	result, ok := decoded.(map[string]any)
	if !ok {
		b, _ := json.Marshal(decoded)
		return map[string]any{"output": string(b), "status": "success"}
	}
	out := make(map[string]any, len(result)+2)
	for key, value := range result {
		out[key] = value
	}
	if calls, ok := result["tool_calls"].([]any); ok {
		out["sdkCalls"] = len(calls)
	}
	if compileError := stringField(result, "compile_error"); compileError != "" && stringField(result, "error") == "" {
		out["error"] = compileError
	}
	status := "success"
	if errText := stringField(result, "error"); errText != "" {
		status = "error"
	}
	if okValue, exists := result["ok"].(bool); exists && !okValue {
		status = "error"
	}
	if statusValue, exists := result["status"].(string); exists && (statusValue == "error" || statusValue == "failed") {
		status = "error"
	}
	if subStatus := stringField(result, "status"); subStatus != "" && subStatus != "success" && subStatus != "error" {
		out["subagentStatus"] = subStatus
	}
	out["status"] = status
	if _, exists := out["output"]; !exists {
		if value, exists := out["summary"]; exists {
			out["output"] = fmt.Sprint(value)
		}
	}
	return out
}

func stringField(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if ok {
		return text
	}
	return ""
}

func boolField(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func subagentIsSynchronous(args map[string]any) bool {
	value, ok := args["run_in_background"]
	if !ok {
		return true
	}
	switch value := value.(type) {
	case bool:
		return !value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "on":
			return false
		case "0", "false", "no", "off":
			return true
		}
	}
	return true
}
