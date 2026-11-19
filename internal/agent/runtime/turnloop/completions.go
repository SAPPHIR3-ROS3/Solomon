package turnloop

import "context"

func deliverSubagentCompletions(ctx context.Context, h Host, wait bool) (bool, error) {
	if host, ok := h.(interface {
		DeliverSubagentCompletions(context.Context, bool) (bool, error)
	}); ok {
		return host.DeliverSubagentCompletions(ctx, wait)
	}
	return false, nil
}
