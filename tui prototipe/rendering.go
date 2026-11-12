package main

import "strings"

type componentRenderKey struct {
	width, height, variant, index, state, mode, modelChoice, reasoningChoice, suggestion int
	revision                                                                             uint64
	draft, menu                                                                          string
	hidden                                                                               bool
	foldVersion, subchatScroll                                                           int
}
type componentRenderCache struct {
	key   componentRenderKey
	valid bool
	body  string
	rows  []string
}

func (m model) componentBody(width, height int) (string, []string) {
	key := componentRenderKey{width: width, height: height, variant: m.variant, index: m.componentIndex, state: m.componentState, mode: m.selected, modelChoice: m.modelChoice, reasoningChoice: m.reasoningChoice, suggestion: m.suggestion, revision: interfaceRevision, draft: string(m.draft), menu: m.guiMenu, hidden: m.menuHidden, foldVersion: m.foldVersion, subchatScroll: m.subchatScroll}
	if m.componentCache != nil && m.componentCache.valid && m.componentCache.key == key {
		return m.componentCache.body, m.componentCache.rows
	}
	body := m.renderComponent(width, height)
	rows := strings.Split(body, "\n")
	if m.componentCache != nil {
		*m.componentCache = componentRenderCache{key: key, valid: true, body: body, rows: rows}
	}
	return body, rows
}

type entryRenderKey struct {
	width, variant                                            int
	details                                                   bool
	sourceClosed, resultClosed, queryClosed, notificationOpen bool
	revision                                                  uint64
	message                                                   *message
	streaming                                                 string
}
type entryRender struct {
	key  entryRenderKey
	rows []string
}
