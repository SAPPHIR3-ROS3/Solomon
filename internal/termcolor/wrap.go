package termcolor

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

var replImgVisibleRE = regexp.MustCompile(`\[img-\d+\]`)

const resetANSI = "\x1b[0m"

const SystemBorder = "===SYSTEM==="

func ResetSeq() string {
	if !colorOn {
		return ""
	}
	return resetANSI
}

func WrapUser(s string) string {
	return renderStyle(dark.user, s)
}

func WrapUserReadline(s string) string {
	if runtime.GOOS == "windows" {
		if !colorOn {
			return s
		}
		if REPLRawStdout() {
			return WrapUser(s)
		}
		return "\033[36m" + s + resetANSI
	}
	return WrapUser(s)
}

func WrapRed(s string) string {
	return renderStyle(dark.red, s)
}

func WrapAssistant(s string) string {
	return renderStyle(dark.assistant, s)
}

func WrapTool(s string) string {
	return renderStyle(dark.tool, s)
}

func WrapEditFileOldString(s string) string {
	return renderStyle(dark.editOld, s)
}

func WrapEditFileNewString(s string) string {
	return renderStyle(dark.editNew, s)
}

func WrapEditFileOldStringLine(s string) string {
	return extendEditLineBackground(WrapEditFileOldString(s))
}

func WrapEditFileNewStringLine(s string) string {
	return extendEditLineBackground(WrapEditFileNewString(s))
}

func extendEditLineBackground(styled string) string {
	if !colorOn || styled == "" {
		return styled
	}
	if strings.HasSuffix(styled, resetANSI) {
		return styled[:len(styled)-len(resetANSI)] + "\x1b[K" + resetANSI
	}
	return styled + "\x1b[K"
}

func IsEditLineDisplay(s string) bool {
	return strings.Contains(s, "\x1b[K")
}

var (
	editStyleOnce      sync.Once
	editOldStylePrefix string
	editNewStylePrefix string
)

func initEditStylePrefixes() {
	editStyleOnce.Do(func() {
		editOldStylePrefix = editStylePrefix(WrapEditFileOldString(""))
		editNewStylePrefix = editStylePrefix(WrapEditFileNewString(""))
	})
}

func editStylePrefix(styled string) string {
	s := styled
	if strings.HasSuffix(s, resetANSI) {
		s = strings.TrimSuffix(s, resetANSI)
	}
	if strings.HasSuffix(s, "\x1b[K") {
		s = strings.TrimSuffix(s, "\x1b[K")
	}
	plain := Plain(s)
	if plain != "" && strings.HasSuffix(s, plain) {
		return strings.TrimSuffix(s, plain)
	}
	return s
}

func RewrapEditLineLike(sampleStyled, plainChunk string) string {
	initEditStylePrefixes()
	if plainChunk == "" {
		plainChunk = " "
	}
	if editStylePrefix(sampleStyled) == editNewStylePrefix {
		return WrapEditFileNewStringLine(plainChunk)
	}
	return WrapEditFileOldStringLine(plainChunk)
}

func GoParen(s string, depth int) string {
	if depth < 0 {
		depth = 0
	}
	return renderStyle(dark.goParen[depth%3], s)
}

func ToolLine(toolName, body string) string {
	if !colorOn {
		if body == "" {
			return toolName
		}
		return toolName + " " + body
	}
	out := dark.toolBold.Render(toolName)
	if body != "" {
		out += dark.tool.Render(" " + body)
	}
	return out
}

func OrchestrateCodeLabel(s string) string {
	return renderStyle(dark.orchestrateCode, s)
}

func OrchestrateToolHeaderLine() string {
	if !colorOn {
		return "Tool: orchestrate Code"
	}
	return dark.tool.Render("Tool: ") + dark.toolBold.Render("orchestrate ") + OrchestrateCodeLabel("Code")
}

func SwitchModeToolHeaderLine(modeLabel string) string {
	if !colorOn {
		return "Tool: switchMode " + modeLabel
	}
	return dark.tool.Render("Tool: ") + dark.toolBold.Render("switchMode ") + OrchestrateCodeLabel(modeLabel)
}

func OrchestrateCodeFooterLine() string {
	return OrchestrateCodeLabel("Code")
}

func GoKeyword(s string) string {
	return renderStyle(dark.goKeyword, s)
}

func GoString(s string) string {
	return renderStyle(dark.goString, s)
}

func GoComment(s string) string {
	return renderStyle(dark.goComment, s)
}

func GoFunction(s string) string {
	return renderStyle(dark.goFunction, s)
}

func GoNumber(s string) string {
	return renderStyle(dark.goNumber, s)
}

func GoPlain(s string) string {
	return renderStyle(dark.goPlain, s)
}

func ToolHeaderLine(toolName, body string) string {
	if !colorOn {
		prefix := "Tool: " + toolName
		if body == "" {
			return prefix
		}
		return prefix + " " + body
	}
	out := dark.tool.Render("Tool: ") + dark.toolBold.Render(toolName)
	if body != "" {
		out += dark.tool.Render(" " + body)
	}
	return out
}

func ToolHeaderRedArgLine(toolName, arg string) string {
	if !colorOn {
		prefix := "Tool: " + toolName
		if arg == "" {
			return prefix
		}
		return prefix + " " + arg
	}
	out := dark.tool.Render("Tool: ") + dark.toolBold.Render(toolName)
	if arg != "" {
		out += dark.tool.Render(" ") + dark.red.Render(arg)
	}
	return out
}

func EditFileDeleteToolLine(path string) string {
	return ToolHeaderRedArgLine("editFile", path)
}

func WrapThinking(s string) string {
	return renderStyle(dark.thinking, s)
}

func WrapWhite(s string) string {
	return renderStyle(dark.white, s)
}

func WrapBoldGold(s string) string {
	return renderStyle(dark.boldGold, s)
}

func WrapContext(s string) string {
	return renderStyle(dark.context, s)
}

func WrapSystem(s string) string {
	return renderStyle(dark.system, s)
}

func FormatSystemBlock(message string) string {
	message = strings.TrimRight(message, "\n")
	if strings.TrimSpace(message) == "" {
		return ""
	}
	lines := append([]string{SystemBorder}, strings.Split(message, "\n")...)
	lines = append(lines, SystemBorder)
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(WrapSystem(line))
	}
	b.WriteByte('\n')
	return b.String()
}

func WriteSystem(w io.Writer, message string) {
	if s := FormatSystemBlock(message); s != "" {
		_, _ = io.WriteString(w, s)
	}
}

func SystemMessageText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return formatSystemString(x)
	case error:
		return x.Error()
	case fmt.Stringer:
		return x.String()
	default:
		return formatSystemValue(x)
	}
}

func formatSystemString(s string) string {
	trim := strings.TrimSpace(s)
	if trim == "" {
		return s
	}
	if trim[0] == '{' || trim[0] == '[' {
		var parsed any
		if json.Unmarshal([]byte(trim), &parsed) == nil {
			return formatSystemValue(parsed)
		}
	}
	return s
}

func formatSystemValue(v any) string {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return ""
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for i, k := range keys {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(formatSystemValue(x[k]))
		}
		return b.String()
	case []any:
		if len(x) == 0 {
			return ""
		}
		var b strings.Builder
		for i, item := range x {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(formatSystemValue(item))
		}
		return b.String()
	default:
		return fmt.Sprint(v)
	}
}

func ForegroundRGB(r, g, b uint8) string {
	if !colorOn {
		return ""
	}
	c := profile().Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
	if c == nil {
		return ""
	}
	seq := c.Sequence(false)
	if seq == "" {
		return ""
	}
	return "\x1b[" + seq + "m"
}

func BackgroundRGB(r, g, b uint8) string {
	if !colorOn {
		return ""
	}
	c := profile().Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
	if c == nil {
		return ""
	}
	seq := c.Sequence(true)
	if seq == "" {
		return ""
	}
	return "\x1b[" + seq + "m"
}

func WrapImgTag(tag string) string {
	return renderStyle(dark.imgTag, tag)
}

func wrapImgTagReplInput(tag string) string {
	if runtime.GOOS == "windows" {
		if !colorOn {
			return tag
		}
		return "\033[37m\033[46m" + tag + resetANSI
	}
	return WrapImgTag(tag)
}

func ColorizeImgTags(s string) string {
	return images.ColorizeVisibleImgTags(images.ExpandForDisplay(s), WrapImgTag)
}

func ColorizeImgTagsReplInput(s string) string {
	return replImgVisibleRE.ReplaceAllStringFunc(s, wrapImgTagReplInput)
}

var replAtTagRE = regexp.MustCompile(`@[^\s@]+`)

func wrapAtTagReplInput(tag string) string {
	if runtime.GOOS == "windows" {
		if !colorOn {
			return tag
		}
		return "\033[30m\033[43m" + tag + resetANSI
	}
	return renderStyle(dark.atTag, tag)
}

func ColorizeAtTagsReplInput(s string) string {
	return replAtTagRE.ReplaceAllStringFunc(s, wrapAtTagReplInput)
}

func ColorizeReplInputTags(s string) string {
	return ColorizeAtTagsReplInput(ColorizeImgTagsReplInput(s))
}
