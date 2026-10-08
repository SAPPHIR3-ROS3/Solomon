package sdk

// ReadChat reads a saved chat across Solomon projects. Both flags may be false
// for the user/assistant transcript without tool results or stored statistics.
func ReadChat(chatID string, tools, stats bool, intent string) (string, error) {
	raw, err := callTool("readChat", map[string]any{"chatId": chatID, "tools": tools, "stats": stats, "intent": intent})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
