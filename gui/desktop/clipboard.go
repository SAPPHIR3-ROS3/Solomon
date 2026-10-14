package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

// ClipboardImage is invoked only by a paste action in the native composer.
func (b ServerBridge) ClipboardImage() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	data, err := clipboard.ReadImage(ctx)
	if errors.Is(err, clipboard.ErrNoImage) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	mime, ok := images.MIMEForBinary(data)
	if !ok {
		return "", fmt.Errorf("unsupported clipboard image")
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
