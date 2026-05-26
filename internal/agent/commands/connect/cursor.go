package connect

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/modelsapi"
)

type bootstrapOut struct {
	out io.Writer
}

func (c bootstrapOut) Print(msg string) {
	printSystem(Deps{Out: c.out}, msg)
}

func cursorAPI(d Deps) error {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	k, err := readLine(d, "Cursor API key: ")
	if err != nil {
		return err
	}
	k = strings.TrimSpace(k)
	if k == "" {
		return fmt.Errorf("missing API key")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	mgr := cursorint.DefaultManager()
	bo := bootstrapOut{out: d.Out}
	rawURL, err := mgr.Ensure(ctx, k, cwd, bo)
	if err != nil {
		return err
	}
	baseURL, err := config.NormalizeAPIBase(rawURL)
	if err != nil {
		return err
	}
	prov := config.Provider{
		Name:        config.ProviderNameCursorAPI,
		BaseURL:     baseURL,
		APIKey:      k,
		AuthKind:    config.AuthKindCursorAPI,
		APIProtocol: config.APIProtocolOpenAI,
	}
	ids, err := modelsapi.List(prov.BaseURL, k)
	if err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}
	ids = cursorint.FilterModelIDs(ids)
	prevProv := d.Cfg.Current.Provider
	prevModel := d.Cfg.Current.Model
	config.AppendOrUpdateProvider(d.Cfg, prov)
	if err := d.SaveCfg(); err != nil {
		return err
	}
	return pickModel(d, prevProv, prevModel, prov.Name, ids)
}
