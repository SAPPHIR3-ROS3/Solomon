package memoryprofile

import (
	"encoding/json"
	"fmt"
)

const ReportByteLimit = 16 * 1024

func (r Report) BoundedJSON(limit int) (string, error) {
	for {
		data, err := json.Marshal(r)
		if err != nil {
			return "", err
		}
		encoded, err := json.Marshal(string(data))
		if err != nil {
			return "", err
		}
		if len(encoded) <= limit {
			return string(data), nil
		}
		if len(r.Samples) == 0 {
			return "", fmt.Errorf("memory report budget is too small")
		}
		r.Samples = r.Samples[1:]
	}
}
