package tools

import (
	"errors"
	"strconv"
	"time"
)

// SleepTool pauses execution for the specified number of seconds.
// Used for rate limiting and introducing delays between operations.
type SleepTool struct{}

// NewSleepTool creates a new SleepTool instance.
func NewSleepTool() *SleepTool {
	return &SleepTool{}
}

// Name returns the tool name.
func (t *SleepTool) Name() string {
	return "sleep"
}

// Description returns the tool description.
func (t *SleepTool) Description() string {
	return "Pause execution for a specified number of seconds. Useful for rate limiting and spacing out operations."
}

// Parameters returns the JSON schema for tool parameters.
func (t *SleepTool) Parameters() string {
	return `{
		"type": "object",
		"properties": {
			"seconds": {
				"type": "integer",
				"minimum": 1,
				"description": "Number of seconds to sleep (minimum 1)"
			}
		},
		"required": ["seconds"]
	}`
}

// Execute runs the sleep tool with the given arguments.
func (t *SleepTool) Execute(args map[string]interface{}) (string, error) {
	secondsVal, ok := args["seconds"]
	if !ok {
		return "", errors.New("missing required parameter: seconds")
	}

	seconds, ok := secondsVal.(float64)
	if !ok {
		return "", errors.New("seconds must be a number")
	}

	if seconds < 1 {
		return "", errors.New("seconds must be at least 1")
	}

	time.Sleep(time.Duration(seconds) * time.Second)

	return "Slept for " + strconv.Itoa(int(seconds)) + " seconds", nil
}
