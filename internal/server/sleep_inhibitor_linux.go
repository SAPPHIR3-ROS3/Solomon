package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
)

func acquireSleepInhibitor() (func(), error) {
	connection, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect to system power manager: %w", err)
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var fd dbus.UnixFD
	err = connection.Object("org.freedesktop.login1", "/org/freedesktop/login1").CallWithContext(ctx,
		"org.freedesktop.login1.Manager.Inhibit", 0, "sleep:idle:handle-lid-switch", "Solomon", "Keep background work running", "block").Store(&fd)
	if err != nil {
		return nil, fmt.Errorf("prevent sleep: %w", err)
	}
	file := os.NewFile(uintptr(fd), "solomon-sleep-inhibitor")
	return func() { _ = file.Close() }, nil
}
