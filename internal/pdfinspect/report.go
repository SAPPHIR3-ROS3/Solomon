package pdfinspect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const maxFileBytes = 128 << 20

type Page struct {
	Number       int                   `json:"number"`
	WidthPoints  float64               `json:"width_points"`
	HeightPoints float64               `json:"height_points"`
	Rotation     int                   `json:"rotation"`
	Boxes        map[string][4]float64 `json:"boxes,omitempty"`
	Images       *int                  `json:"image_entries,omitempty"`
}

type Report struct {
	FontEntriesInRange *int              `json:"font_entries_in_range,omitempty"`
	Status             string            `json:"status"`
	Bytes              int               `json:"bytes"`
	SHA256             string            `json:"sha256"`
	PageCount          int               `json:"page_count,omitempty"`
	Properties         map[string]string `json:"properties"`
	Indicators         map[string]int    `json:"indicators"`
	RawNameHints       map[string]int    `json:"raw_name_hints"`
	Pages              []Page            `json:"pages,omitempty"`
	Issues             []string          `json:"issues,omitempty"`
}

func Analyze(ctx context.Context, path string, pageLimit int) (Report, error) {
	data, err := loadPDF(path)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Status: "no_indicators_detected", Bytes: len(data),
		SHA256:       fmt.Sprintf("%x", sha256.Sum256(data)),
		Properties:   make(map[string]string),
		Indicators:   make(map[string]int),
		RawNameHints: scanNames(data),
	}
	if updates := bytes.Count(data, []byte("\nstartxref")); updates > 1 {
		report.RawNameHints["multiple startxref markers (possible incremental updates)"] = updates
	}
	header := bytes.Index(data[:min(len(data), 1024)], []byte("%PDF-"))
	if header > 0 {
		report.Indicators["bytes_before_header"] = header
	}
	eof := bytes.LastIndex(data, []byte("%%EOF"))
	if eof < 0 {
		report.Indicators["missing_eof_marker"] = 1
	} else if trailing := len(bytes.Trim(data[eof+5:], "\x00\t\n\f\r ")); trailing > 0 {
		report.Indicators["bytes_after_eof"] = trailing
	}

	inspectObjectStreams(data, &report)
	inspect(ctx, data, pageLimit, &report)
	switch {
	case len(report.Issues) > 0:
		report.Status = "incomplete"
	case len(report.Indicators) > 0 || len(report.RawNameHints) > 0:
		report.Status = "review"
	}
	return report, ctx.Err()
}

func loadPDF(path string) ([]byte, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > maxFileBytes {
		return nil, fmt.Errorf("expected a regular PDF file of at most 128 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileBytes {
		return nil, fmt.Errorf("PDF exceeds 128 MiB")
	}
	if !bytes.Contains(data[:min(len(data), 1024)], []byte("%PDF-")) {
		return nil, fmt.Errorf("PDF header not found")
	}
	return data, nil
}

var pdfName = regexp.MustCompile(`/[^\x00\t\n\f\r ()<>\[\]{}/%]+`)

var watchedNames = map[string]string{
	"JavaScript": "JavaScript", "JS": "JavaScript payload",
	"OpenAction": "open action or destination", "AA": "additional actions",
	"Launch": "launch action", "GoToR": "remote destination", "GoToE": "embedded destination",
	"SubmitForm": "form submission", "ImportData": "form data import",
	"Collection": "PDF portfolio", "Filespec": "file specification", "AF": "associated files",
	"URI": "URI action or base", "EmbeddedFiles": "embedded file name tree",
	"EmbeddedFile": "embedded file stream", "EF": "embedded file reference",
	"RichMedia": "rich media", "Rendition": "multimedia action", "Movie": "movie",
	"Sound": "sound", "3D": "3D content", "XFA": "XFA form",
	"JBIG2Decode": "JBIG2 image filter", "JPXDecode": "JPEG 2000 image filter",
}

func scanNames(data []byte) map[string]int {
	hints := make(map[string]int)
	for len(data) > 0 {
		match := pdfName.FindIndex(data)
		if match == nil {
			break
		}
		if match[1]-match[0] <= 64 {
			name := decodeName(string(data[match[0]+1 : match[1]]))
			if label := watchedNames[name]; label != "" {
				hints[label]++
			}
		}
		data = data[match[1]:]
	}
	return hints
}

func decodeName(name string) string {
	var decoded strings.Builder
	for i := 0; i < len(name); i++ {
		if name[i] == '#' && i+2 < len(name) {
			value, err := strconv.ParseUint(name[i+1:i+3], 16, 8)
			if err == nil {
				decoded.WriteByte(byte(value))
				i += 2
				continue
			}
		}
		decoded.WriteByte(name[i])
	}
	return decoded.String()
}
