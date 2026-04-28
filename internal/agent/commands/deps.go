package commands

import (
	"context"
	"io"

	"solomon/internal/chatstore"
	"solomon/internal/config"

	"github.com/openai/openai-go/v2"
)

type Deps struct {
	Ctx context.Context

	Out   io.Writer
	Stdin io.Reader

	Cfg     *config.Root
	SaveCfg func() error

	ProjHex string

	Session    func() *chatstore.Session
	SetSession func(*chatstore.Session)

	SetMode func(string)
	GetMode func() string

	ApplyCurrentModel func(providerName, modelID string) error
	Model             func() string
	Provider          func() *config.Provider

	Client openai.Client
}
