package termcolor

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const resetANSI = "\x1b[0m"

const SystemBorder = "===SYSTEM==="

var imgTagRe = regexp.MustCompile(`\[img-\d+\]`)

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
		return "\033[96m" + s + resetANSI
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
	body := SystemBorder + "\n" + message + "\n" + SystemBorder + "\n"
	return WrapSystem(body)
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
	return imgTagRe.ReplaceAllStringFunc(s, WrapImgTag)
}

func ColorizeImgTagsReplInput(s string) string {
	return imgTagRe.ReplaceAllStringFunc(s, wrapImgTagReplInput)
}
