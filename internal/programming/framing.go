package programming

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const MaxMessageSize = 16 << 20

func ReadMessage(reader *bufio.Reader) ([]byte, error) {
	length := -1
	headerBytes := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerBytes += len(line)
		if headerBytes > 8192 {
			return nil, fmt.Errorf("language server headers exceed limit")
		}
		if line == "\r\n" || line == "\n" {
			break
		}
		name, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && strings.EqualFold(name, "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
		}
	}
	if length < 0 || length > MaxMessageSize {
		return nil, fmt.Errorf("invalid language server content length")
	}
	data := make([]byte, length)
	_, err := io.ReadFull(reader, data)
	return data, err
}

func WriteMessage(writer io.Writer, data []byte) error {
	if len(data) > MaxMessageSize {
		return fmt.Errorf("language server message exceeds limit")
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}
