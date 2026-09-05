package test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_chatDisplayKeepsVisibleImageTags(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	decodeServerTestJSON(t, projectResponse, &created)

	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	var createdChat struct {
		ID string `json:"id"`
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	imgDir, err := paths.ChatImagesDir(created.Project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(imgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	imgPath := filepath.Join(imgDir, "pasted.png")
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if err := os.WriteFile(imgPath, png, 0o600); err != nil {
		t.Fatal(err)
	}

	sess, err := chatstore.ReadSession(created.Project.ID, createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}
	tag := images.VisibleTag(0)
	sess.Messages = []chatstore.Message{{
		Role:    "user",
		Content: "caption " + tag,
	}}
	sess.ImageFiles = map[int]string{0: imgPath}
	if err := chatstore.WriteSession(created.Project.ID, sess); err != nil {
		t.Fatal(err)
	}

	response, err := http.Get(server.URL + "/__solomon/projects/" + created.Project.ID + "/chats/" + createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("open chat status = %d", response.StatusCode)
	}
	var payload struct {
		Messages []struct {
			Content string `json:"content"`
			Images  []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"images"`
		} `json:"messages"`
	}
	decodeServerTestJSON(t, response, &payload)
	if len(payload.Messages) != 1 {
		t.Fatalf("messages = %d", len(payload.Messages))
	}
	if !strings.Contains(payload.Messages[0].Content, tag) {
		t.Fatalf("display dropped image tag, content=%q", payload.Messages[0].Content)
	}
	if len(payload.Messages[0].Images) != 1 {
		t.Fatalf("images = %+v", payload.Messages[0].Images)
	}
}
