//go:build desktop

package main

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/gui/desktop/app"

func prepareDesktop() (string, error) {
	return "", nil
}

func launchDesktopWindow() error {
	app.Run()
	return nil
}
