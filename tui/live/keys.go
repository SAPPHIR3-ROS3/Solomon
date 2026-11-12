package live

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func defaultKeys() map[string]map[string][]string {
	return map[string]map[string][]string{
		"global": {
			"quit": {"ctrl+q"}, "new_thread": {"ctrl+n"}, "toggle_details": {"ctrl+d"}, "toggle_artifact": {"ctrl+o"}, "commands": {"ctrl+k"}, "models": {"ctrl+p"}, "reasoning": {"ctrl+r"}, "cancel": {"esc"},
			"scroll_up": {"ctrl+up"}, "scroll_down": {"ctrl+down"}, "page_up": {"pgup"}, "page_down": {"pgdown"}, "scroll_start": {"home", "ctrl+home"}, "scroll_end": {"end", "ctrl+end"}, "artifact_previous": {"alt+up"}, "artifact_next": {"alt+down"},
			"layout_1": {"f1"}, "layout_2": {"f2"}, "layout_3": {"f3"}, "layout_4": {"f4"}, "archive": {"f5"}, "gallery": {"f6"}, "components": {"f7"},
		},
		"composer":    {"send": {"enter"}, "newline": {"ctrl+j", "alt+enter"}, "history_previous": {"up"}, "history_next": {"down"}, "clear_draft": {"ctrl+u"}, "backspace": {"backspace", "ctrl+h"}, "toggle_mode": {"tab"}},
		"suggestions": {"previous": {"up"}, "next": {"down"}, "complete": {"tab"}, "accept": {"enter"}, "dismiss": {"esc"}},
		"picker":      {"previous": {"up"}, "next": {"down"}, "accept": {"enter"}, "dismiss": {"esc"}},
		"components":  {"previous": {"up"}, "next": {"down"}, "state": {"tab"}, "dismiss": {"esc"}},
	}
}

var keyPattern = regexp.MustCompile("^(?:(?:ctrl|alt|shift)\\+)*(?:[a-z0-9]|f(?:[1-9]|1[0-9]|20)|enter|esc|tab|space|backspace|delete|up|down|left|right|home|end|pgup|pgdown)$")

func NormalizeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "ctrl+m":
		return "enter"
	case "ctrl+i":
		return "tab"
	case "shift+enter":
		return "alt+enter"
	}
	parts := strings.Split(key, "+")
	if len(parts) < 2 {
		return key
	}
	modifiers := map[string]bool{}
	for _, part := range parts[:len(parts)-1] {
		modifiers[part] = true
	}
	prefix := []string{}
	for _, modifier := range []string{"ctrl", "alt", "shift"} {
		if modifiers[modifier] {
			prefix = append(prefix, modifier)
			delete(modifiers, modifier)
		}
	}
	if len(modifiers) > 0 {
		return key
	}
	return strings.Join(append(prefix, parts[len(parts)-1]), "+")
}
func (c Config) Action(context, key string) string {
	key = NormalizeKey(key)
	if key == "ctrl+c" {
		return "quit"
	}
	for _, scope := range []string{context, "global"} {
		actions := c.Keys[scope]
		names := make([]string, 0, len(actions))
		for name := range actions {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, binding := range actions[name] {
				if NormalizeKey(binding) == key {
					return name
				}
			}
		}
	}
	return ""
}
func (c Config) Hint(context, action string) string {
	if keys := c.Keys[context][action]; len(keys) > 0 {
		return keys[0]
	}
	return "unbound"
}
func (c Config) validateKeys() error {
	defaults := defaultKeys()
	globals := map[string]string{}
	for context, actions := range c.Keys {
		allowed, ok := defaults[context]
		if !ok {
			return fmt.Errorf("unknown key context %q", context)
		}
		seen := map[string]string{}
		for action, keys := range actions {
			if _, ok := allowed[action]; !ok {
				return fmt.Errorf("unknown action %q in %s", action, context)
			}
			if len(keys) > 8 {
				return fmt.Errorf("too many keys for %s.%s", context, action)
			}
			for _, key := range keys {
				normalized := NormalizeKey(key)
				if !keyPattern.MatchString(normalized) {
					return fmt.Errorf("invalid key %q for %s.%s", key, context, action)
				}
				if normalized == "ctrl+c" || normalized == "f17" || normalized == "f18" {
					return fmt.Errorf("key %q is reserved for emergency exit or mouse transport", key)
				}
				if other, ok := seen[normalized]; ok && other != action {
					return fmt.Errorf("key %q conflicts between %s and %s in %s", key, other, action, context)
				}
				seen[normalized] = action
				if context == "global" {
					globals[normalized] = action
				}
			}
		}
	}
	for context, actions := range c.Keys {
		if context == "global" {
			continue
		}
		for action, keys := range actions {
			for _, key := range keys {
				if global, exists := globals[NormalizeKey(key)]; exists && !(global == "cancel" && action == "dismiss") {
					return fmt.Errorf("key %q in %s.%s conflicts with global.%s", key, context, action, global)
				}
			}
		}
	}
	return nil
}
