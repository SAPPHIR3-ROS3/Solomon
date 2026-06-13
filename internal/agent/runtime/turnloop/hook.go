package turnloop

import "sync"

var (
	generationStopMu sync.Mutex
	generationStop   func(error)
)

func setGenerationStop(fn func(error)) {
	generationStopMu.Lock()
	generationStop = fn
	generationStopMu.Unlock()
}

func clearGenerationStop() {
	generationStopMu.Lock()
	generationStop = nil
	generationStopMu.Unlock()
}

func StopForTest(stopErr error) {
	generationStopMu.Lock()
	fn := generationStop
	generationStopMu.Unlock()
	if fn != nil {
		fn(stopErr)
	}
}