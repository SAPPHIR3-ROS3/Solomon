package chatstore

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

type storedAttachment struct {
	Attachment images.Attachment `json:"attachment"`
	Snapshot   string            `json:"snapshot,omitempty"`
}

func storeAttachments(home string, attachments []images.Attachment) ([]storedAttachment, error) {
	if attachments == nil {
		return nil, nil
	}
	result := make([]storedAttachment, len(attachments))
	for i, attachment := range attachments {
		result[i].Attachment = attachment
		result[i].Attachment.Data = nil
		if len(attachment.Data) != 0 {
			id, err := database.StoreArtifact(home, attachment.Data)
			if err != nil {
				return nil, err
			}
			result[i].Snapshot = id
		}
	}
	return result, nil
}

func restoreAttachments(home string, attachments []storedAttachment) ([]images.Attachment, error) {
	if attachments == nil {
		return nil, nil
	}
	result := make([]images.Attachment, len(attachments))
	for i, stored := range attachments {
		result[i] = stored.Attachment
		if stored.Snapshot == "" {
			continue
		}
		content, err := database.ReadArtifact(home, stored.Snapshot)
		if err != nil {
			return nil, err
		}
		result[i].Data = content
	}
	return result, nil
}
