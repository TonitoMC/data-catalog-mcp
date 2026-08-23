package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// maxMessageSize bounds a single line's length, sized generously for
// large tool results (e.g. a full column dump from profile_column).
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

// WriteRequest encodes and sends req, terminated by a newline. Used by the
// client side of a connection.
func (c *Conn) WriteRequest(req *Request) error {
	if err := c.enc.Encode(req); err != nil {
		return fmt.Errorf("jsonrpc: write: %w", err)
	}
	return nil
}

// ReadResponse blocks until the next line arrives and parses it as a
// Response. Returns io.EOF when the input stream is closed. Used by the
// client side of a connection.
func (c *Conn) ReadResponse() (*Response, error) {
	line, err := c.ReadLine()
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("jsonrpc: %w: %w", ErrParse, err)
	}
	return &resp, nil
}

// ReadLine blocks until the next line arrives and returns it unparsed.
// Returns io.EOF when the input stream is closed. Used by callers that
// need to inspect a message before deciding whether it's a Request or a
// Response (e.g. a client watching for unsolicited server notifications).
func (c *Conn) ReadLine() ([]byte, error) {
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("jsonrpc: read: %w", err)
		}
		return nil, io.EOF
	}
	line := c.scanner.Bytes()
	buf := make([]byte, len(line))
	copy(buf, line)
	return buf, nil
}
