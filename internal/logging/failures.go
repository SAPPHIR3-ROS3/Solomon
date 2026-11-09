package logging

import (
	"fmt"
	"os"
	"time"
)

var lastStorageFailure time.Time

func storageFailure(operation string, err error) {
	if err == nil || time.Since(lastStorageFailure) < time.Minute {
		return
	}
	lastStorageFailure = time.Now()
	fmt.Fprintf(os.Stderr, "Solomon logging storage failure: operation=%s error=%s\n", operation, RedactReport(err.Error()))
}
