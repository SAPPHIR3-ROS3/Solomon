package commands

import (
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"strings"
)

func Disconnect(d Deps, parts []string) error {
	if d.Cfg == nil || d.ReadLine == nil {
		return fmt.Errorf("disconnect requires interactive confirmation")
	}
	name := strings.Join(parts[1:], " ")
	if name == "" {
		for _, p := range config.ProviderList(d.Cfg) {
			fmt.Fprintln(d.Out, p.Name)
		}
		var err error
		name, err = d.ReadLine("Provider to remove: ")
		if err != nil {
			return err
		}
	}
	name = strings.TrimSpace(name)
	if config.ProviderByName(d.Cfg, name) == nil {
		return fmt.Errorf("unknown provider %q", name)
	}
	answer, err := d.ReadLine(fmt.Sprintf("Remove provider %q and its saved credentials? Type yes to confirm: ", name))
	if err != nil {
		return err
	}
	if strings.TrimSpace(answer) != "yes" {
		PrintSystem(d.Out, "Removal cancelled.")
		return nil
	}
	latest, err := config.RemoveProvider(name)
	if err != nil {
		return err
	}
	*d.Cfg = *latest
	PrintSystem(d.Out, "Provider removed. Select a model before continuing if the active provider was removed.")
	return nil
}
