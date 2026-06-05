//go:build !windows

package shellhist

func psReadLinePath() (string, historyKind) {
	return "", historyNone
}
