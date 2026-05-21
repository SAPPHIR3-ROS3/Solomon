package termcolor

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func formatContextPromptTok(n int64, estimated bool) string {
	s := strconv.FormatInt(n, 10)
	if estimated && n > 0 {
		return "~" + s
	}
	return s
}

func WelcomeUsageTotals(userTok, reasoningTok, responseTok, totalTok int64) string {
	return WrapUser(strconv.FormatInt(userTok, 10)) + "+" +
		WrapThinking(strconv.FormatInt(reasoningTok, 10)) + "+" +
		WrapAssistant(strconv.FormatInt(responseTok, 10)) + "=" +
		WrapWhite(strconv.FormatInt(totalTok, 10))
}

func formatFloatMax3(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0"
	}
	s := fmt.Sprintf("%.3f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func FormatWorkedDuration(secs float64) string {
	if secs <= 0 || math.IsNaN(secs) {
		return "0s"
	}
	h := int(secs / 3600)
	r1 := secs - float64(h*3600)
	m := int(r1 / 60)
	s := r1 - float64(m*60)
	var b strings.Builder
	if h > 0 {
		fmt.Fprintf(&b, "%dh", h)
	}
	if m > 0 || h > 0 {
		fmt.Fprintf(&b, "%dm", m)
	}
	fmt.Fprintf(&b, "%ss", formatFloatMax3(s))
	return b.String()
}

func UsageTokensLine(contextPromptTok, lastUserPromptTok, reasoningTokens, responseTokens, totalTokens int64, outputTPS, ttftSecs, promptTPS float64, contextEstimated bool, turnWallSecs float64) string {
	var promptSeg string
	switch {
	case contextPromptTok <= 0 && lastUserPromptTok <= 0:
		promptSeg = WrapUser("0")
	case lastUserPromptTok <= 0:
		promptSeg = WrapContext(formatContextPromptTok(contextPromptTok, contextEstimated))
	case contextPromptTok <= 0:
		promptSeg = WrapUser(strconv.FormatInt(lastUserPromptTok, 10))
	default:
		promptSeg = WrapContext(formatContextPromptTok(contextPromptTok, contextEstimated)) + "+" + WrapUser(strconv.FormatInt(lastUserPromptTok, 10))
	}
	line := "token: " + promptSeg + "+" +
		WrapThinking(strconv.FormatInt(reasoningTokens, 10)) + "+" +
		WrapAssistant(strconv.FormatInt(responseTokens, 10)) + "=" +
		WrapWhite(strconv.FormatInt(totalTokens, 10)) +
		fmt.Sprintf("\t%st/s ttft:%ss pp:%st/s", formatFloatMax3(outputTPS), formatFloatMax3(ttftSecs), formatFloatMax3(promptTPS))
	line += "\t worked for " + FormatWorkedDuration(turnWallSecs)
	return line
}

func ThoughtForSuffix(secs float64) string {
	return WrapThinking("thought for " + FormatWorkedDuration(secs))
}
