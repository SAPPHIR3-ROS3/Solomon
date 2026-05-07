package test

import (
	"context"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func testDeps(sess *chatstore.Session) commands.Deps {
	if sess == nil {
		sess = &chatstore.Session{}
	}
	thresh := config.DefaultCompactionThresholdTokens
	var shellFirst bool
	return commands.Deps{
		Ctx:   context.Background(),
		Out:   io.Discard,
		Stdin: strings.NewReader(""),
		Cfg: &config.Root{Current: config.Current{Provider: "p", Model: "m"}, Providers: []config.Provider{{Name: "p", BaseURL: "http://127.0.0.1:9", APIKey: "k"}}}, 
		SaveCfg: func() error { return nil },
		ProjHex:  "0000000000000000000000000000000000000000000000000000000000000000",
		ProjRoot: "/tmp/solomon-test-proj",
		Session: func() *chatstore.Session {
			return sess
		},
		SetSession: func(s *chatstore.Session) { *sess = *s },

		SetMode: func(string) {},
		GetMode: func() string { return "build" },

		ApplyCurrentModel: func(_, _ string) error { return nil },
		Model:             func() string { return "m" },
		Provider: func() *config.Provider {
			return &config.Provider{Name: "p", BaseURL: "http://127.0.0.1:9", APIKey: "k"}
		},
		CompactionThresholdTokens:    func() int64 { return thresh },
		SetCompactionThresholdTokens: func(n int64) { thresh = n },
		Client: openai.NewClient(option.WithAPIKey("x"), option.WithBaseURL("http://127.0.0.1:9/")),
		CheckpointGoto: func(*checkpoint.FullCheckpointID) error { return nil },

		GetReplShellFirst: func() bool { return shellFirst },
		SetReplShellFirst: func(v bool) { shellFirst = v },
	}
}
