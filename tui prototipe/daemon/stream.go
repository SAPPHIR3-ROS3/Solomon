package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

type Event struct {
	Type string
	ID   string
	Data json.RawMessage
}

func (c *Client) Send(ctx context.Context, project, chat, content string, handle func(Event) error) error {
	if handle == nil {
		return errors.New("event handler is required")
	}
	path, err := chatPath(project, chat)
	if err != nil {
		return err
	}
	resp, err := c.request(ctx, http.MethodPost, path+"/messages", map[string]string{"content": content, "surface": "tui"}, "text/event-stream")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || media != "text/event-stream" {
		return errors.New("daemon response is not an event stream")
	}
	return consume(resp.Body, handle)
}

func consume(body io.Reader, handle func(Event) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), 32<<20)
	var kind, id string
	var data []string
	size := 0
	dispatch := func() error {
		if len(data) == 0 {
			kind = ""
			return nil
		}
		payload := []byte(strings.Join(data, "\n"))
		if !json.Valid(payload) {
			return errors.New("invalid SSE JSON data")
		}
		err := handle(Event{Type: kind, ID: id, Data: json.RawMessage(payload)})
		kind = ""
		data = nil
		size = 0
		return err
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if found {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "data":
			size += len(value) + 1
			if size > 32<<20 {
				return errors.New("SSE event too large")
			}
			data = append(data, value)
		case "event":
			kind = value
		case "id":
			if !strings.ContainsRune(value, 0) {
				id = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read SSE: %w", err)
	}
	return nil
}
