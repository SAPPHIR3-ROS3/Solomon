package connect

import (
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/internal/integrations/cursor"
)

func pickModel(d Deps, prevProv, prevModel, newProvName string, newIDs []string) error {
	if newProvName == config.ProviderNameCursorAPI {
		newIDs = cursorint.FilterModelIDs(newIDs)
	}
	choice, err := config.PickModelAfterAdd(PromptIO(d), prevProv, prevModel, newProvName, newIDs, false)
	if err != nil {
		return err
	}
	if err := d.ApplyCurrentModel(choice.ProviderName, choice.ModelID); err != nil {
		return err
	}
	printSystemf(d, "Using %s[%s]", d.Model(), d.Provider().Name)
	return nil
}
