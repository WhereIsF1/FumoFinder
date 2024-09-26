// internal/identifier/episode_identifier.go
package identifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WhereIsF1/FumoFinder/internal/model" // Import the model package for TraceMoeResponse
	"github.com/WhereIsF1/FumoFinder/internal/proxy" // Import proxy package to access ProxyDetails
)

// Define a struct for saving match information
type MatchInfo struct {
	AnilistID    int     `json:"anilist_id"`
	Episode      string  `json:"episode"`
	Similarity   float64 `json:"similarity"`
	Timestamp    float64 `json:"timestamp"`
	From         float64 `json:"from"`
	To           float64 `json:"to"`
	VideoName    string  `json:"video_name"`
	FrameName    string  `json:"frame_name"`
	MatchedRange string  `json:"matched_range"`
	ProxyUsed    string  `json:"proxy_used"`
}

// EpisodeIdentifier handles identifying episodes using trace.moe
type EpisodeIdentifier struct {
	apiEndpoint string
	aniListID   int
	Matches     []MatchInfo
	httpClients map[*http.Client]string      // Map to track clients and their corresponding proxy URLs
	clientLocks map[*http.Client]*sync.Mutex // Map to track a mutex for each client to prevent concurrent usage
	frameCounts map[string]int               // Map to count frames processed by each proxy
	mu          sync.Mutex                   // Mutex to safely update matches and counters
}

// NewEpisodeIdentifier creates a new EpisodeIdentifier with optional proxy support
func NewEpisodeIdentifier(apiEndpoint string, aniListID int, proxies []proxy.ProxyDetails) *EpisodeIdentifier {
	clients := make(map[*http.Client]string)
	clientLocks := make(map[*http.Client]*sync.Mutex)
	frameCounts := make(map[string]int)

	// Check if proxies are provided
	if len(proxies) > 0 {
		// Set up multiple clients with proxies
		for _, p := range proxies {
			transport := &http.Transport{
				Proxy: http.ProxyURL(p.URL),
			}

			client := &http.Client{
				Transport: transport,
				Timeout:   30 * time.Second,
			}

			clients[client] = p.URL.String()    // Track which proxy URL corresponds to each client
			clientLocks[client] = &sync.Mutex{} // Create a mutex for each client to control access
			frameCounts[p.URL.String()] = 0     // Initialize frame counter for each proxy
			fmt.Printf("ℹ️ Proxy %s has been configured.\n", p.URL)
		}
	} else {
		// Set up a default client without a proxy if no proxies are given
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		clients[client] = "No Proxy (Direct Connection)"
		clientLocks[client] = &sync.Mutex{}
		frameCounts["No Proxy (Direct Connection)"] = 0 // Initialize frame counter for direct connection
		fmt.Println("ℹ️ No proxies provided. Using direct connection.")
	}

	return &EpisodeIdentifier{
		apiEndpoint: apiEndpoint,
		aniListID:   aniListID,
		Matches:     []MatchInfo{},
		httpClients: clients,
		clientLocks: clientLocks,
		frameCounts: frameCounts,
	}
}

// IdentifyEpisodes processes frames concurrently using multiple proxies with dynamic allocation
func (ei *EpisodeIdentifier) IdentifyEpisodes(frames []string, threshold float64) {
	var wg sync.WaitGroup
	frameChan := make(chan string, len(frames))

	// Load all frames into the shared channel
	for _, frame := range frames {
		frameChan <- frame
	}
	close(frameChan)

	// Start processing frames dynamically with each proxy client concurrently
	for client, proxyURL := range ei.httpClients {
		wg.Add(1)
		go ei.processFrames(client, proxyURL, threshold, frameChan, &wg)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Display summary of frames processed by each proxy
	ei.displayFrameProcessingSummary()
}

// processFrames dynamically fetches frames from the channel and processes them using the specific client
func (ei *EpisodeIdentifier) processFrames(client *http.Client, proxyURL string, threshold float64, frames <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for frame := range frames {
		// Lock the mutex for this client to ensure sequential requests
		ei.clientLocks[client].Lock()
		info, similarity, err := ei.IdentifyEpisode(frame, threshold, client, proxyURL)
		// Unlock the mutex after the request is done
		ei.clientLocks[client].Unlock()

		if err != nil {
			fmt.Printf("Error identifying episode: %v\n", err)
			continue
		}

		// Log match information if a match is found
		if similarity > 0 {
			fmt.Println(info)
		}

		// Increment the frame count for the proxy
		ei.mu.Lock()
		ei.frameCounts[proxyURL]++
		ei.mu.Unlock()
	}
}

// IdentifyEpisode identifies the episode by sending a frame to trace.moe using a specific client
func (ei *EpisodeIdentifier) IdentifyEpisode(imagePath string, threshold float64, client *http.Client, proxyURL string) (string, float64, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open frame: %v", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	if _, err = buf.ReadFrom(file); err != nil {
		return "", 0, fmt.Errorf("failed to read frame: %v", err)
	}

	// Ensure requests go through the provided client
	req, err := http.NewRequest("POST", ei.apiEndpoint, &buf)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request to trace.moe: %v", err)
	}
	req.Header.Set("Content-Type", "image/jpeg")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send frame to trace.moe: %v", err)
	}
	defer resp.Body.Close()

	var result model.TraceMoeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, fmt.Errorf("failed to parse trace.moe response: %v", err)
	}

	// Extract timestamp from the frame filename in seconds
	timestampSec := extractTimestampInSeconds(imagePath)
	var reasons []string         // To collect reasons for mismatches
	foundPotentialMatch := false // Flag to indicate potential matches

	// Extract video filename
	videoFilename := filepath.Base(filepath.Dir(imagePath))

	// Iterate through results to find matches based on AniList ID
	for _, match := range result.Result {
		// Check AniList ID match
		if ei.aniListID != 0 && ei.aniListID != match.AnilistID {
			// Collect mismatch reason and skip to next result
			reasons = append(reasons, fmt.Sprintf(
				"❌ AniList ID Mismatch:\n   - Expected: %d\n   - Found: %d\n   - Video: %s\n   - Frame: %s",
				ei.aniListID, match.AnilistID, videoFilename, filepath.Base(imagePath)))
			continue
		}

		// Check if the extracted timestamp is within the range or threshold
		if (timestampSec >= match.From && timestampSec <= match.To) || // within range
			(timestampSec >= match.From-threshold && timestampSec < match.From) || // within threshold before `from`
			(timestampSec > match.To && timestampSec <= match.To+threshold) { // within threshold after `to`

			// Save match details
			matchInfo := MatchInfo{
				AnilistID:    match.AnilistID,
				Episode:      match.Episode.String(),
				Similarity:   match.Similarity * 100,
				Timestamp:    timestampSec,
				From:         match.From,
				To:           match.To,
				VideoName:    videoFilename,
				FrameName:    filepath.Base(imagePath),
				MatchedRange: fmt.Sprintf("%.2f to %.2f", match.From, match.To),
				ProxyUsed:    proxyURL, // Store which proxy was used
			}

			// Add the match information to the EpisodeIdentifier's matches slice
			ei.mu.Lock()
			ei.Matches = append(ei.Matches, matchInfo)
			ei.mu.Unlock()

			// Display match info, including proxy used
			info := fmt.Sprintf(
				"\n✅ Match Found!\n"+
					"   - Title: %d\n"+
					"   - Episode: %s\n"+
					"   - Similarity: %.2f%%\n"+
					"   - Timestamp: %.2f (matches range %.2f to %.2f)\n"+
					"   - Video: %s\n"+
					"   - Frame: %s\n"+
					"   - Proxy Used: %s\n", // Display which proxy was used
				match.AnilistID, match.Episode.String(), match.Similarity*100, timestampSec, match.From, match.To,
				videoFilename, filepath.Base(imagePath), proxyURL)
			return info, match.Similarity, nil
		} else {
			// Set the flag if we are still processing potential matches, but the timestamp doesn't match
			foundPotentialMatch = true
			// Collect reason for timestamp mismatch
			reasons = append(reasons, fmt.Sprintf(
				"❌ Timestamp Mismatch:\n   - Timestamp: %.2f\n   - Expected Range: %.2f to %.2f\n   - Threshold: ±%.2f seconds\n   - Video: %s\n   - Frame: %s",
				timestampSec, match.From, match.To, threshold, videoFilename, filepath.Base(imagePath)))
		}
	}

	// Log only the most relevant reason if no match is found after checking all results
	if foundPotentialMatch && len(reasons) > 0 {
		fmt.Printf(
			"\n❌ Failed to Identify Episode for Frame:\n   - Video: %s\n   - Frame: %s\n"+
				"🔍 Reason: %s\n"+
				"   - Checked %d potential matches.\n\n",
			videoFilename, filepath.Base(imagePath), reasons[0], len(reasons))
	} else {
		fmt.Printf(
			"\n❌ No Match Found for Frame:\n   - Video: %s\n   - Frame: %s\n"+
				"🔍 Reason: No potential matches found.\n\n",
			videoFilename, filepath.Base(imagePath))
	}

	return "", 0, nil
}

// ExtractTimestampInSeconds extracts the timestamp from the frame filename in seconds
func extractTimestampInSeconds(imagePath string) float64 {
	filename := filepath.Base(imagePath)
	parts := strings.Split(filename, "_timestamp_")
	if len(parts) < 2 {
		return 0
	}
	timestamp := strings.TrimSuffix(parts[1], ".jpg")
	parsedTime, err := time.Parse("15-04-05", timestamp)
	if err != nil {
		return 0
	}
	return float64(parsedTime.Hour()*3600 + parsedTime.Minute()*60 + parsedTime.Second())
}

// displayFrameProcessingSummary prints the summary of frames processed by each proxy
func (ei *EpisodeIdentifier) displayFrameProcessingSummary() {
	fmt.Println("\n📊 Frame Processing Summary:")
	for proxy, count := range ei.frameCounts {
		fmt.Printf("   - %s processed %d frames\n", proxy, count)
	}
	fmt.Println(strings.Repeat("=", 50))
}
