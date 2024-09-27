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

// TraceMoeResult represents the response from the trace.moe API.
type TraceMoeResult struct {
	Anilist    AnilistInfo   `json:"anilist"` // Changed from struct to custom type
	Filename   string        `json:"filename"`
	Episode    EpisodeNumber `json:"episode"` // Use EpisodeNumber to handle both string and number
	From       float64       `json:"from"`
	To         float64       `json:"to"`
	Similarity float64       `json:"similarity"`
	Video      string        `json:"video"`
	Image      string        `json:"image"`
}

// AnilistInfo handles parsing of anilist data that might be in different formats
type AnilistInfo struct {
	ID       int      `json:"id"`
	IDMal    int      `json:"idMal"`
	Title    Title    `json:"title"`
	Synonyms []string `json:"synonyms"`
	IsAdult  bool     `json:"isAdult"`
	Raw      any      `json:"-"` // To hold raw data if parsing fails
}

// Title holds the title information of the anime
type Title struct {
	Native  string `json:"native"`
	Romaji  string `json:"romaji"`
	English string `json:"english"`
}

// UnmarshalJSON custom unmarshal to handle unexpected formats for anilist info
func (a *AnilistInfo) UnmarshalJSON(data []byte) error {
	// Define a temporary struct with the expected structure
	type Alias AnilistInfo
	var aux Alias
	if err := json.Unmarshal(data, &aux); err == nil {
		*a = AnilistInfo(aux)
		return nil
	}

	// If parsing as structured data fails, store the raw value
	var raw any
	if err := json.Unmarshal(data, &raw); err == nil {
		a.Raw = raw
		return nil
	}

	return fmt.Errorf("failed to unmarshal anilist info: %s", string(data))
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
