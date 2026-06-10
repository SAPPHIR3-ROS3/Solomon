package host

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

type CountingCaller struct {
	Inner     ToolCaller
	MaxCalls  int
	count     atomic.Int32
	LastError error
}

func (c *CountingCaller) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	n := c.count.Add(1)
	if c.MaxCalls > 0 && int(n) > c.MaxCalls {
		err := fmt.Errorf("max tool calls exceeded (%d)", c.MaxCalls)
		c.LastError = err
		return nil, err
	}
	return c.Inner.Call(ctx, name, args)
}

func (c *CountingCaller) ToolCalls() int {
	return int(c.count.Load())
}
