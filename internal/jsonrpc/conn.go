package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// maxMessageSize bounds a single line's length. Large enough for tool
// results like chart_column's base64-encoded image content.
const maxMessageSize = 16 * 1024 * 1024

// Conn reads newline-delimited JSON-RPC requests from r and writes
// newline-delimited responses to w. Not safe for concurrent use by
// multiple goroutines on the same side (read loop is single-threaded by
// design; writes should be serialized by the caller if calls run
// concurrently).
type Conn struct {
	scanner *bufio.Scanner
	enc     *json.Encoder
}

// NewConn wraps a reader/writer pair (typically os.Stdin / os.Stdout) as a
// JSON-RPC connection.
func NewConn(r io.Reader, w io.Writer) *Conn {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxMessageSize)
	return &Conn{
		scanner: scanner,
		enc:     json.NewEncoder(w),
	}
}

// ReadRequest blocks until the next line arrives, parses it as a Request,
// and returns it. Returns io.EOF when the input stream is closed.
func (c *Conn) ReadRequest() (*Request, error) {
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("jsonrpc: read: %w", err)
		}
		return nil, io.EOF
	}

	line := c.scanner.Bytes()
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, fmt.Errorf("jsonrpc: %w: %w", ErrParse, err)
	}
	return &req, nil
}

// WriteResponse encodes and sends resp, terminated by a newline (which
// json.Encoder already appends).
func (c *Conn) WriteResponse(resp *Response) error {
	if err := c.enc.Encode(resp); err != nil {
		return fmt.Errorf("jsonrpc: write: %w", err)
	}
	return nil
}
