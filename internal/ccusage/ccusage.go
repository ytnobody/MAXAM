// Package ccusage provides integration with ccusage CLI tool for token usage tracking.
package ccusage

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Usage represents the usage data from ccusage
type Usage struct {
	TodayCost float64 // Today's cost in USD
	Available bool    // Whether ccusage is available
}

// DailyUsage represents a single day's usage from ccusage JSON output
type DailyUsage struct {
	Date      string  `json:"date"`
	TotalCost float64 `json:"totalCost"`
}

// ccusageOutput represents the full JSON output from ccusage
type ccusageOutput struct {
	Daily []DailyUsage `json:"daily"`
}

// Client provides methods to interact with ccusage CLI
type Client struct {
	timeout time.Duration
}

// NewClient creates a new ccusage client
func NewClient() *Client {
	return &Client{
		timeout: 10 * time.Second,
	}
}

// GetTodayUsage retrieves today's usage from ccusage
// Returns empty Usage with Available=false if ccusage is not installed or fails
func (c *Client) GetTodayUsage(ctx context.Context) Usage {
	// Check if npx is available
	if _, err := exec.LookPath("npx"); err != nil {
		return Usage{Available: false}
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Get today's date in YYYYMMDD format
	today := time.Now().Format("20060102")

	// Run ccusage with JSON output, filtering to today only
	cmd := exec.CommandContext(ctx, "npx", "ccusage", "daily", "--since", today, "--json")
	output, err := cmd.Output()
	if err != nil {
		return Usage{Available: false}
	}

	// Parse JSON output
	var result ccusageOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return Usage{Available: false}
	}

	// Get today's cost from daily array
	todayStr := time.Now().Format("2006-01-02")
	var totalCost float64
	for _, d := range result.Daily {
		if d.Date == todayStr {
			totalCost = d.TotalCost
			break
		}
	}

	return Usage{
		TodayCost: totalCost,
		Available: true,
	}
}

// FormatCost formats the cost as a string for display
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return "$0.00"
	}
	return fmt.Sprintf("$%.2f", cost)
}
