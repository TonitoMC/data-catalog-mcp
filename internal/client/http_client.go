package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tonitomc/data-catalog-mcp/internal/jsonrpc"
)

// httpClient talks MCP to a server reachable over plain HTTP (see
// internal/mcp.Server.HTTPHandler on the other end) — one POST per
// JSON-RPC message, no persistent connection, no subprocess to manage.
type httpClient struct {
	baseURL    string
	httpClient *http.Client
	nextID     int64
}

// ConnectHTTP builds a Client that reaches an MCP server already running
// at baseURL (e.g. a Cloud Run URL), instead of spawning one locally.
// Unlike Connect/ConnectWithEnv this makes no network call itself —
// there's no subprocess to start, so "connecting" is just constructing
// the client; the first real request is Initialize.
func ConnectHTTP(baseURL string) (Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("client: ConnectHTTP: empty URL")
	}
	return &httpClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Close is a no-op: HTTP is stateless between calls, there's nothing this
// side holds open.
func (c *httpClient) Close() error { return nil }

func (c *httpClient) Initialize() error {
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

func (c *httpClient) ListTools() ([]ToolInfo, error) {
	var result listToolsResult
	if err := c.call("tools/list", nil, &result); err != nil {
		return nil, fmt.Errorf("client: list tools: %w", err)
	}
	return result.Tools, nil
}

func (c *httpClient) CallTool(name string, args map[string]any) (string, error) {
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

// call POSTs one JSON-RPC request and decodes its response's result into
// out (if non-nil). Fails if the response carries a JSON-RPC error.
func (c *httpClient) call(method string, params any, out any) error {
	c.nextID++
	id := json.RawMessage(fmt.Sprintf("%d", c.nextID))

	req, err := buildRequest(&id, method, params)
	if err != nil {
		return err
	}

	resp, err := c.do(req)
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

// notify POSTs a request with no ID. The server replies 204 with no
// body — nothing to decode.
func (c *httpClient) notify(method string, params any) error {
	req, err := buildRequest(nil, method, params)
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// do POSTs req as JSON and, unless it was a notification, decodes the
// JSON-RPC response body. A notification's response has no body (the
// server replies 204), so it returns a zero Response rather than trying
// to decode nothing.
func (c *httpClient) do(req *jsonrpc.Request) (*jsonrpc.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("client: encode request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("client: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if req.IsNotification() {
		return &jsonrpc.Response{}, nil
	}

	if httpResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("client: unexpected status %d: %s", httpResp.StatusCode, b)
	}

	var resp jsonrpc.Response
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("jsonrpc: %w: %w", jsonrpc.ErrParse, err)
	}
	return &resp, nil
}

func buildRequest(id *json.RawMessage, method string, params any) (*jsonrpc.Request, error) {
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
