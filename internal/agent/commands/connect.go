package commands

import (
	connectcmd "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands/connect"
)

func Connect(d Deps) error {
	return connectcmd.Run(connectDeps(d))
}

func connectDeps(d Deps) connectcmd.Deps {
	return connectcmd.Deps{
		Ctx:               d.Ctx,
		Out:               d.Out,
		Stdin:             d.Stdin,
		ReadLine:          d.ReadLine,
		Cfg:               d.Cfg,
		SaveCfg:           d.SaveCfg,
		ApplyCurrentModel: d.ApplyCurrentModel,
		Model:             d.Model,
		Provider:          d.Provider,
	}
}
