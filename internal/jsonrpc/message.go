// Package jsonrpc implements bare JSON-RPC 2.0 message framing over a
// line-delimited stream (as used by MCP's stdio transport). It only knows
// about the wire format — method dispatch belongs to the caller.
package jsonrpc

import (
	"encoding/json"
	"errors"
)

const Version = "2.0"

// ErrParse wraps malformed-JSON read errors so callers can map them to
// CodeParseError without string-matching.
var ErrParse = errors.New("malformed JSON-RPC message")

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Request is an incoming call. A nil ID means it's a notification, the
// caller does not expect (and must not receive) a Response.
type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// IsNotification reports whether this request expects no reply.
func (r *Request) IsNotification() bool {
	return r.ID == nil
}

// Response is a reply to a Request that had a non-nil ID. Exactly one of
// Result / Error should be set.
type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// NewError builds an *Error for one of the standard codes above. data is
// optional structured context for the caller to parse programmatically;
// pass nil when the message alone is enough.
func NewError(code int, message string, data any) *Error {
	return &Error{Code: code, Message: message, Data: data}
}

// NewResult builds a successful Response for the given request ID.
func NewResult(id *json.RawMessage, result any) *Response {
	return &Response{JSONRPC: Version, ID: id, Result: result}
}

// NewErrorResponse builds a failed Response for the given request ID.
func NewErrorResponse(id *json.RawMessage, err *Error) *Response {
	return &Response{JSONRPC: Version, ID: id, Error: err}
}
