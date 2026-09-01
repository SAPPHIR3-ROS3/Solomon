package sdk

func FindInfo(pattern string, files bool, intent string) (FindResult, error) {
	return findInfoCall("", pattern, files, 0, intent)
}

func FindInInfo(dir, pattern string, files bool, intent string) (FindResult, error) {
	return findInfoCall(dir, pattern, files, 0, intent)
}

func FindTimeoutInfo(pattern string, files bool, secs int, intent string) (FindResult, error) {
	return findInfoCall("", pattern, files, secs, intent)
}

func FindInTimeoutInfo(dir, pattern string, files bool, secs int, intent string) (FindResult, error) {
	return findInfoCall(dir, pattern, files, secs, intent)
}
