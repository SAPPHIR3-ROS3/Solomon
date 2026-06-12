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
	colorAtTagBG        = "#B8860B"
	colorEditFileOldBG     = "#5c0000"
	colorEditFileNewBG     = "#005c00"
	colorOrchestrateCodeFG = "#006400"
	colorGoKeyword         = "#569CD6"
	colorGoString          = "#FF79C6"
	colorGoComment         = "#6A9955"
	colorGoFunction        = "#DCDCAA"
	colorGoNumber          = "#B5CEA8"
	colorGoPlain           = "#D4D4D4"
	colorGoParen0          = "#FFD700"
	colorGoParen1          = "#FF55FF"
	colorGoParen2          = "#5555FF"
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
	atTag     lipgloss.Style
	editOld        lipgloss.Style
	editNew        lipgloss.Style
	orchestrateCode lipgloss.Style
	goKeyword       lipgloss.Style
	goString        lipgloss.Style
	goComment       lipgloss.Style
	goFunction      lipgloss.Style
	goNumber        lipgloss.Style
	goPlain         lipgloss.Style
	goParen         [3]lipgloss.Style
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
	dark.atTag = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorWhite)).
		Background(lipgloss.Color(colorAtTagBG))
	dark.editOld = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorWhite)).
		Background(lipgloss.Color(colorEditFileOldBG))
	dark.editNew = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorWhite)).
		Background(lipgloss.Color(colorEditFileNewBG))
	dark.orchestrateCode = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrchestrateCodeFG)).Bold(true)
	dark.goKeyword = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoKeyword))
	dark.goString = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoString))
	dark.goComment = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoComment))
	dark.goFunction = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoFunction))
	dark.goNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoNumber))
	dark.goPlain = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoPlain))
	dark.goParen[0] = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoParen0))
	dark.goParen[1] = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoParen1))
	dark.goParen[2] = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGoParen2))
}
