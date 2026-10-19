package commands

import (
	connectcmd "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func Connect(d Deps) error {
	return connectcmd.Run(connectDeps(d))
}

func connectDeps(d Deps) connectcmd.Deps {
	return connectcmd.Deps{
		Ctx:      d.Ctx,
		Out:      d.Out,
		Stdin:    d.Stdin,
		ReadLine: d.ReadLine,
		Cfg:      d.Cfg,
		SaveCfg:  d.SaveCfg,
		SaveProvider: func(provider config.Provider) error {
			latest, err := config.UpdateProvider(provider)
			if err == nil {
				*d.Cfg = *latest
			}
			return err
		},
		ApplyCurrentModel: d.ApplyCurrentModel,
		Model:             d.Model,
		Provider:          d.Provider,
	}
}
