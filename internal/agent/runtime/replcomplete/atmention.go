package replcomplete

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
)

var atIndexCache = atmention.NewIndexCache()

func AtIndexEntries(ctx context.Context, env ReplCompleteEnv) ([]atmention.Entry, error) {
	if env.ProjRoot == "" {
		return nil, nil
	}
	return atIndexCache.Get(ctx, env.ProjRoot)
}

func (c *replCompleter) completeAtMention(line []rune, pos int) ([][]rune, int, bool) {
	ctx := atmention.AtContextAt(line, pos)
	if !ctx.Active {
		return nil, 0, false
	}
	entries, err := AtIndexEntries(context.Background(), c.env)
	if err != nil || len(entries) == 0 {
		return nil, 0, false
	}
	matches := atmention.MatchQuery(ctx.Query, entries, atmention.MaxPickerResults)
	if len(matches) == 0 {
		return nil, 0, false
	}
	sel := matches[0]
	tag := "@" + atmention.ShortTag(sel.RelPath, entries)
	query := ctx.Query
	if len(tag) <= 1+len(query) {
		return nil, 0, false
	}
	suffix := []rune(tag[1+len(query):])
	return [][]rune{suffix}, pos - ctx.QueryStart, true
}
