package research

import "errors"

func ParseJSONArrayForTest(s string) []string { return parseJSONArray(s) }

func IsLowQualityForTest(s string) bool { return isLowQuality(s) }

func ShouldStopFromLLMForTest(s string) bool { return shouldStopFromLLM(s) }

func FallbackQueriesForTest(question string, round int) []string { return fallbackQueries(question, round) }

func PauseLLMErrorForTest(detail string) error { return pauseLLMError(detail) }

func IsPausedLLMForTest(err error) bool { return errors.Is(err, ErrPausedLLM) }

func FormatResearchErrorForTest(msg string) string { return FormatResearchError(msg) }

func ApplyURLAttemptForTest(rec *JobRecord, ev ProgressEvent) { applyURLAttempt(rec, ev) }
