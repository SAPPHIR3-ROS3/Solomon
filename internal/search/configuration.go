package search

import (
	"net/url"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/validation"
)

func ValidateConfiguration(engine string, extras map[string]any) error {
	engine = strings.ToLower(strings.TrimSpace(engine))
	var required []string
	switch engine {
	case "brave", "bing":
		required = []string{"apiKey"}
	case "googlepse":
		required = []string{"apiKey", "cx"}
	case "searxng":
		required = []string{"baseURL"}
	}
	for _, key := range required {
		raw, exists := extras[key]
		if !exists || raw == nil {
			return validation.Configuration("webSearch", engine+"."+key, "required setting is missing; configure this provider or supply the corresponding extras field")
		}
		value, ok := raw.(string)
		if !ok {
			return validation.Argument("webSearch", "extras."+key, "must be a string")
		}
		if strings.TrimSpace(value) == "" {
			return validation.Configuration("webSearch", engine+"."+key, "required setting must not be empty")
		}
	}
	for _, key := range []string{"baseURL", "endpoint"} {
		if raw, exists := extras[key]; exists && ((engine == "searxng" && key == "baseURL") || (engine == "bing" && key == "endpoint")) {
			value, ok := raw.(string)
			if !ok {
				return validation.Argument("webSearch", "extras."+key, "must be a string")
			}
			u, err := url.Parse(strings.TrimSpace(value))
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return validation.Configuration("webSearch", engine+"."+key, "must be an absolute HTTP or HTTPS URL")
			}
		}
	}
	return nil
}
