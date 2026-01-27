package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// WebhookHandler handles GitHub webhook events
type WebhookHandler struct {
	secret   string
	handlers map[string]EventHandler
}

// EventHandler processes a specific event type
type EventHandler func(payload json.RawMessage) error

// Event types
const (
	EventIssues             = "issues"
	EventIssueComment       = "issue_comment"
	EventPullRequest        = "pull_request"
	EventPullRequestReview  = "pull_request_review"
	EventPush               = "push"
)

// IssuesEvent represents an issues event payload
type IssuesEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	} `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// PullRequestEvent represents a pull_request event payload
type PullRequestEvent struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		Head   struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// PullRequestReviewEvent represents a pull_request_review event
type PullRequestReviewEvent struct {
	Action string `json:"action"`
	Review struct {
		State string `json:"state"`
		Body  string `json:"body"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"review"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(secret string) *WebhookHandler {
	return &WebhookHandler{
		secret:   secret,
		handlers: make(map[string]EventHandler),
	}
}

// On registers a handler for an event type
func (h *WebhookHandler) On(event string, handler EventHandler) {
	h.handlers[event] = handler
}

// ServeHTTP implements http.Handler
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature if secret is set
	if h.secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !h.verifySignature(body, sig) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Get event type
	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		http.Error(w, "Missing event type", http.StatusBadRequest)
		return
	}

	// Find handler
	handler, ok := h.handlers[event]
	if !ok {
		// No handler for this event, just acknowledge
		w.WriteHeader(http.StatusOK)
		return
	}

	// Process event
	if err := handler(body); err != nil {
		fmt.Printf("Webhook error (%s): %v\n", event, err)
		http.Error(w, "Handler error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sig, err := hex.DecodeString(signature[7:])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}

// ParseIssuesEvent parses an issues event
func ParseIssuesEvent(payload json.RawMessage) (*IssuesEvent, error) {
	var event IssuesEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// ParsePullRequestEvent parses a pull_request event
func ParsePullRequestEvent(payload json.RawMessage) (*PullRequestEvent, error) {
	var event PullRequestEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// ParsePullRequestReviewEvent parses a pull_request_review event
func ParsePullRequestReviewEvent(payload json.RawMessage) (*PullRequestReviewEvent, error) {
	var event PullRequestReviewEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
