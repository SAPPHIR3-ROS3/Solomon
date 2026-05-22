package tooloutput

const (
	truncatedMarker = "---TRUNCATED---"
)

func FormatTruncatedMessage(spillPath string) string {
	line := "full output unavailable"
	if spillPath != "" {
		line = "full output at " + spillPath
	}
	return truncatedMarker + "\n" + line + "\n" + truncatedMarker
}

func truncatedResult(spillPath string, spillErr error) map[string]any {
	msg := FormatTruncatedMessage(spillPath)
	out := map[string]any{
		"truncated": true,
		"output":    msg,
	}
	if spillPath != "" {
		out["spill_path"] = spillPath
	}
	if spillErr != nil {
		out["spill_error"] = spillErr.Error()
	}
	return out
}
