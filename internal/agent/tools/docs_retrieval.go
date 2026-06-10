package tools

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/docs"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"

	"github.com/openai/openai-go/v2"
)

func signatureDocsRetrieval(query string) {}

const docsRetrievalDescription = `Search embedded Solomon documentation. Generic queries return BM25-ranked snippets with source paths (up to 5 articles). Specific path or filename queries return the full article. Short queries (<=5 words) with very high match score (>=0.9) also return the full winning article. Paths omit the docs/ prefix (e.g. user-guide/configuration.md); docs wiki index is docs-index.md (query README.md or readme).`

type docsRetrievalArgs struct {
	Query string `json:"query"`
}

func docsRetrievalOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("docsRetrieval", docsRetrievalDescription, map[string]any{
		"query": map[string]any{"type": "string", "description": "Search terms or documentation path/filename"},
	}, []string{"query"})
}

func appendDocsRetrievalDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureDocsRetrieval)
	if err != nil {
		return err
	}
	b.addBlock("docsRetrieval", docsRetrievalDescription, sig)
	return nil
}

func execDocsRetrieval(env *Env, raw json.RawMessage) (any, error) {
	var a docsRetrievalArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	opts := docs.Options{
		MinNormalizedScore: config.EffectiveDocSearchMinNorm(env.Cfg),
		FullArticleScore:   config.EffectiveDocSearchFullArticleScore(env.Cfg),
	}
	return docs.Retrieve(a.Query, opts)
}
