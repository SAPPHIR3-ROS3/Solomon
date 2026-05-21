package commands

import (
	"fmt"
)

func Connect(d Deps) error {
	choice, err := connectChooseKind(d)
	if err != nil {
		return err
	}
	switch choice {
	case 1:
		return connectChatGPTSub(d)
	case 2:
		return connectCompatibleAPI(d)
	case 3:
		return connectAnthropicCompatibleAPI(d)
	case 4:
		return connectClaudeSubComingSoon(d)
	default:
		return fmt.Errorf("internal error: unknown connect choice %d", choice)
	}
}
