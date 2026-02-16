package handover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ytnobody/MAXAM/internal/retry"
)

// Client handles handover API operations.
type Client struct {
	baseURL    string
	httpClient *http.Client
	retryCfg   retry.Config
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithRetryConfig sets the retry configuration.
func WithRetryConfig(cfg retry.Config) ClientOption {
	return func(c *Client) {
		c.retryCfg = cfg
	}
}

// NewClient creates a new handover client.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryCfg: retry.DefaultConfig(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ListPending returns tasks waiting for handover.
func (c *Client) ListPending(ctx context.Context) ([]PendingTask, error) {
	resp, err := retry.DoWithResult(ctx, c.retryCfg, func() (PendingTasksResponse, error) {
		return c.doGetPending(ctx)
	})
	if err != nil {
		return nil, err
	}
	return resp.Tasks, nil
}

func (c *Client) doGetPending(ctx context.Context) (PendingTasksResponse, error) {
	var result PendingTasksResponse

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tasks/pending", nil)
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return result, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// Assign reassigns a task to another agent.
func (c *Client) Assign(ctx context.Context, taskID int, targetAgent string) (*AssignResult, error) {
	// Client-side validation
	if taskID <= 0 {
		return nil, ErrInvalidTaskID
	}
	if targetAgent == "" {
		return nil, ErrInvalidTargetAgent
	}

	result, err := retry.DoWithResult(ctx, c.retryCfg, func() (*AssignResult, error) {
		return c.doAssign(ctx, taskID, targetAgent)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) doAssign(ctx context.Context, taskID int, targetAgent string) (*AssignResult, error) {
	reqBody := AssignRequest{TargetAgent: targetAgent}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/tasks/%d/assign", c.baseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return nil, err
	}

	var result AssignResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GetHandoverDetail returns handover details with history.
func (c *Client) GetHandoverDetail(ctx context.Context, taskID int) (*HandoverDetail, error) {
	if taskID <= 0 {
		return nil, ErrInvalidTaskID
	}

	result, err := retry.DoWithResult(ctx, c.retryCfg, func() (*HandoverDetail, error) {
		return c.doGetDetail(ctx, taskID)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) doGetDetail(ctx context.Context, taskID int) (*HandoverDetail, error) {
	url := fmt.Sprintf("%s/api/tasks/%d/handover", c.baseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if err := c.checkStatus(resp); err != nil {
		return nil, err
	}

	var result HandoverDetail
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// checkStatus handles HTTP status codes and returns appropriate errors.
func (c *Client) checkStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limited: 429")
	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}
