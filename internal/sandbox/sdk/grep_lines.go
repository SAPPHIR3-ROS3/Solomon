package sdk

func GrepLines(pattern, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, intent: intent})
}

func GrepLinesIn(dir, pattern, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, intent: intent})
}

func GrepLinesIgnoreCase(pattern, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepLinesInIgnoreCase(dir, pattern, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepLinesMultiline(pattern, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, multiline: true, intent: intent})
}

func GrepLinesInMultiline(dir, pattern, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, multiline: true, intent: intent})
}

func GrepLinesPathGlob(pattern, pathGlob, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, pathGlob: pathGlob, intent: intent})
}

func GrepLinesInPathGlob(dir, pattern, pathGlob, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, pathGlob: pathGlob, intent: intent})
}

func GrepLinesLimit(pattern string, headLimit int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, headLimit: headLimit, intent: intent})
}

func GrepLinesInLimit(dir, pattern string, headLimit int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, headLimit: headLimit, intent: intent})
}

func GrepLinesWithContext(pattern string, contextLines int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, context: contextLines, intent: intent})
}

func GrepLinesInWithContext(dir, pattern string, contextLines int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, context: contextLines, intent: intent})
}

func GrepLinesContextBefore(pattern string, before int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, contextBefore: before, intent: intent})
}

func GrepLinesContextAfter(pattern string, after int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, contextAfter: after, intent: intent})
}

func GrepLinesContextBeforeAfter(pattern string, before, after int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, contextBefore: before, contextAfter: after, intent: intent})
}

func GrepLinesTimeout(pattern string, secs int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{pattern: pattern, timeoutSecs: secs, intent: intent})
}

func GrepLinesInTimeout(dir, pattern string, secs int, intent string) ([]GrepLine, error) {
	return grepLinesCall(grepTextQuery{dir: dir, pattern: pattern, timeoutSecs: secs, intent: intent})
}

func GrepCountEntries(pattern, intent string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{pattern: pattern, intent: intent})
}

func GrepCountEntriesIn(dir, pattern, intent string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{dir: dir, pattern: pattern, intent: intent})
}

func GrepCountEntriesIgnoreCase(pattern, intent string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepCountEntriesInIgnoreCase(dir, pattern, intent string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{dir: dir, pattern: pattern, caseInsensitive: true, intent: intent})
}

func GrepCountEntriesTimeout(pattern string, secs int, intent string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{pattern: pattern, timeoutSecs: secs, intent: intent})
}

func GrepCountEntriesInTimeout(dir, pattern string, secs int, intent string) ([]GrepCountEntry, error) {
	return grepCountEntriesCall(grepTextQuery{dir: dir, pattern: pattern, timeoutSecs: secs, intent: intent})
}
