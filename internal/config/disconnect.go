package config

import "fmt"

func RemoveProvider(name string) (*Root, error) {
	return updateConfig(func(root *Root) error {
		if root.Providers[name] == nil {
			return fmt.Errorf("unknown provider %q", name)
		}
		delete(root.Providers, name)
		delete(root.RecentModels, name)
		delete(root.HiddenModels, name)
		delete(root.ModelModes, name)
		if root.Current.Provider == name {
			root.Current = Current{}
		}
		return nil
	})
}
