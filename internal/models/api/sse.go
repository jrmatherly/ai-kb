package api

import (
	"bufio"
	"io"
	"strings"
)

// SSEEvent is one Server-Sent Events event
type SSEEvent struct {
	Data []byte
	Done bool
}

// SSEReader reads an SSE stream
type SSEReader struct {
	scanner *bufio.Scanner
}

// NewSSEReader creates an SSE reader
func NewSSEReader(reader io.Reader) *SSEReader {
	scanner := bufio.NewScanner(reader)
	// Use a larger buffer for long lines (chain-of-thought content can be long)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024)
	return &SSEReader{scanner: scanner}
}

// ReadEvent reads the next SSE event
func (r *SSEReader) ReadEvent() (*SSEEvent, error) {
	for r.scanner.Scan() {
		line := r.scanner.Text()

		// Empty line; skip it
		if line == "" {
			continue
		}

		// Parse the data line. The SSE spec only requires the "data:" prefix; the single
		// space after the colon is optional, so both forms are accepted.
		if !strings.HasPrefix(line, "data:") {
			// Skip other lines (event:, id: and so on)
			continue
		}
		payload := strings.TrimPrefix(line[len("data:"):], " ")

		// Check for the end marker. Some gateways write "data:[DONE]" or leave trailing whitespace,
		// so compare the trimmed content, to avoid parsing the sentinel as JSON.
		if strings.TrimSpace(payload) == "[DONE]" {
			return &SSEEvent{Done: true}, nil
		}

		return &SSEEvent{Data: []byte(payload)}, nil

		// Skip other lines (event:, id: and so on)
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}
