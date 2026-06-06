package termcolor

import (
	"github.com/charmbracelet/lipgloss"
)

const (
	colorUser      = "#00FFFF"
	colorAssistant = "#00FF00"
	colorTool      = "#FFF69D"
	colorThinking  = "#808080"
	colorWhite     = "#FFFFFF"
	colorContext   = "#5555FF"
	colorSystem    = "#AA55FF"
	colorRed       = "#FF5555"
	colorGold      = "#FFD700"
	colorImgBG          = "#00D1F0"
	colorEditFileOldBG  = "#5c0000"
	colorEditFileNewBG  = "#005c00"
)

var dark struct {
	user      lipgloss.Style
	assistant lipgloss.Style
	tool      lipgloss.Style
	toolBold  lipgloss.Style
	thinking  lipgloss.Style
	white     lipgloss.Style
	context   lipgloss.Style
	system    lipgloss.Style
	red       lipgloss.Style
	boldGold  lipgloss.Style
	imgTag    lipgloss.Style
	editOld   lipgloss.Style
	editNew   lipgloss.Style
}

func rebuildTheme() {
	rebuildZshStyles()
	dark.user = lipgloss.NewStyle().Foreground(lipgloss.Color(colorUser))
	dark.assistant = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAssistant))
	dark.tool = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTool))
	dark.toolBold = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTool)).Bold(true)
	dark.thinking = lipgloss.NewStyle().Foreground(lipgloss.Color(colorThinking))
	dark.white = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWhite))
	dark.context = lipgloss.NewStyle().Foreground(lipgloss.Color(colorContext))
	dark.system = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSystem))
	dark.red = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed))
	dark.boldGold = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGold)).Bold(true)
	dark.imgTag = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorWhite)).
		Background(lipgloss.Color(colorImgBG))
	dark.editOld = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorWhite)).
		Background(lipgloss.Color(colorEditFileOldBG))
	dark.editNew = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorWhite)).
		Background(lipgloss.Color(colorEditFileNewBG))
}
