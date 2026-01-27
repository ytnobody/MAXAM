// Package comms handles inter-agent communication via files
package comms

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Message represents an inter-agent message
type Message struct {
	Timestamp time.Time
	From      string
	To        string
	Subject   string
	Body      string
	Action    string
	IssueNum  int
	PRNum     int
}

// Format returns the message in markdown format
func (m *Message) Format() string {
	ts := m.Timestamp.Format("2006-01-02 15:04")

	content := fmt.Sprintf(`## [%s] From: %s To: %s

### Subject
%s

### Body
%s

### Action Required
%s
`, ts, m.From, m.To, m.Subject, m.Body, m.Action)

	if m.IssueNum > 0 || m.PRNum > 0 {
		content += "\n### Related\n"
		if m.IssueNum > 0 {
			content += fmt.Sprintf("- Issue: #%d\n", m.IssueNum)
		}
		if m.PRNum > 0 {
			content += fmt.Sprintf("- PR: #%d\n", m.PRNum)
		}
	}

	content += "\n---\n"
	return content
}

// Channel represents a communication channel between agents
type Channel struct {
	dir  string
	from string
	to   string
}

// NewChannel creates a new communication channel
func NewChannel(dir, from, to string) *Channel {
	return &Channel{
		dir:  dir,
		from: from,
		to:   to,
	}
}

// filename returns the channel's file path
func (c *Channel) filename() string {
	name := fmt.Sprintf("%s_to_%s.md", c.from, c.to)
	return filepath.Join(c.dir, name)
}

// Send appends a message to the channel
func (c *Channel) Send(msg *Message) error {
	msg.From = c.from
	msg.To = c.to
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	f, err := os.OpenFile(c.filename(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open channel file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(msg.Format())
	return err
}

// Clear removes all messages from the channel
func (c *Channel) Clear() error {
	return os.Remove(c.filename())
}

// Router manages all communication channels
type Router struct {
	dir      string
	channels map[string]*Channel
}

// NewRouter creates a new message router
func NewRouter(dir string) *Router {
	// Ensure directory exists
	os.MkdirAll(dir, 0755)

	return &Router{
		dir:      dir,
		channels: make(map[string]*Channel),
	}
}

// GetChannel returns a channel between two agents
func (r *Router) GetChannel(from, to string) *Channel {
	key := fmt.Sprintf("%s->%s", from, to)
	if ch, ok := r.channels[key]; ok {
		return ch
	}

	ch := NewChannel(r.dir, from, to)
	r.channels[key] = ch
	return ch
}

// Send sends a message through the router
func (r *Router) Send(from, to string, msg *Message) error {
	ch := r.GetChannel(from, to)
	return ch.Send(msg)
}
