package codex

var subModelCatalog = []string{
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-4.1",
	"gpt-4.1-mini",
	"gpt-4.1-nano",
	"gpt-5",
	"gpt-5-mini",
	"gpt-5.1",
	"gpt-5.2",
	"gpt-5.3-codex",
	"gpt-5.4",
	"gpt-5.4-mini",
}

func SubModelCatalog() []string {
	out := make([]string, len(subModelCatalog))
	copy(out, subModelCatalog)
	return out
}
