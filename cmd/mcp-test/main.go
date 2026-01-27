package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	fmt.Println("Starting Claude Code MCP server...")

	cmd := exec.Command("claude", "mcp", "serve")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stdin pipe: %v\n", err)
		os.Exit(1)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stdout pipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(1)
	}

	// Give it a moment to start
	time.Sleep(500 * time.Millisecond)

	reader := bufio.NewReader(stdout)
	reqID := 0

	send := func(method string, params interface{}) (*Response, error) {
		reqID++
		req := Request{
			JSONRPC: "2.0",
			ID:      reqID,
			Method:  method,
			Params:  params,
		}

		data, _ := json.Marshal(req)
		fmt.Printf(">> %s\n", string(data))
		data = append(data, '\n')

		if _, err := stdin.Write(data); err != nil {
			return nil, err
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		fmt.Printf("<< %s\n", string(line))

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, err
		}

		return &resp, nil
	}

	// Initialize
	fmt.Println("\n=== Initialize ===")
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": true,
			},
		},
		"clientInfo": map[string]interface{}{
			"name":    "maxam-test",
			"version": "0.1.0",
		},
	}
	resp, err := send("initialize", initParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize: %v\n", err)
		os.Exit(1)
	}
	if resp.Error != nil {
		fmt.Fprintf(os.Stderr, "initialize error: %s\n", resp.Error.Message)
		os.Exit(1)
	}

	// Send initialized notification
	notif := Request{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notif)
	data = append(data, '\n')
	stdin.Write(data)
	fmt.Printf(">> %s (notification)\n", string(data[:len(data)-1]))

	// List tools
	fmt.Println("\n=== List Tools ===")
	resp, err = send("tools/list", nil)
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "tools/list: %v\n", err)
	}
	if resp != nil && resp.Result != nil {
		var tools struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		}
		json.Unmarshal(resp.Result, &tools)
		fmt.Printf("\nAvailable tools (%d):\n", len(tools.Tools))
		for _, t := range tools.Tools {
			fmt.Printf("  - %s: %s\n", t.Name, truncate(t.Description, 60))
		}
	}

	// List prompts
	fmt.Println("\n=== List Prompts ===")
	resp, err = send("prompts/list", nil)
	if err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "prompts/list: %v\n", err)
	}
	if resp != nil && resp.Result != nil {
		var prompts struct {
			Prompts []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"prompts"`
		}
		json.Unmarshal(resp.Result, &prompts)
		fmt.Printf("\nAvailable prompts (%d):\n", len(prompts.Prompts))
		for _, p := range prompts.Prompts {
			fmt.Printf("  - %s: %s\n", p.Name, truncate(p.Description, 60))
		}
	}

	// Cleanup
	stdin.Close()
	cmd.Process.Kill()
	fmt.Println("\nDone.")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
