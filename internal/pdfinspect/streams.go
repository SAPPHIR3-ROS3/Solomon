package pdfinspect

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
)

const maxObjectStreamBytes = 16 << 20

var objectStreamHeader = regexp.MustCompile(`/Type[\x00\t\n\f\r ]*/ObjStm\b`)
var objectStreamStart = regexp.MustCompile(`>>[\x00\t\n\f\r ]*stream(?:\r\n|\n|\r)`)
var objectStreamFilter = regexp.MustCompile(`/Filter[\x00\t\n\f\r ]*(?:/FlateDecode\b|\[[\x00\t\n\f\r ]*/FlateDecode[\x00\t\n\f\r ]*\])`)

func inspectObjectStreams(data []byte, report *Report) {
	remaining := maxObjectStreamBytes
	unavailable := 0
	for index, match := range objectStreamHeader.FindAllIndex(data, 1025) {
		if index == 1024 {
			unavailable++
			break
		}
		end := objectStreamStart.FindIndex(data[match[1]:min(len(data), match[1]+4096)])
		if end == nil {
			unavailable++
			continue
		}
		start := bytes.LastIndex(data[max(0, match[0]-4096):match[0]], []byte("<<"))
		if start < 0 || remaining <= 0 {
			unavailable++
			continue
		}
		header := data[max(0, match[0]-4096)+start : match[1]+end[0]]
		if !objectStreamFilter.Match(header) {
			unavailable++
			continue
		}
		reader, err := zlib.NewReader(bytes.NewReader(data[match[1]+end[1]:]))
		if err != nil {
			unavailable++
			continue
		}
		decoded, err := io.ReadAll(io.LimitReader(reader, int64(remaining)+1))
		_ = reader.Close()
		remaining -= len(decoded)
		if err != nil || remaining < 0 {
			unavailable++
			continue
		}
		for name, count := range scanNames(decoded) {
			report.RawNameHints[name] += count
		}
	}
	if unavailable > 0 {
		report.Issues = append(report.Issues, "Some PDF object streams could not be inspected for structural hints")
	}
}
