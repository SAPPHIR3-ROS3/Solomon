package images

// Attachment is a snapshot of a tool-read image. Keeping the bytes rather than
// the original path makes replay independent of later edits or file deletion.
type Attachment struct {
	Path             string `json:"path"`
	PreviewPath      string `json:"preview_path,omitempty"`
	MediaType        string `json:"media_type,omitempty"`
	PresentationOnly bool   `json:"presentation_only,omitempty"`
	MIMEType         string `json:"mime_type"`
	Data             []byte `json:"data"`
}

// ToolAttachmentsKey is an in-process side channel, removed before tool output
// is printed, truncated, or sent through the WASM SDK.
const ToolAttachmentsKey = "_solomon_images"

// TakeToolAttachments separates binary attachments from ordinary tool output.
// Only the typed in-process value is accepted, never arbitrary tool JSON.
func TakeToolAttachments(result any) (any, []Attachment) {
	m, ok := result.(map[string]any)
	if !ok {
		return result, nil
	}
	attachments, ok := m[ToolAttachmentsKey].([]Attachment)
	if !ok {
		return result, nil
	}
	clean := make(map[string]any, len(m)-1)
	for k, v := range m {
		if k != ToolAttachmentsKey {
			clean[k] = v
		}
	}
	return clean, attachments
}
