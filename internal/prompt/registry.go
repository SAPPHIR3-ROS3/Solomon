package prompt

var embeddedTemplates = map[string]string{
	"agent":            agentRaw,
	"chat":             chatRaw,
	"title":            titleRaw,
	"summarize":        summarizeRaw,
	"summarize_system": summarizeSystemRaw,
	"images":           imagesWorkflowRaw,
	"atmention":        atMentionWorkflowRaw,
}

func TemplateNames() []string {
	names := make([]string, 0, len(embeddedTemplates))
	for name := range embeddedTemplates {
		names = append(names, name)
	}
	return names
}

func EmbeddedTemplate(name string) (string, bool) {
	s, ok := embeddedTemplates[name]
	return s, ok
}
