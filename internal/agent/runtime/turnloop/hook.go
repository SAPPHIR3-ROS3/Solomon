package turnloop

var generationStop func(error)

func setGenerationStop(fn func(error)) {
	generationStop = fn
}

func clearGenerationStop() {
	generationStop = nil
}

func StopForTest(stopErr error) {
	if generationStop != nil {
		generationStop(stopErr)
	}
}
