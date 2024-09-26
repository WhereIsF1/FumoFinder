// internal/model/response.go
package model

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// TraceMoeResponse represents the response from the trace.moe API
type TraceMoeResponse struct {
	FrameCount int              `json:"frameCount"`
	Error      string           `json:"error"`
	Result     []TraceMoeResult `json:"result"`
}

// TraceMoeResult represents individual results from trace.moe
type TraceMoeResult struct {
	AnilistID  int           `json:"anilist"`
	Filename   string        `json:"filename"`
	Episode    EpisodeNumber `json:"episode"`
	From       float64       `json:"from"`
	To         float64       `json:"to"`
	Similarity float64       `json:"similarity"`
	Video      string        `json:"video"`
	Image      string        `json:"image"`
}

// EpisodeNumber handles parsing of episode numbers that might be string, float, or unexpected formats
type EpisodeNumber struct {
	Number float64
	Raw    string
}

// UnmarshalJSON custom unmarshal to handle both string and float formats for episode numbers
func (e *EpisodeNumber) UnmarshalJSON(data []byte) error {
	var num float64
	// Try parsing as a float
	if err := json.Unmarshal(data, &num); err == nil {
		e.Number = num
		e.Raw = fmt.Sprintf("%.0f", num)
		return nil
	}

	// If parsing as float fails, try parsing as a string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		// Attempt to parse as a float if the string is clean
		parsedNum, err := strconv.ParseFloat(str, 64)
		if err == nil {
			e.Number = parsedNum
			e.Raw = fmt.Sprintf("%.0f", parsedNum)
		} else {
			// If parsing as float fails, store the raw value
			e.Raw = str
		}
		return nil
	}

	// Default case when parsing fails
	e.Raw = string(data)
	return fmt.Errorf("failed to unmarshal episode number: %s", string(data))
}

// String returns a formatted string representation of the episode number
func (e EpisodeNumber) String() string {
	if e.Number != 0 {
		return fmt.Sprintf("%.0f", e.Number)
	}
	return e.Raw
}
