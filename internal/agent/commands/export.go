package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func Export(d Deps, parts []string) error {
	if len(parts) < 2 {
		return fmt.Errorf("usage: /export current | /export last | /export <id|title>")
	}
	target := strings.TrimSpace(parts[1])
	if target == "" {
		return fmt.Errorf("usage: /export current | /export last | /export <id|title>")
	}
	rejectIfExists := strings.EqualFold(target, "last")
	sess, err := resolveExportSession(d, target)
	if err != nil {
		return err
	}
	if len(sess.Messages) == 0 {
		return fmt.Errorf("chat has no messages to export")
	}
	exportRoot, exportRootFromCfg, err := exportRootDir(d.Cfg)
	if err != nil {
		return err
	}
	day := time.Now().UTC()
	base := exportChatBasename(sess)
	plan, err := planExportPath(exportRoot, day, base, rejectIfExists)
	if err != nil {
		return err
	}
	model := d.Model()
	if d.Cfg != nil {
		model = d.Cfg.ModelDisplayName(d.Provider(), model)
	}
	providerName := ""
	if p := d.Provider(); p != nil {
		providerName = strings.TrimSpace(p.Name)
	}
	meta := markdownExportMeta{
		Title:           sess.Title,
		ExportedAt:      day,
		ChatID:          sess.ID,
		ProjectRoot:     d.ProjRoot,
		ProjectHex:      d.ProjHex,
		Model:           model,
		Provider:        providerName,
		Mode:            exportModeLabel(d),
		CreatedAt:       sess.CreatedAt,
		LastMessageAt:   sess.LastMessageAt,
		MessageCount:    len(sess.Messages),
		Ephemeral:       strings.EqualFold(target, "current") && d.GetEphemeralSession != nil && d.GetEphemeralSession(),
		ExportRoot:      exportRoot,
		ExportRootIsCfg: exportRootFromCfg,
	}
	var buf bytes.Buffer
	if err := writeMarkdownExport(&buf, meta, sess, usageStatsEnabled(d)); err != nil {
		return err
	}
	if err := writeExportFile(plan.AbsolutePath, buf.Bytes()); err != nil {
		return err
	}
	abs, err := filepathAbs(plan.AbsolutePath)
	if err != nil {
		abs = plan.AbsolutePath
	}
	PrintSystemf(d.Out, "exported chat to %s", abs)
	return nil
}

func exportModeLabel(d Deps) string {
	if d.GetMode == nil {
		return ""
	}
	return d.GetMode()
}

func exportRootDir(cfg *config.Root) (root string, fromCfg bool, err error) {
	if cfg != nil && strings.TrimSpace(cfg.Export.Path) != "" {
		root, err = cfg.EffectiveExportRoot()
		return root, true, err
	}
	if cfg == nil {
		cfg = &config.Root{}
	}
	root, err = cfg.EffectiveExportRoot()
	return root, false, err
}

func resolveExportSession(d Deps, target string) (*chatstore.Session, error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "current":
		sess := d.Session()
		if sess == nil {
			return nil, fmt.Errorf("no active session")
		}
		return sess, nil
	case "last":
		sess, err := chatstore.SessionWithLatestUserMessage(d.ProjHex)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("no saved chats yet")
			}
			return nil, err
		}
		return sess, nil
	default:
		sess, err := chatstore.ReadSession(d.ProjHex, target)
		if err != nil {
			sess, err = chatstore.FindByTitle(d.ProjHex, target)
		}
		if err != nil {
			return nil, err
		}
		return sess, nil
	}
}

func filepathAbs(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	return filepath.Abs(path)
}
