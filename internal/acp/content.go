package acp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func (c *Content) UnmarshalJSON(data []byte) error {
	type plain Content
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if value.Type == "text" {
		var text string
		raw, ok := fields["text"]
		if !ok || string(raw) == "null" || json.Unmarshal(raw, &text) != nil {
			return fmt.Errorf("text content requires a string text field")
		}
	}
	if value.Type == "image" {
		if _, err := base64.StdEncoding.DecodeString(value.Data); err != nil {
			return fmt.Errorf("image data must be base64")
		}
	}
	if value.Resource != nil && value.Resource.Blob != "" {
		if _, err := base64.StdEncoding.DecodeString(value.Resource.Blob); err != nil {
			return fmt.Errorf("embedded resource blob must be base64")
		}
	}
	*c = Content(value)
	return nil
}
