package chatstore

import (
	"path/filepath"
)

func SubchatPath(projectHex, idHex string) (string, error) {
	d, err := SubchatsDir(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, idHex+".json"), nil
}
