package app

import (
	"os"

	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

func webviewGPUPolicy() linux.WebviewGpuPolicy {
	switch os.Getenv("SOLOMON_WEBVIEW_GPU") {
	case "never":
		return linux.WebviewGpuPolicyNever
	case "always":
		return linux.WebviewGpuPolicyAlways
	default:
		return linux.WebviewGpuPolicyOnDemand
	}
}
