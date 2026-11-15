package sdk

import "encoding/json"

type FileInfo struct {
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Mode     uint32 `json:"mode"`
	Modified string `json:"modified"`
}

func Stat(path, intent string) (FileInfo, error) {
	raw, err := callTool("stat", map[string]any{"path": path, "intent": intent})
	if err != nil {
		return FileInfo{}, err
	}
	var info FileInfo
	err = json.Unmarshal(raw, &info)
	return info, err
}

func CopyFile(path, destination, intent string) error {
	_, err := callTool("copyFile", map[string]any{"path": path, "destination": destination, "intent": intent})
	return err
}

func MoveFile(path, destination, intent string) error {
	return RenameFile(path, destination, intent)
}

func CreateDir(path, intent string) error {
	_, err := callTool("createDir", map[string]any{"path": path, "intent": intent})
	return err
}
