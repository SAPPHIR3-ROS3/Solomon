package sdk

func GrepLines(pattern string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern})
}

func GrepLinesIn(dir, pattern string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern})
}

func GrepLinesIgnoreCase(pattern string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, caseInsensitive: true})
}

func GrepLinesInIgnoreCase(dir, pattern string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, caseInsensitive: true})
}

func GrepLinesMultiline(pattern string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, multiline: true})
}

func GrepLinesInMultiline(dir, pattern string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, multiline: true})
}

func GrepLinesPathGlob(pattern, pathGlob string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, pathGlob: pathGlob})
}

func GrepLinesInPathGlob(dir, pattern, pathGlob string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, pathGlob: pathGlob})
}

func GrepLinesLimit(pattern string, headLimit int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, headLimit: headLimit})
}

func GrepLinesInLimit(dir, pattern string, headLimit int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, headLimit: headLimit})
}

func GrepLinesWithContext(pattern string, contextLines int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, context: contextLines})
}

func GrepLinesInWithContext(dir, pattern string, contextLines int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, context: contextLines})
}

func GrepLinesContextBefore(pattern string, before int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, contextBefore: before})
}

func GrepLinesContextAfter(pattern string, after int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, contextAfter: after})
}

func GrepLinesContextBeforeAfter(pattern string, before, after int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, contextBefore: before, contextAfter: after})
}

func GrepLinesTimeout(pattern string, secs int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, timeoutSecs: secs})
}

func GrepLinesInTimeout(dir, pattern string, secs int) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, timeoutSecs: secs})
}

func GrepCountEntries(pattern string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{pattern: pattern})
}

func GrepCountEntriesIn(dir, pattern string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{dir: dir, pattern: pattern})
}

func GrepCountEntriesIgnoreCase(pattern string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{pattern: pattern, caseInsensitive: true})
}

func GrepCountEntriesInIgnoreCase(dir, pattern string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{dir: dir, pattern: pattern, caseInsensitive: true})
}

func GrepCountEntriesTimeout(pattern string, secs int) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{pattern: pattern, timeoutSecs: secs})
}

func GrepCountEntriesInTimeout(dir, pattern string, secs int) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{dir: dir, pattern: pattern, timeoutSecs: secs})
}
