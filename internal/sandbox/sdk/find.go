package sdk

func Find(pattern string, files bool) (string, error) {
	return findRawCall("", pattern, files, 0)
}

func FindIn(dir, pattern string, files bool) (string, error) {
	return findRawCall(dir, pattern, files, 0)
}

func FindTimeout(pattern string, files bool, secs int) (string, error) {
	return findRawCall("", pattern, files, secs)
}

func FindInTimeout(dir, pattern string, files bool, secs int) (string, error) {
	return findRawCall(dir, pattern, files, secs)
}

func Glob(pattern string) ([]string, error) {
	return globCall(globQuery{pattern: pattern})
}

func GlobIn(dir, pattern string) ([]string, error) {
	return globCall(globQuery{dir: dir, pattern: pattern})
}

func GlobLimit(pattern string, headLimit int) ([]string, error) {
	return globCall(globQuery{pattern: pattern, headLimit: headLimit})
}

func GlobInLimit(dir, pattern string, headLimit int) ([]string, error) {
	return globCall(globQuery{dir: dir, pattern: pattern, headLimit: headLimit})
}

func GlobTimeout(pattern string, secs int) ([]string, error) {
	return globCall(globQuery{pattern: pattern, timeoutSecs: secs})
}

func GlobInTimeout(dir, pattern string, secs int) ([]string, error) {
	return globCall(globQuery{dir: dir, pattern: pattern, timeoutSecs: secs})
}

func Grep(pattern string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern})
}

func GrepIn(dir, pattern string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern})
}

func GrepIgnoreCase(pattern string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, caseInsensitive: true})
}

func GrepInIgnoreCase(dir, pattern string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, caseInsensitive: true})
}

func GrepMultiline(pattern string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, multiline: true})
}

func GrepInMultiline(dir, pattern string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, multiline: true})
}

func GrepPathGlob(pattern, pathGlob string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, pathGlob: pathGlob})
}

func GrepInPathGlob(dir, pattern, pathGlob string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, pathGlob: pathGlob})
}

func GrepLimit(pattern string, headLimit int) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, headLimit: headLimit})
}

func GrepInLimit(dir, pattern string, headLimit int) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, headLimit: headLimit})
}

func GrepWithContext(pattern string, contextLines int) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, context: contextLines})
}

func GrepInWithContext(dir, pattern string, contextLines int) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, context: contextLines})
}

func GrepContextBefore(pattern string, before int) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, contextBefore: before})
}

func GrepContextAfter(pattern string, after int) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, contextAfter: after})
}

func GrepContextBeforeAfter(pattern string, before, after int) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, contextBefore: before, contextAfter: after})
}

func GrepTimeout(pattern string, secs int) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, timeoutSecs: secs})
}

func GrepInTimeout(dir, pattern string, secs int) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, timeoutSecs: secs})
}

func GrepCount(pattern string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, outputMode: "count"})
}

func GrepFiles(pattern string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{pattern: pattern})
}

func GrepFilesIn(dir, pattern string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{dir: dir, pattern: pattern})
}

func GrepFilesIgnoreCase(pattern string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{pattern: pattern, caseInsensitive: true})
}

func GrepFilesInIgnoreCase(dir, pattern string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{dir: dir, pattern: pattern, caseInsensitive: true})
}

func GrepFilesTimeout(pattern string, secs int) ([]string, error) {
	return grepPathsCall(grepPathsQuery{pattern: pattern, timeoutSecs: secs})
}

func GrepFilesInTimeout(dir, pattern string, secs int) ([]string, error) {
	return grepPathsCall(grepPathsQuery{dir: dir, pattern: pattern, timeoutSecs: secs})
}
