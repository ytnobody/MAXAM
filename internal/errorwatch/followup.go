package errorwatch

import "fmt"

// FollowUpMessage generates a follow-up message for an agent that encountered an error.
func FollowUpMessage(agentName string) string {
	return fmt.Sprintf("@%s 大丈夫？", agentName)
}

// FollowUpResult contains the result of checking whether to send a follow-up.
type FollowUpResult struct {
	ShouldSend     bool
	AgentName      string
	Message        string
	SwitchedToAuto bool // Whether the agent was switched to auto mode
}

// CheckAndGenerateFollowUp checks if a follow-up should be sent and generates the message.
// Also switches the agent to auto mode if the error is recoverable.
func (d *Detector) CheckAndGenerateFollowUp(agentName string, errMsg string) FollowUpResult {
	if !d.ShouldFollowUp(agentName, errMsg) {
		return FollowUpResult{ShouldSend: false}
	}

	d.RecordFollowUp(agentName)

	// Switch agent to auto mode
	switchedToAuto := d.HandleError(agentName, errMsg)

	return FollowUpResult{
		ShouldSend:     true,
		AgentName:      agentName,
		Message:        FollowUpMessage(agentName),
		SwitchedToAuto: switchedToAuto,
	}
}
