package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
)

func connectPickModel(d Deps, prevProv, prevModel, newProvName string, newIDs []string) error {
	choice, err := config.PickModelAfterAdd(PromptIO(d), prevProv, prevModel, newProvName, newIDs, false)
	if err != nil {
		return err
	}
	if err := d.ApplyCurrentModel(choice.ProviderName, choice.ModelID); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Using %s[%s]\n", d.Model(), d.Provider().Name)
	return nil
}
