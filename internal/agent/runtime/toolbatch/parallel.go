package toolbatch

import (
	"context"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

// Result keeps the result for the invocation at the same index in the input
// batch. Keeping that association lets callers execute concurrently while
// still appending tool messages in the order requested by the model.
type Result struct {
	Value any
	Err   error
}

// CanRunConcurrently reports whether a batch can use the shared runtime
// concurrently. These tools deliberately remain serialized with any sibling
// call because they mutate process-wide runtime state while they run.
func CanRunConcurrently(invs []tooling.Invocation) bool {
	for _, inv := range invs {
		switch inv.Name {
		case "subagent", "switchMode":
			return false
		}
	}
	return true
}

// Execute starts every invocation in the batch before waiting for any result.
// The executor is responsible for honoring ctx. A failed invocation does not
// cancel its siblings: tool failures are represented by the corresponding
// Result so the model receives one result for every requested tool call.
func Execute(ctx context.Context, invs []tooling.Invocation, exec func(context.Context, tooling.Invocation) (any, error)) []Result {
	results := make([]Result, len(invs))
	var wg sync.WaitGroup
	wg.Add(len(invs))
	for i := range invs {
		go func(i int) {
			defer wg.Done()
			results[i].Value, results[i].Err = exec(ctx, invs[i])
		}(i)
	}
	wg.Wait()
	return results
}
