package updater

import "io"

type downloadReader struct {
	reader            io.Reader
	downloaded, total int64
	report            func(int64, int64)
}

func (r *downloadReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.downloaded += int64(n)
	r.report(r.downloaded, r.total)
	return n, err
}

func trackDownload(reader io.Reader, total int64, output io.Writer) io.Reader {
	reporter, ok := output.(interface{ DownloadProgress(int64, int64) })
	if !ok {
		return reader
	}
	reporter.DownloadProgress(0, total)
	return &downloadReader{reader: reader, total: total, report: reporter.DownloadProgress}
}
