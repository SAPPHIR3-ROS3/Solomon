package server

import (
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func apiChatFromSession(projectID string, sess *chatstore.Session, run *chatRun) apiChat {
	if sess == nil {
		return apiChat{ProjectID: projectID, Messages: []apiMessage{}}
	}
	status := ""
	runStartedAt := ""
	if run != nil {
		status = "running"
		runStartedAt = run.startedAt.UTC().Format(time.RFC3339Nano)
	}
	return apiChat{
		CreatedAt:    sess.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:           sess.ID,
		Messages:     apiMessagesFromSession(projectID, sess.ID, sess.Messages, sess.ImageFiles, sess.UncompactedRaw, run != nil),
		Mode:         "agent",
		ProjectID:    projectID,
		RunStartedAt: runStartedAt,
		Status:       status,
		Title:        sessionTitle(sess),
	}
}

func apiChatFromSubSession(projectID string, sess *chatstore.SubSession) apiChat {
	if sess == nil {
		return apiChat{ProjectID: projectID, Messages: []apiMessage{}}
	}
	return apiChat{
		CreatedAt: sess.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        sess.ID,
		Messages:  apiMessagesFromSession(projectID, sess.ID, sess.Messages, sess.ImageFiles, nil, chatstore.SubSessionRunning(sess.Status)),
		Mode:      "agent",
		ProjectID: projectID,
		Status:    strings.TrimSpace(sess.Status),
		Title:     strings.TrimSpace(sess.Title),
	}
}

func needsChatTitleGeneration(sess *chatstore.Session) bool {
	if sess == nil || len(sess.Messages) != 0 {
		return false
	}
	title := strings.ToLower(strings.TrimSpace(sess.Title))
	return title == "" || title == "new chat" || title == "untitled chat"
}

func sessionTitle(sess *chatstore.Session) string {
	title := strings.TrimSpace(sess.Title)
	if title == "" {
		return "Untitled chat"
	}
	return title
}

func apiMessagesFromSession(projectID, ownerID string, messages []chatstore.Message, imageFiles map[int]string, dumps []chatstore.UncompactedDump, runActive bool) []apiMessage {
	out := make([]apiMessage, 0, len(messages)+len(dumps)*8)
	for dumpIndex := range dumps {
		archive := archiveMessagesForDisplay(dumps, dumpIndex)
		visibleCount := countDisplayableMessages(archive)
		retainedCount := chatMinInt(8, visibleCount)
		visibleIndex := 0
		for sourceIndex, archivedMessage := range archive {
			if isHiddenSessionMessage(archivedMessage) || isCompactionSummaryMessage(archivedMessage) {
				continue
			}
			if visibleIndex >= visibleCount-retainedCount {
				visibleIndex++
				continue
			}
			if value, ok := apiMessageFromSession(projectID, ownerID, archivedMessage, archive, sourceIndex, imageFiles, runActive, fmt.Sprintf("m-archive-%d-%d", dumpIndex, sourceIndex)); ok {
				out = append(out, value)
			}
			visibleIndex++
		}
		retained := retainedMessagesForDisplay(projectID, ownerID, archive, imageFiles, visibleCount-retainedCount)
		out = append(out, apiMessage{
			Content:          "",
			ID:               fmt.Sprintf("compaction-%d", dumpIndex),
			Kind:             "compaction",
			RetainedMessages: retained,
			Role:             "assistant",
			Summary:          compactionSummary(messages, dumps, dumpIndex),
		})
	}
	for index, message := range messages {
		if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
			continue
		}
		if value, ok := apiMessageFromSession(projectID, ownerID, message, messages, index, imageFiles, runActive, fmt.Sprintf("m-%d", index)); ok {
			out = append(out, value)
		}
	}
	return out
}

func archiveMessagesForDisplay(dumps []chatstore.UncompactedDump, dumpIndex int) []chatstore.Message {
	if dumpIndex < 0 || dumpIndex >= len(dumps) {
		return nil
	}
	archive := dumps[dumpIndex].Messages
	if dumpIndex == 0 {
		return archive
	}
	for index, message := range archive {
		if isCompactionSummaryMessage(message) {
			return archive[index+1:]
		}
	}
	return archive
}

func countDisplayableMessages(messages []chatstore.Message) int {
	count := 0
	for _, message := range messages {
		if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
			continue
		}
		count++
	}
	return count
}

func retainedMessagesForDisplay(projectID, ownerID string, messages []chatstore.Message, imageFiles map[int]string, skip int) []apiRetainedMessage {
	retained := make([]apiRetainedMessage, 0, chatMinInt(8, len(messages)))
	visibleIndex := 0
	for _, message := range messages {
		if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
			continue
		}
		if visibleIndex < skip {
			visibleIndex++
			continue
		}
		content, imgs := displayContentAndImages(projectID, ownerID, message.Content, imageFiles)
		retained = append(retained, apiRetainedMessage{Content: content, Images: imgs, Role: normalizedMessageRole(message.Role)})
		visibleIndex++
	}
	return retained
}

func chatMinInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func apiMessageFromSession(projectID, ownerID string, message chatstore.Message, source []chatstore.Message, index int, imageFiles map[int]string, runActive bool, id string) (apiMessage, bool) {
	if isHiddenSessionMessage(message) || isCompactionSummaryMessage(message) {
		return apiMessage{}, false
	}
	role := normalizedMessageRole(message.Role)
	displaySource := message.Content
	var reasoning string
	if role == "assistant" {
		reasoning, displaySource = chatstore.AssistantDisplayParts(message)
	}
	content, imgs := displayContentAndImages(projectID, ownerID, displaySource, imageFiles)
	value := apiMessage{Content: content, ID: id, Images: imgs, Role: role}
	if !message.CreatedAt.IsZero() {
		value.CreatedAt = message.CreatedAt.Local().Format(time.RFC3339Nano)
	}
	if role == "assistant" {
		value.FileChanges = message.FileChanges
		value.FileChangesRecorded = message.FileChangesRecorded
		value.Reasoning = reasoning
		value.ToolCalls = apiToolCallsFromMessage(projectID, ownerID, source, index, runActive, true)
		value.Stats = apiStatsForMessageTurn(message, source, index)
		value.ThoughtFor = message.TurnTTFTSecs
		if value.ThoughtFor == 0 {
			value.ThoughtFor = message.TTFTSecs
		}
		value.WorkedFor = message.TurnWallDisplay
		if value.WorkedFor == 0 {
			value.WorkedFor = message.TurnWallSecs
		}
		value.WorkedFor += messageToolWallSeconds(message, source, index)
	}
	if chatstore.MessageCheckpointTagVisible(message) {
		seq := message.CheckpointSeq
		value.CheckpointSeq = &seq
		value.CheckpointBranch = message.CheckpointBranchKey
	}
	return value, true
}

func isHiddenSessionMessage(message chatstore.Message) bool {
	if message.GUIHidden || message.Role == "tool" {
		return true
	}
	// Command lines are marked GUIHidden when recorded; a skill request starting with "/" stays visible.
	return message.Role == "user" && strings.HasPrefix(strings.TrimSpace(message.Content), "tool_result(")
}

func normalizedMessageRole(role string) string {
	if role == "user" {
		return "user"
	}
	return "assistant"
}

func compactionSummary(messages []chatstore.Message, dumps []chatstore.UncompactedDump, dumpIndex int) string {
	var source []chatstore.Message
	if dumpIndex+1 < len(dumps) {
		source = dumps[dumpIndex+1].Messages
	} else {
		source = messages
	}
	for _, message := range source {
		if isCompactionSummaryMessage(message) {
			return compactSummaryText(message.Content)
		}
	}
	return fmt.Sprintf("Context compacted after archived turn %d.", dumpIndex+1)
}

func isCompactionSummaryMessage(message chatstore.Message) bool {
	return chatstore.IsCompactionSummary(message)
}

func compactSummaryText(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	start := -1
	end := len(lines)
	for index, line := range lines {
		if strings.TrimSpace(line) == "[Conversation summary]" {
			start = index + 1
			break
		}
	}
	if start < 0 {
		return strings.TrimSpace(body)
	}
	for index := start; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "[Retained messages]" {
			end = index
			break
		}
	}
	for start < end && isCompactSummarySeparator(lines[start]) {
		start++
	}
	for end > start && isCompactSummarySeparator(lines[end-1]) {
		end--
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func isCompactSummarySeparator(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) < 3 {
		return false
	}
	first := line[0]
	for index := 1; index < len(line); index++ {
		if line[index] != first {
			return false
		}
	}
	return first == '-' || first == '='
}

func displayContentAndImages(projectID, ownerID, content string, imageFiles map[int]string) (string, []apiImage) {
	imageValues := make([]apiImage, 0)
	display := images.ExpandForDisplay(content)
	seen := make(map[int]bool)
	display = imageTagPattern.ReplaceAllStringFunc(display, func(tag string) string {
		match := imageTagPattern.FindStringSubmatch(tag)
		if len(match) != 2 {
			return tag
		}
		seq, err := strconv.Atoi(match[1])
		if err != nil || imageFiles == nil {
			return tag
		}
		filePath := strings.TrimSpace(imageFiles[seq])
		if filePath == "" || !isSafeProjectImagePath(projectID, filePath) {
			return tag
		}
		if !seen[seq] {
			seen[seq] = true
			name := filepath.Base(filePath)
			if name == "." || name == string(filepath.Separator) {
				name = fmt.Sprintf("image-%d", seq)
			}
			imageValues = append(imageValues, apiImage{
				Name: name,
				URL:  fmt.Sprintf("%s/%s/chats/%s/images/%d", chatAPIPath, url.PathEscape(projectID), url.PathEscape(ownerID), seq),
			})
		}
		return tag
	})
	return strings.TrimSpace(display), imageValues
}

func chatTitleInput(content string, hasImages bool) string {
	content = imageTagPattern.ReplaceAllString(content, "image")
	content = strings.TrimSpace(content)
	if content == "" && hasImages {
		return "The user shared an image and needs help with it."
	}
	return content
}

func messageToolWallSeconds(message chatstore.Message, source []chatstore.Message, index int) float64 {
	if message.ToolWallSecs > 0 {
		return message.ToolWallSecs
	}
	if len(message.ToolCalls) == 0 || message.CreatedAt.IsZero() {
		return 0
	}
	for i := index + 1; i < len(source); i++ {
		next := source[i]
		if next.Role == "tool" {
			continue
		}
		if next.Role != "assistant" || next.CreatedAt.IsZero() || next.TurnWallSecs <= 0 {
			return 0
		}
		seconds := next.CreatedAt.Sub(message.CreatedAt).Seconds() - next.TurnWallSecs
		if seconds > 0 {
			return seconds
		}
		return 0
	}
	return 0
}
