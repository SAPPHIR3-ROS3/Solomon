package chat

import (
	"bufio"
	"bytes"
	"io"
)

func RewriteCodexSSEStream(r io.Reader, w io.Writer, model string) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	transformer := newSSETransformer(model)
	var dataLines [][]byte
	doneSeen := false
	flushEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		raw := bytes.Join(dataLines, []byte("\n"))
		dataLines = dataLines[:0]
		out, done, err := transformer.transform(raw)
		if err != nil {
			return err
		}
		if done {
			doneSeen = true
			_, err := w.Write([]byte("data: [DONE]\n\n"))
			return err
		}
		if len(out) > 0 {
			for _, line := range bytes.Split(out, []byte("\n")) {
				if len(line) == 0 {
					continue
				}
				if _, err := w.Write([]byte("data: ")); err != nil {
					return err
				}
				if _, err := w.Write(line); err != nil {
					return err
				}
				if _, err := w.Write([]byte("\n\n")); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			if err := flushEvent(); err != nil {
				return err
			}
			continue
		}
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			payload := bytes.TrimPrefix(line, []byte("data:"))
			if len(payload) > 0 && payload[0] == ' ' {
				payload = payload[1:]
			}
			if bytes.Equal(bytes.TrimSpace(payload), []byte("[DONE]")) {
				if err := flushEvent(); err != nil {
					return err
				}
				continue
			}
			cp := make([]byte, len(payload))
			copy(cp, payload)
			dataLines = append(dataLines, cp)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := flushEvent(); err != nil {
		return err
	}
	if !doneSeen {
		_, err := w.Write([]byte("data: [DONE]\n\n"))
		return err
	}
	return nil
}
