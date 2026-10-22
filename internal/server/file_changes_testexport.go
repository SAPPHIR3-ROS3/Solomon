package server

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"

func APIChatFromSessionForTest(projectID string, session *chatstore.Session) apiChat {
	return apiChatFromSession(projectID, session, nil)
}
