package sdk

func Find(pattern string, files bool, intent string) (string, error) {
	return findRawCall("", pattern, files, 0, intent)
}

func FindIn(dir, pattern string, files bool, intent string) (string, error) {
	return findRawCall(dir, pattern, files, 0, intent)
}

func FindTimeout(pattern string, files bool, secs int, intent string) (string, error) {
	return findRawCall("", pattern, files, secs, intent)
}

func FindInTimeout(dir, pattern string, files bool, secs int, intent string) (string, error) {
	return findRawCall(dir, pattern, files, secs, intent)
}

func Glob(pattern, intent string) ([]string, error) {
	return globCall(globQuery{pattern: pattern, intent: intent})
}

func GlobIn(dir, pattern, intent string) ([]string, error) {
	return globCall(globQuery{dir: dir, pattern: pattern, intent: intent})
}

func GlobLimit(pattern string, headLimit int, intent string) ([]string, error) {
	return globCall(globQuery{pattern: pattern, headLimit: headLimit, intent: intent})
}

func GlobInLimit(dir, pattern string, headLimit int, intent string) ([]string, error) {
	return globCall(globQuery{dir: dir, pattern: pattern, headLimit: headLimit, intent: intent})
}

func GlobTimeout(pattern string, secs int, intent string) ([]string, error) {
	return globCall(globQuery{pattern: pattern, timeoutSecs: secs, intent: intent})
}

func GlobInTimeout(dir, pattern string, secs int, intent string) ([]string, error) {
	return globCall(globQuery{dir: dir, pattern: pattern, timeoutSecs: secs, intent: intent})
}

func Grep(pattern, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, intent: intent})
}

func GrepIn(dir, pattern, intent string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, intent: intent})
}

func GrepIgnoreCase(pattern, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepInIgnoreCase(dir, pattern, intent string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepMultiline(pattern, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, multiline: true, intent: intent})
}

func GrepInMultiline(dir, pattern, intent string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, multiline: true, intent: intent})
}

func GrepPathGlob(pattern, pathGlob, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, pathGlob: pathGlob, intent: intent})
}

func GrepInPathGlob(dir, pattern, pathGlob, intent string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, pathGlob: pathGlob, intent: intent})
}

func GrepLimit(pattern string, headLimit int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, headLimit: headLimit, intent: intent})
}

func GrepInLimit(dir, pattern string, headLimit int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, headLimit: headLimit, intent: intent})
}

func GrepWithContext(pattern string, contextLines int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, context: contextLines, intent: intent})
}

func GrepInWithContext(dir, pattern string, contextLines int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, context: contextLines, intent: intent})
}

func GrepContextBefore(pattern string, before int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, contextBefore: before, intent: intent})
}

func GrepContextAfter(pattern string, after int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, contextAfter: after, intent: intent})
}

func GrepContextBeforeAfter(pattern string, before, after int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, contextBefore: before, contextAfter: after, intent: intent})
}

func GrepTimeout(pattern string, secs int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, timeoutSecs: secs, intent: intent})
}

func GrepInTimeout(dir, pattern string, secs int, intent string) (string, error) {
	return grepTextCall(grepTextQuery{dir: dir, pattern: pattern, timeoutSecs: secs, intent: intent})
}

func GrepCount(pattern, intent string) (string, error) {
	return grepTextCall(grepTextQuery{pattern: pattern, outputMode: "count", intent: intent})
}

func GrepFiles(pattern, intent string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{pattern: pattern, intent: intent})
}

func GrepFilesIn(dir, pattern, intent string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{dir: dir, pattern: pattern, intent: intent})
}

func GrepFilesIgnoreCase(pattern, intent string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepFilesInIgnoreCase(dir, pattern, intent string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{dir: dir, pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepFilesTimeout(pattern string, secs int, intent string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{pattern: pattern, timeoutSecs: secs, intent: intent})
}

func GrepFilesInTimeout(dir, pattern string, secs int, intent string) ([]string, error) {
	return grepPathsCall(grepPathsQuery{dir: dir, pattern: pattern, timeoutSecs: secs, intent: intent})
}
