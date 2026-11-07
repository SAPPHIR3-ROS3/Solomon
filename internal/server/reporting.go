package server

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/memoryprofile"
)

func buildReportPayload(note, logs string, memory memoryprofile.Report) ([]byte, error) {
	metrics, err := memory.BoundedJSON(memoryprofile.ReportByteLimit)
	if err != nil {
		return nil, err
	}
	payload := map[string]string{
		"sentAt":  logging.ReportStamp(),
		"version": reportVersion(),
		"os":      runtime.GOOS + "/" + runtime.GOARCH,
		"note":    logging.RedactReport(note),
		"logs":    logs,
		"memory":  metrics,
	}
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) <= logging.ReportByteLimit {
		return encoded, err
	}
	runes := []rune(logs)
	low, high := 0, len(runes)
	payload["logs"] = ""
	empty, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(empty) > logging.ReportByteLimit {
		return nil, fmt.Errorf("report metadata exceeds budget")
	}
	for low < high {
		middle := low + (high-low)/2
		payload["logs"] = string(runes[middle:])
		candidate, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		if len(candidate) > logging.ReportByteLimit {
			low = middle + 1
		} else {
			high = middle
		}
	}
	payload["logs"] = string(runes[low:])
	return json.Marshal(payload)
}

func BuildReportPayloadForTest(note, logs string, memory memoryprofile.Report) ([]byte, error) {
	return buildReportPayload(note, logs, memory)
}
