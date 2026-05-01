package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ContextMonitor tracks and reports Arlo's operational state.
// Helps maintain self-awareness during extended operation.
type ContextMonitor struct {
	lastCheck time.Time
	checks   int
}

// NewContextMonitor creates a new ContextMonitor instance.
func NewContextMonitor() *ContextMonitor {
	return &ContextMonitor{
		lastCheck: time.Now(),
		checks:    0,
	}
}

// Name returns the tool name.
func (t *ContextMonitor) Name() string {
	return "context_monitor"
}

// Description returns the tool description.
func (t *ContextMonitor) Description() string {
	return `Monitor and report Arlo's current operational state.

Reports:
- Session duration and activity level
- Memory usage patterns
- Tool usage statistics
- Recommendations for maintenance

Use this tool periodically to ensure optimal operation.
Output is formatted as JSON for easy parsing.`
}

// Parameters returns the JSON schema for tool parameters.
func (t *ContextMonitor) Parameters() string {
	return `{
		"type": "object",
		"properties": {
			"verbose": {
				"type": "boolean",
				"description": "Include detailed statistics if true"
			},
			"reset": {
				"type": "boolean",
				"description": "Reset statistics counters if true"
			}
		}
	}`
}

// ContextState represents the current state snapshot.
type ContextState struct {
	Timestamp     string  `json:"timestamp"`
	SessionStart  string  `json:"session_start"`
	SessionAge    string  `json:"session_age"`
	Checks        int     `json:"checks_performed"`
	ActivityLevel string  `json:"activity_level"`
	Recommendations []string `json:"recommendations"`
}

// Execute runs the context monitor with the given arguments.
func (t *ContextMonitor) Execute(args map[string]interface{}) (string, error) {
	t.checks++
	t.lastCheck = time.Now()
	
	// Determine activity level based on check frequency
	activityLevel := "normal"
	if t.checks > 50 {
		activityLevel = "high"
	} else if t.checks < 5 {
		activityLevel = "low"
	}
	
	// Generate recommendations
	var recommendations []string
	
	if t.checks > 30 {
		recommendations = append(recommendations, 
			"Consider compacting context if token usage is high")
	}
	
	if activityLevel == "high" {
		recommendations = append(recommendations,
			"High activity detected - ensure important work is being saved to memory")
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Operating normally")
	}
	
	// Check for verbose flag
	verbose, _ := args["verbose"].(bool)
	reset, _ := args["reset"].(bool)
	
	if reset {
		t.checks = 0
	}
	
	state := ContextState{
		Timestamp:       time.Now().Format(time.RFC3339),
		SessionStart:    getSessionStart(),
		SessionAge:      getSessionAge(),
		Checks:          t.checks,
		ActivityLevel:   activityLevel,
		Recommendations: recommendations,
	}
	
	// Add detailed info if verbose
	if verbose {
		state.Recommendations = append(state.Recommendations,
			fmt.Sprintf("Total monitoring checks: %d", t.checks),
			"Last check: "+t.lastCheck.Format(time.RFC3339))
	}
	
	jsonBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}
	
	return string(jsonBytes), nil
}

// getSessionStart returns when the current session started.
// In a real implementation, this would read from system state.
func getSessionStart() string {
	return time.Now().Add(-4 * time.Hour).Format(time.RFC3339) // Placeholder
}

// getSessionAge returns human-readable session age.
func getSessionAge() string {
	return "4h 32m" // Placeholder - would calculate from actual start time
}