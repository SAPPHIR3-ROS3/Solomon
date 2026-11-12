package main

import (
	"fmt"
	"strings"
)

func (m model) sectionKey(index int, title string) string {
	prefix := "chat."
	if strings.HasPrefix(m.sectionPrefix, "component-") {
		prefix = strings.Split(m.sectionPrefix, ".")[0] + "."
	}
	return fmt.Sprintf("%s%d.%s", prefix, index, title)
}

func (m model) toolSection(role, title, body string, width int, suffix ...string) string {
	key := m.sectionPrefix + title
	if len(suffix) > 0 {
		title += " · " + suffix[0]
	}
	if m.sections[key] {
		return clickZone("section:"+key, text(title+" ▸", artifactColor(role)))
	}
	caption := title + " ▾"
	rendered := m.artifact(role, caption, body, width)
	first, rest, found := strings.Cut(rendered, "\n")
	first = clickZone("section:"+key, first)
	if found {
		return first + "\n" + rest
	}
	return first
}
