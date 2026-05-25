package connect

import "fmt"

func Run(d Deps) error {
	choice, err := chooseKind(d)
	if err != nil {
		return err
	}
	switch choice {
	case 1:
		return chatGPTSub(d)
	case 2:
		return compatibleAPI(d)
	case 3:
		return anthropicCompatibleAPI(d)
	case 4:
		return claudeSubComingSoon(d)
	case 5:
		return cursorAPI(d)
	default:
		return fmt.Errorf("internal error: unknown connect choice %d", choice)
	}
}
