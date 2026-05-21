package commands

import (
	"fmt"
)

func Connect(d Deps) error {
	sc := connectScanner(d.Stdin)
	choice, err := connectChooseKind(d, sc)
	if err != nil {
		return err
	}
	switch choice {
	case 1:
		return connectChatGPTSub(d, sc)
	case 2:
		return connectCompatibleAPI(d, sc)
	case 3:
		return connectAnthropicCompatibleAPI(d, sc)
	case 4:
		return connectClaudeSubComingSoon(d)
	default:
		return fmt.Errorf("internal error: unknown connect choice %d", choice)
	}
}
