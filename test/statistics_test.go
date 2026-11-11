package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerTurnStatisticsContextIsLastRequest(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	projectRoot := t.TempDir()
	projectResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects", map[string]string{"path": projectRoot})
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if projectResponse.StatusCode != http.StatusCreated {
		projectResponse.Body.Close()
		t.Fatalf("create project status = %d", projectResponse.StatusCode)
	}
	decodeServerTestJSON(t, projectResponse, &created)
	chatResponse := postJSONForServerTest(t, server.URL+"/__solomon/projects/"+created.Project.ID+"/chats", map[string]string{})
	var createdChat struct {
		ID string `json:"id"`
	}
	if chatResponse.StatusCode != http.StatusCreated {
		chatResponse.Body.Close()
		t.Fatalf("create chat status = %d", chatResponse.StatusCode)
	}
	decodeServerTestJSON(t, chatResponse, &createdChat)

	sess, err := chatstore.ReadSession(created.Project.ID, createdChat.ID)
	if err != nil {
		t.Fatal(err)
	}

	sess.Messages = []chatstore.Message{
		{Role: "user", Content: "Continue", CreatedAt: time.Now()},
		{Role: "assistant", PromptTokens: 177974, CachedPromptTokens: 177792, ResponseTokens: 154, TurnTotalTokens: 178128},
		{Role: "assistant", PromptTokens: 178870, CachedPromptTokens: 178048, ReasoningTokens: 163, ResponseTokens: 1155, TurnTotalTokens: 180188},
		{Role: "assistant", PromptTokens: 180320, CachedPromptTokens: 180096, ReasoningTokens: 23, ResponseTokens: 233, TurnTotalTokens: 180576},
		{Role: "assistant", Content: "Done", PromptTokens: 180689, CachedPromptTokens: 180480, ResponseTokens: 53, TurnTotalTokens: 180742},
	}
	for _, saved := range []bool{false, true} {
		sess.Messages[4].TurnDisplaySaved = saved
		sess.Messages[4].TurnContextTokens = 717853
		sess.Messages[4].TurnCachedPromptTokens = 716416
		sess.Messages[4].TurnReasonTokens = 186
		sess.Messages[4].TurnRespTokens = 1595
		sess.Messages[4].TurnTotalDisplay = 719634
		if err := chatstore.WriteSession(created.Project.ID, sess); err != nil {
			t.Fatal(err)
		}
		response, err := http.Get(server.URL + "/__solomon/projects/" + created.Project.ID + "/chats/" + createdChat.ID)
		if err != nil {
			t.Fatal(err)
		}
		var opened struct {
			Messages []struct {
				Stats *struct {
					ContextTokens      int64 `json:"contextTokens"`
					TotalTokens        int64 `json:"totalTokens"`
					CachedPromptTokens int64 `json:"cachedPromptTokens"`
				} `json:"stats"`
			} `json:"messages"`
		}
		decodeServerTestJSON(t, response, &opened)
		stats := opened.Messages[len(opened.Messages)-1].Stats
		if stats == nil || stats.ContextTokens != 180689 {
			t.Fatalf("saved=%v stats=%+v", saved, stats)
		}
		if stats.TotalTokens != 180742 || stats.CachedPromptTokens != 180480 {
			t.Fatalf("aggregate=%+v", stats)
		}
	}
}
