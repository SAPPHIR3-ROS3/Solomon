package sdk

func FindInfo(pattern string, files bool) (FindResult, error) {
	return findInfoCall("", pattern, files, 0)
}

func FindInInfo(dir, pattern string, files bool) (FindResult, error) {
	return findInfoCall(dir, pattern, files, 0)
}

func FindTimeoutInfo(pattern string, files bool, secs int) (FindResult, error) {
	return findInfoCall("", pattern, files, secs)
}

func FindInTimeoutInfo(dir, pattern string, files bool, secs int) (FindResult, error) {
	return findInfoCall(dir, pattern, files, secs)
}
