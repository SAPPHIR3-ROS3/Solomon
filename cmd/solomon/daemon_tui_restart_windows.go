package main

// The coordinator reopens registered Windows transports after replacing the
// executable; exiting here releases the old binary for installation.
func restartTUIAfterDaemonUpdate([]string, int) error { return nil }
