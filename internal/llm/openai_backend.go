package llm

import (
	"context"
	"io"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
)

type OpenAIBackend struct {
	Client openai.Client
}

func (b *OpenAIBackend) Protocol() Protocol { return ProtocolOpenAI }

func (b *OpenAIBackend) buildParams(req TurnRequest) openai.ChatCompletionNewParams {
	tools := openaiToolsFromDefs(req.Tools)
	p := openai.ChatCompletionNewParams{
		Model:             shared.ChatModel(req.Model),
		Messages:          MessageParams(req.System, req.Messages, req.ImageFiles),
		Tools:             tools,
		ParallelToolCalls: param.NewOpt(req.ParallelToolCalls),
	}
	ApplyProviderTurnParams(ProtocolOpenAI, req.Cfg, &p, req.ForceDisableReasoning)
	return p
}

func (b *OpenAIBackend) StreamTurn(ctx context.Context, req TurnRequest, contentOut io.Writer, opts StreamOpts) (AssistantTurnResult, error) {
	return StreamAssistantTurn(ctx, b.Client, b.buildParams(req), contentOut, opts)
}

func (b *OpenAIBackend) StreamText(ctx context.Context, req SimpleCompletionRequest, contentOut io.Writer, opts StreamOpts) (string, UsageStats, error) {
	p := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(req.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.System),
			openai.UserMessage(req.User),
		},
	}
	ApplyProviderSimpleParams(ProtocolOpenAI, req.Cfg, &p, req.ForceDisableReasoning)
	return StreamText(ctx, b.Client, p, contentOut, opts)
}

func (b *OpenAIBackend) CompleteText(ctx context.Context, req SimpleCompletionRequest) (string, error) {
	p := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(req.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.System),
			openai.UserMessage(req.User),
		},
	}
	ApplyProviderSimpleParams(ProtocolOpenAI, req.Cfg, &p, req.ForceDisableReasoning)
	resp, err := b.Client.Chat.Completions.New(ctx, p)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}

func (b *OpenAIBackend) ListModels(ctx context.Context) ([]string, error) {
	return nil, nil
}

func openaiToolsFromDefs(defs []ToolDef) []openai.ChatCompletionToolUnionParam {
	var out []openai.ChatCompletionToolUnionParam
	for _, d := range defs {
		props := d.Parameters
		if props == nil {
			props = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        d.Name,
					Description: openai.String(d.Description),
					Parameters:  openai.FunctionParameters(props),
				},
			},
		})
	}
	return out
}
