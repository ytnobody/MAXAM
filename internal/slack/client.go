// Package slack provides Slack integration via Socket Mode
package slack

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Client handles Slack communication
type Client struct {
	api          *slack.Client
	socket       *socketmode.Client
	botUserID    string

	// Message handling
	messageQueue chan *IncomingMessage
	waitTimer    *time.Timer
	waitDuration time.Duration
	pendingMsgs  []*IncomingMessage
}

// IncomingMessage represents a message from Slack
type IncomingMessage struct {
	Channel      string
	User         string
	UserName     string
	Text         string
	Timestamp    string
	ThreadTS     string
	IsFromBot    bool   // true if message is from a bot (agent)
	SenderAgent  string // agent name if from bot (e.g., "Mei Chen")
}

// Config holds Slack configuration
type Config struct {
	BotToken    string
	AppToken    string
	WaitSeconds int // Wait time before processing (default 120)
}

// NewClient creates a new Slack client
func NewClient(cfg Config) (*Client, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN required")
	}
	if cfg.AppToken == "" {
		return nil, fmt.Errorf("SLACK_APP_TOKEN required")
	}

	api := slack.New(
		cfg.BotToken,
		slack.OptionAppLevelToken(cfg.AppToken),
	)

	// Get bot user ID
	authResp, err := api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("auth test: %w", err)
	}

	waitDuration := 120 * time.Second
	if cfg.WaitSeconds > 0 {
		waitDuration = time.Duration(cfg.WaitSeconds) * time.Second
	}

	socket := socketmode.New(
		api,
		socketmode.OptionDebug(false),
	)

	return &Client{
		api:          api,
		socket:       socket,
		botUserID:    authResp.UserID,
		messageQueue: make(chan *IncomingMessage, 100),
		waitDuration: waitDuration,
		pendingMsgs:  make([]*IncomingMessage, 0),
	}, nil
}

// Start begins listening for Slack events
func (c *Client) Start(ctx context.Context, handler func([]*IncomingMessage)) {
	go c.runSocketMode(ctx)
	go c.processMessages(ctx, handler)
}

func (c *Client) runSocketMode(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-c.socket.Events:
			c.handleEvent(evt)
		}
	}
}

func (c *Client) handleEvent(evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeConnecting:
		log.Println("[Slack] Connecting...")

	case socketmode.EventTypeConnected:
		log.Println("[Slack] Connected!")

	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}

		c.socket.Ack(*evt.Request)

		switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			c.handleMessageEvent(ev)
		}

	case socketmode.EventTypeSlashCommand:
		cmd, ok := evt.Data.(slack.SlashCommand)
		if !ok {
			return
		}
		c.socket.Ack(*evt.Request)
		c.handleSlashCommand(cmd)
	}
}

func (c *Client) handleMessageEvent(ev *slackevents.MessageEvent) {
	// Ignore message changes/deletions (but not bot_message subtype)
	if ev.SubType != "" && ev.SubType != "bot_message" {
		return
	}

	// For bot messages, only process if it contains an @mention
	// This allows agent-to-agent mentions while preventing loops
	isBotMessage := ev.User == c.botUserID || ev.SubType == "bot_message"
	if isBotMessage {
		if !strings.Contains(ev.Text, "@") {
			return
		}
	}

	// Get user info
	var userName string
	if isBotMessage {
		// For bot messages, use the Username field (agent name)
		userName = ev.Username
		if userName == "" {
			userName = "Bot"
		}
	} else {
		user, err := c.api.GetUserInfo(ev.User)
		userName = ev.User
		if err == nil {
			userName = user.RealName
		}
	}

	msg := &IncomingMessage{
		Channel:     ev.Channel,
		User:        ev.User,
		UserName:    userName,
		Text:        ev.Text,
		Timestamp:   ev.TimeStamp,
		ThreadTS:    ev.ThreadTimeStamp,
		IsFromBot:   isBotMessage,
		SenderAgent: "",
	}

	// For bot messages, set the sender agent name
	if isBotMessage && ev.Username != "" {
		msg.SenderAgent = ev.Username
	}

	c.messageQueue <- msg
}

func (c *Client) handleSlashCommand(cmd slack.SlashCommand) {
	// Handle /maxam commands
	switch cmd.Command {
	case "/maxam":
		c.handleMaxamCommand(cmd)
	}
}

func (c *Client) handleMaxamCommand(cmd slack.SlashCommand) {
	parts := strings.Fields(cmd.Text)
	if len(parts) == 0 {
		c.PostMessage(cmd.ChannelID, "Usage: /maxam <status|help>", "")
		return
	}

	switch parts[0] {
	case "status":
		c.PostMessage(cmd.ChannelID, "MAXAM is running. Team: Mei, Yuki, Priya, Amara", "")
	case "help":
		c.PostMessage(cmd.ChannelID, `MAXAM Commands:
• /maxam status - Check system status
• /maxam help - Show this help
• Just chat in the channel to interact with the team!`, "")
	default:
		c.PostMessage(cmd.ChannelID, "Unknown command. Try /maxam help", "")
	}
}

func (c *Client) processMessages(ctx context.Context, handler func([]*IncomingMessage)) {
	for {
		select {
		case <-ctx.Done():
			return

		case msg := <-c.messageQueue:
			c.pendingMsgs = append(c.pendingMsgs, msg)

			// Reset or start wait timer
			if c.waitTimer != nil {
				c.waitTimer.Stop()
			}
			c.waitTimer = time.AfterFunc(c.waitDuration, func() {
				if len(c.pendingMsgs) > 0 {
					// Process all pending messages
					msgs := c.pendingMsgs
					c.pendingMsgs = make([]*IncomingMessage, 0)
					handler(msgs)
				}
			})
		}
	}
}

// PostMessage sends a message to a channel
func (c *Client) PostMessage(channel, text, threadTS string) error {
	opts := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}

	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	_, _, err := c.api.PostMessage(channel, opts...)
	return err
}

// PostMessageAsAgent sends a message with agent identity
func (c *Client) PostMessageAsAgent(channel, agentName, text, threadTS string) error {
	emoji := getAgentEmoji(agentName)

	opts := []slack.MsgOption{
		slack.MsgOptionText(text, false),
		slack.MsgOptionUsername(agentName),
		slack.MsgOptionIconEmoji(emoji),
	}

	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	_, _, err := c.api.PostMessage(channel, opts...)
	return err
}

// AddReaction adds a reaction to a message
func (c *Client) AddReaction(channel, timestamp, emoji string) error {
	return c.api.AddReaction(emoji, slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	})
}

// Run starts the socket mode client (blocking)
func (c *Client) Run() error {
	return c.socket.Run()
}

func getAgentEmoji(name string) string {
	switch strings.ToLower(name) {
	case "mei", "mei chen":
		return ":coffee:" // コーヒー好き
	case "yuki", "yuki tanaka":
		return ":keyboard:" // キーボード好き
	case "priya", "priya sharma":
		return ":mag:" // レビュー
	case "amara", "amara okonkwo":
		return ":book:" // SF好き
	default:
		return ":robot_face:"
	}
}
