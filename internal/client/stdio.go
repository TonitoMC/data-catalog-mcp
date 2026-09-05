package client

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/tonitomc/data-catalog-mcp/internal/jsonrpc"
)

// callTimeout bounds how long call() waits for a subprocess's response.
// Without it, a server that never replies (crashed, or a tool like
// git_commit blocking on an editor/GPG prompt it can't get input for)
// hangs the caller forever — there's no way to cancel a blocking
// bufio.Scanner read, so a stuck response leaks the read goroutine, but
// that's a fair trade for never freezing the whole chat loop.
const callTimeout = 20 * time.Second

// stdioClient talks MCP to a single server subprocess over stdio.
type stdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	conn   *jsonrpc.Conn
	nextID int64
}

// Connect starts name(args...) as a subprocess and wires up a JSON-RPC
// connection to its stdin/stdout. The subprocess's stderr is passed
// through so server logs are still visible.
func Connect(name string, args ...string) (Client, error) {
	return ConnectWithEnv(name, args, nil)
}

// ConnectWithEnv is like Connect but additionally sets the given
// environment variables on the subprocess (on top of the current
// process's environment). Used for servers like the data-catalog one that
// take configuration via env vars (e.g. CATALOG_PATH).
func ConnectWithEnv(name string, args []string, env map[string]string) (Client, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("client: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("client: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("client: start %q: %w", name, err)
	}

	return &stdioClient{
		cmd:   cmd,
		stdin: stdin,
		conn:  jsonrpc.NewConn(stdout, stdin),
	}, nil
}

// Close closes the subprocess's stdin (signaling EOF) and waits for it to
// exit.
func (c *stdioClient) Close() error {
	_ = c.stdin.Close()
	return c.cmd.Wait()
}

func (c *stdioClient) Initialize() error {
	params := initializeParams{
		ProtocolVersion: protocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo:      clientInfo{Name: "data-catalog-mcp-client", Version: "0.0.1"},
	}
	if err := c.call("initialize", params, nil); err != nil {
		return fmt.Errorf("client: initialize: %w", err)
	}
	return c.notify("notifications/initialized", nil)
}

func (c *stdioClient) ListTools() ([]ToolInfo, error) {
	var result listToolsResult
	if err := c.call("tools/list", nil, &result); err != nil {
		return nil, fmt.Errorf("client: list tools: %w", err)
	}
	return result.Tools, nil
}

func (c *stdioClient) CallTool(name string, args map[string]any) (string, error) {
	var result callToolResult
	params := map[string]any{"name": name, "arguments": args}
	if err := c.call("tools/call", params, &result); err != nil {
		return "", fmt.Errorf("client: call tool %q: %w", name, err)
	}

	var text string
	for _, block := range result.Content {
		text += block.Text
	}
	if result.IsError {
		return text, fmt.Errorf("client: tool %q returned an error: %s", name, text)
	}
	return text, nil
}

// call sends a request, waits for its response, and decodes the result
// into out (if non-nil). It fails if the response carries a JSON-RPC
// error.
func (c *stdioClient) call(method string, params any, out any) error {
	c.nextID++
	id := json.RawMessage(fmt.Sprintf("%d", c.nextID))

	req, err := c.buildRequest(&id, method, params)
	if err != nil {
		return err
	}
	if err := c.conn.WriteRequest(req); err != nil {
		return err
	}

	type result struct {
		resp *jsonrpc.Response
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := c.readResponse()
		done <- result{resp, err}
	}()

	var res result
	select {
	case res = <-done:
	case <-time.After(callTimeout):
		return fmt.Errorf("client: %s: no response after %s (server unresponsive)", method, callTimeout)
	}
	resp, err := res.resp, res.err
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("%s (code %d)", resp.Error.Message, resp.Error.Code)
	}
	if out == nil {
		return nil
	}

	raw, err := json.Marshal(resp.Result)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// readResponse reads lines until it finds one that's a reply (no "method"
// field) rather than a server-initiated notification or request. Servers
// commonly emit unsolicited notifications (e.g. logging messages) on
// stdout; those are skipped. A server-initiated request (has both
// "method" and "id") gets a method-not-found reply so the server doesn't
// hang waiting on it — this barebones client doesn't implement any
// server-to-client methods (sampling, roots, elicitation, ...).
//
// This assumes at most one of our requests is ever in flight at a time,
// which holds for this client since call() is synchronous.
func (c *stdioClient) readResponse() (*jsonrpc.Response, error) {
	for {
		line, err := c.conn.ReadLine()
		if err != nil {
			return nil, err
		}

		var probe struct {
			Method *string          `json:"method"`
			ID     *json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			return nil, fmt.Errorf("jsonrpc: %w: %w", jsonrpc.ErrParse, err)
		}
		if probe.Method != nil {
			if probe.ID != nil {
				_ = c.conn.WriteResponse(jsonrpc.NewErrorResponse(probe.ID,
					jsonrpc.NewError(jsonrpc.CodeMethodNotFound, "client does not support "+*probe.Method, nil)))
			}
			continue
		}

		var resp jsonrpc.Response
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, fmt.Errorf("jsonrpc: %w: %w", jsonrpc.ErrParse, err)
		}
		return &resp, nil
	}
}

// notify sends a request with no ID, which expects no response.
func (c *stdioClient) notify(method string, params any) error {
	req, err := c.buildRequest(nil, method, params)
	if err != nil {
		return err
	}
	return c.conn.WriteRequest(req)
}

func (c *stdioClient) buildRequest(id *json.RawMessage, method string, params any) (*jsonrpc.Request, error) {
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("client: encode params for %s: %w", method, err)
		}
		rawParams = b
	}
	return &jsonrpc.Request{
		JSONRPC: jsonrpc.Version,
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}, nil
}
