package sdk

func CloakBrowser(tool string, args map[string]any, intent string) (map[string]any, error) {
	raw, err := callTool("cloakBrowser", map[string]any{"tool": tool, "args": args, "intent": intent})
	if err != nil {
		return nil, err
	}
	return unmarshalToolMap(raw)
}
