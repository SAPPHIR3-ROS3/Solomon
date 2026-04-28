package test

import (
	"context"
	"io"
	"strings"

	"solomon/internal/agent/commands"
	"solomon/internal/chatstore"
	"solomon/internal/config"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func testDeps(sess *chatstore.Session) commands.Deps {
	if sess == nil {
		sess = &chatstore.Session{}
	}
	thresh := config.DefaultCompactionThresholdTokens
	return commands.Deps{
		Ctx:   context.Background(),
		Out:   io.Discard,
		Stdin: strings.NewReader(""),
		Cfg: &config.Root{Current: config.Current{Provider: "p", Model: "m"}, Providers: []config.Provider{{Name: "p", BaseURL: "http://127.0.0.1:9", APIKey: "k"}}}, 
		SaveCfg: func() error { return nil },
		ProjHex: "0000000000000000000000000000000000000000000000000000000000000000",
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
	}
}
