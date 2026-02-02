// Package workflow implements agent collaboration workflows
package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/logger"
)

// Threshold for determining if implementation is "lightweight"
const lightweightThreshold = 100

// ReviewCycle represents a Yuki -> Priya review cycle
type ReviewCycle struct {
	agents *agent.Agents
	logMgr *logger.Manager

	MaxIterations int
}

// NewReviewCycle creates a new review cycle workflow
func NewReviewCycle(agents *agent.Agents, logMgr *logger.Manager) *ReviewCycle {
	return &ReviewCycle{
		agents:        agents,
		logMgr:        logMgr,
		MaxIterations: 3, // Max review rounds before escalation
	}
}

// ReviewResult contains the outcome of a review cycle
type ReviewResult struct {
	Approved    bool
	Iterations  int
	Escalated   bool
	FinalOutput string
	History     []ReviewRound
}

// ReviewRound represents one round of implementation + review
type ReviewRound struct {
	Round          int
	Implementation string
	Review         string
	Approved       bool
	Tags           []string
}

// Run executes the review cycle for a given task
func (rc *ReviewCycle) Run(ctx context.Context, task string) (*ReviewResult, error) {
	result := &ReviewResult{
		History: make([]ReviewRound, 0),
	}

	yuki, ok := rc.agents.Get("yuki")
	if !ok {
		return nil, fmt.Errorf("yuki agent not found")
	}
	priya, ok := rc.agents.Get("priya")
	if !ok {
		return nil, fmt.Errorf("priya agent not found")
	}

	currentTask := task

	for i := 1; i <= rc.MaxIterations; i++ {
		round := ReviewRound{Round: i}
		fmt.Printf("\n=== Round %d ===\n", i)

		// Yuki implements
		fmt.Println("[Yuki] Implementing...")
		implPrompt := fmt.Sprintf(`タスク: %s

実装してください。コードを書く場合は、適切にファイルを作成・編集してください。`, currentTask)

		start := time.Now()
		impl, err := yuki.Run(ctx, implPrompt)
		if err != nil {
			return nil, fmt.Errorf("yuki implementation: %w", err)
		}
		round.Implementation = impl
		fmt.Printf("[Yuki] Done (%v)\n", time.Since(start).Round(time.Millisecond))

		// Log Yuki's work
		if log, err := rc.logMgr.Get("yuki"); err == nil {
			log.LogSimple(implPrompt, impl, time.Since(start))
		}

		// Priya reviews
		// Use Haiku for follow-up reviews (round > 1) with lightweight changes
		reviewModel := selectReviewModel(i, impl)
		modelInfo := ""
		if reviewModel != agent.ModelDefault {
			modelInfo = fmt.Sprintf(" (model: %s)", reviewModel)
		}
		fmt.Printf("[Priya] Reviewing...%s\n", modelInfo)

		reviewPrompt := fmt.Sprintf(`以下の実装をレビューしてください。

## タスク
%s

## Yukiの実装
%s

レビュー結果を以下の形式で出力してください:
- 問題がなければ「APPROVED」と書いてください
- 問題があれば「[MINOR]」「[DESIGN]」「[REQUIREMENTS]」「[SEC-CRITICAL]」のいずれかのタグをつけて指摘してください`, task, impl)

		start = time.Now()
		var review string
		if reviewModel != agent.ModelDefault {
			review, err = priya.RunWithModel(ctx, reviewPrompt, reviewModel)
		} else {
			review, err = priya.Run(ctx, reviewPrompt)
		}
		if err != nil {
			return nil, fmt.Errorf("priya review: %w", err)
		}
		round.Review = review
		fmt.Printf("[Priya] Done (%v)\n", time.Since(start).Round(time.Millisecond))

		// Log Priya's work
		if log, err := rc.logMgr.Get("priya"); err == nil {
			log.LogSimple(reviewPrompt, review, time.Since(start))
		}

		// Parse review result
		round.Tags = parseTags(review)
		round.Approved = strings.Contains(strings.ToUpper(review), "APPROVED")

		result.History = append(result.History, round)
		result.Iterations = i

		if round.Approved {
			fmt.Println("[Priya] Approved!")
			result.Approved = true
			result.FinalOutput = impl
			return result, nil
		}

		// Check for critical issues
		for _, tag := range round.Tags {
			if tag == "SEC-CRITICAL" || tag == "DESIGN" || tag == "REQUIREMENTS" {
				fmt.Printf("[Priya] Escalation required: %s\n", tag)
				result.Escalated = true
				return result, nil
			}
		}

		// Update task with feedback for next round
		currentTask = fmt.Sprintf(`元のタスク: %s

前回のレビューフィードバック:
%s

フィードバックを反映して修正してください。`, task, review)
	}

	// Max iterations reached
	fmt.Println("[System] Max iterations reached, escalating...")
	result.Escalated = true

	return result, nil
}

func parseTags(review string) []string {
	tags := []string{}
	possibleTags := []string{"MINOR", "DESIGN", "REQUIREMENTS", "SEC-CRITICAL"}

	upper := strings.ToUpper(review)
	for _, tag := range possibleTags {
		if strings.Contains(upper, "["+tag+"]") {
			tags = append(tags, tag)
		}
	}

	return tags
}

// selectReviewModel chooses model based on review round and implementation size
// - Round 1: Always use default (full review)
// - Round 2+: Use Haiku if implementation is lightweight
func selectReviewModel(round int, impl string) agent.Model {
	if round == 1 {
		return agent.ModelDefault
	}

	lines := strings.Count(impl, "\n") + 1
	if lines <= lightweightThreshold {
		return agent.ModelHaiku
	}

	return agent.ModelDefault
}
