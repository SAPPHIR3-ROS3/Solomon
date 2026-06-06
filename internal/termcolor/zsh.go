package termcolor

import "github.com/charmbracelet/lipgloss"

type ZshStyleKey string

const (
	ZshUnknownToken              ZshStyleKey = "unknown-token"
	ZshReservedWord              ZshStyleKey = "reserved-word"
	ZshArg0                      ZshStyleKey = "arg0"
	ZshBuiltin                   ZshStyleKey = "builtin"
	ZshPath                      ZshStyleKey = "path"
	ZshPathPrefix                ZshStyleKey = "path_prefix"
	ZshGlobbing                  ZshStyleKey = "globbing"
	ZshSingleQuoted              ZshStyleKey = "single-quoted-argument"
	ZshDoubleQuoted              ZshStyleKey = "double-quoted-argument"
	ZshDollarDoubleQuoted        ZshStyleKey = "dollar-double-quoted-argument"
	ZshRedirection               ZshStyleKey = "redirection"
	ZshCommandSeparator          ZshStyleKey = "commandseparator"
	ZshComment                   ZshStyleKey = "comment"
	ZshDefault                   ZshStyleKey = "default"
)

var zshStyles map[ZshStyleKey]lipgloss.Style

func rebuildZshStyles() {
	zshStyles = map[ZshStyleKey]lipgloss.Style{
		ZshUnknownToken:       lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		ZshReservedWord:       lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		ZshArg0:               lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		ZshBuiltin:            lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		ZshPath:               lipgloss.NewStyle().Underline(true),
		ZshPathPrefix:         lipgloss.NewStyle().Underline(true).Faint(true),
		ZshGlobbing:           lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
		ZshSingleQuoted:       lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		ZshDoubleQuoted:       lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		ZshDollarDoubleQuoted: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		ZshRedirection:        lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		ZshCommandSeparator:   lipgloss.NewStyle(),
		ZshComment:            lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Bold(true),
		ZshDefault:            lipgloss.NewStyle(),
	}
}

func ZshStyle(key ZshStyleKey, s string) string {
	if s == "" || key == ZshDefault {
		return s
	}
	if zshStyles == nil {
		rebuildZshStyles()
	}
	st, ok := zshStyles[key]
	if !ok || !colorOn {
		return s
	}
	return st.Render(s)
}
