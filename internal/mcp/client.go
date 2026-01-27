package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Client handles MCP communication over stdio
type Client struct {
	stdin  io.Writer
	stdout io.Reader
	reader *bufio.Reader

	mu        sync.Mutex
	requestID int
	pending   map[int]chan *Response
}

// NewClient creates a new MCP client
func NewClient(stdin io.Writer, stdout io.Reader) *Client {
	return &Client{
		stdin:   stdin,
		stdout:  stdout,
		reader:  bufio.NewReader(stdout),
		pending: make(map[int]chan *Response),
	}
}

// Start begins reading responses from stdout
func (c *Client) Start() {
	go c.readLoop()
}

func (c *Client) readLoop() {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Printf("read error: %v\n", err)
			}
			return
		}

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			// Might be a notification, skip for now
			continue
		}

		c.mu.Lock()
		if ch, ok := c.pending[resp.ID]; ok {
			ch <- &resp
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()
	}
}

// Call sends a JSON-RPC request and waits for response
func (c *Client) Call(method string, params interface{}) (*Response, error) {
	c.mu.Lock()
	c.requestID++
	id := c.requestID
	ch := make(chan *Response, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	resp := <-ch
	if resp.Error != nil {
		return resp, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp, nil
}

// Initialize sends the initialize request
func (c *Client) Initialize() (*InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Roots: &RootsCapability{ListChanged: true},
		},
		ClientInfo: ClientInfo{
			Name:    "maxam-orchestrator",
			Version: "0.1.0",
		},
	}

	resp, err := c.Call("initialize", params)
	if err != nil {
		return nil, err
	}

	// Send initialized notification
	notif := Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notif)
	data = append(data, '\n')
	c.stdin.Write(data)

	// Parse result
	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result InitializeResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CallTool invokes a tool on the MCP server
func (c *Client) CallTool(name string, args map[string]interface{}) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.Call("tools/call", params)
	if err != nil {
		return nil, err
	}

	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result CallToolResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetPrompt retrieves a prompt from the MCP server
func (c *Client) GetPrompt(name string, args map[string]string) (*GetPromptResult, error) {
	params := GetPromptParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.Call("prompts/get", params)
	if err != nil {
		return nil, err
	}

	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var result GetPromptResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Close sends shutdown request
func (c *Client) Close() error {
	_, err := c.Call("shutdown", nil)
	return err
}
