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
	"sync/atomic"
	"time"

	"github.com/WhereIsF1/FumoFinder/internal/model" // Import the model package for TraceMoeResponse
	"github.com/WhereIsF1/FumoFinder/internal/proxy" // Import proxy package to access ProxyDetails
)

// Define a struct for saving match information
type MatchInfo struct {
	AnilistID    int                 `json:"anilist_id"`
	MalID        int                 `json:"idMal"`
	TitleNative  string              `json:"title_native"`
	TitleRomaji  string              `json:"title_romaji"`
	TitleEnglish string              `json:"title_english"`
	Synonyms     []string            `json:"synonyms"`
	IsAdult      bool                `json:"is_adult"`
	Episode      model.EpisodeNumber `json:"episode"`
	Similarity   float64             `json:"similarity"`
	Timestamp    float64             `json:"timestamp"`
	From         float64             `json:"from"`
	To           float64             `json:"to"`
	VideoName    string              `json:"video_name"`
	FrameName    string              `json:"frame_name"`
	MatchedRange string              `json:"matched_range"`
	ProxyUsed    string              `json:"proxy_used"`
	VideoURL     string              `json:"video_url"`
	ImageURL     string              `json:"image_url"`
}

// EpisodeIdentifier handles identifying episodes using trace.moe
type EpisodeIdentifier struct {
	apiEndpoint    string                       // API endpoint for trace.moe
	aniListID      int                          // AniList ID to filter results
	Matches        []MatchInfo                  // Slice to store match information
	httpClients    map[*http.Client]string      // Map of HTTP clients with proxy URLs
	clientLocks    map[*http.Client]*sync.Mutex // Map to guard access to the clients
	frameCounts    map[string]int               // Map to track frames processed by each proxy
	failCounts     map[string]int               // Map to track failed attempts
	brokenProxies  map[string]bool              // Map to track broken proxies
	mu             sync.Mutex                   // Mutex to guard access to the maps
	done           chan struct{}                // Channel to signal when processing is complete
	channelClosed  atomic.Bool                  // Atomic flag to track if the channel is closed
	sendMutex      sync.Mutex                   // Mutex to guard access to the SafeSend function
	wg             sync.WaitGroup               // WaitGroup to wait for all workers to finish
	completionChan chan struct{}                // Channel to signal completion of identification process
}

// NewEpisodeIdentifier creates a new EpisodeIdentifier with optional proxy support
func NewEpisodeIdentifier(apiEndpoint string, aniListID int, proxies []proxy.ProxyDetails) *EpisodeIdentifier {
	clients := make(map[*http.Client]string)
	clientLocks := make(map[*http.Client]*sync.Mutex)
	frameCounts := make(map[string]int)
	failCounts := make(map[string]int)
	brokenProxies := make(map[string]bool)

	// Set up proxies
	if len(proxies) > 0 {
		for _, p := range proxies {
			transport := &http.Transport{Proxy: http.ProxyURL(p.URL)}
			client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
			clients[client] = p.URL.String()
			clientLocks[client] = &sync.Mutex{}
			frameCounts[p.URL.String()] = 0
			failCounts[p.URL.String()] = 0
			brokenProxies[p.URL.String()] = false
			fmt.Printf("ℹ️ Proxy %s has been configured.\n", p.URL)
		}
	} else {
		// Default direct connection if no proxies are given
		client := &http.Client{Timeout: 30 * time.Second}
		clients[client] = "No Proxy (Direct Connection)"
		clientLocks[client] = &sync.Mutex{}
		frameCounts["No Proxy (Direct Connection)"] = 0
		failCounts["No Proxy (Direct Connection)"] = 0
		brokenProxies["No Proxy (Direct Connection)"] = false
		fmt.Println("ℹ️ No proxies provided. Using direct connection.")
	}

	return &EpisodeIdentifier{
		apiEndpoint:    apiEndpoint,
		aniListID:      aniListID,
		Matches:        []MatchInfo{},
		httpClients:    clients,
		clientLocks:    clientLocks,
		frameCounts:    frameCounts,
		failCounts:     failCounts,
		brokenProxies:  brokenProxies,
		done:           make(chan struct{}),
		completionChan: make(chan struct{}), // Initialize completion channel
	}
}

// IdentifyEpisodes processes frames concurrently using multiple proxies with dynamic allocation
func (ei *EpisodeIdentifier) IdentifyEpisodes(frames []string, threshold float64) {
	frameChan := make(chan string, len(frames))

	// Load all frames into the shared channel
	for _, frame := range frames {
		frameChan <- frame
	}

	// Start processing frames dynamically with each proxy client concurrently
	for client, proxyURL := range ei.httpClients {
		ei.wg.Add(1)
		go ei.processFrames(client, proxyURL, threshold, frameChan)
	}

	// Wait for all goroutines to finish processing
	ei.wg.Wait()

	// Safely close the channel after all processing is done
	ei.CloseFramesChannel(frameChan)

	// Display summary of frames processed by each proxy
	ei.displayFrameProcessingSummary()

	fmt.Println("Episode identification process completed.")
	close(ei.completionChan) // Signal completion when the function exits
}

// SafeSend safely sends a frame back to the channel without panic
func (ei *EpisodeIdentifier) SafeSend(frames chan<- string, frame string) {
	ei.sendMutex.Lock()
	defer ei.sendMutex.Unlock()

	if !ei.channelClosed.Load() {
		select {
		case frames <- frame:
			// Successfully sent
		case <-ei.done:
			// Processing is complete, don't send
		default:
			// Channel might be full, don't block
		}
	}
}

// CloseFramesChannel safely closes the frames channel after all operations are completed
func (ei *EpisodeIdentifier) CloseFramesChannel(frames chan string) {
	close(ei.done)               // Signal that processing is complete
	ei.channelClosed.Store(true) // Mark the channel as closed
	ei.sendMutex.Lock()          // Lock to ensure no sends occur during closure
	defer ei.sendMutex.Unlock()  // Unlock after closing
	close(frames)                // Safely close the channel
}

// processFrames fetches frames from the channel and processes them
func (ei *EpisodeIdentifier) processFrames(client *http.Client, proxyURL string, threshold float64, frames chan string) {
	defer ei.wg.Done()

	// Create a ticker to periodically check the state of the channel
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				return // Channel is closed, exit the goroutine
			}

			// Check if the proxy is flagged as broken before processing
			ei.mu.Lock()
			if ei.brokenProxies[proxyURL] {
				ei.mu.Unlock()
				// Skip processing for broken proxy and terminate this worker
				fmt.Printf("⚠️ Proxy %s is marked as broken, terminating worker.\n", proxyURL)
				return // Exit to prevent further processing
			}
			ei.mu.Unlock()

			// Process the frame
			ei.clientLocks[client].Lock()
			info, similarity, err := ei.IdentifyEpisode(frame, threshold, client, proxyURL)
			ei.clientLocks[client].Unlock()

			if err != nil {
				if proxyURL != "No Proxy (Direct Connection)" {
					ei.handleProxyFailure(proxyURL, frames, frame) // Handle the broken proxy
					fmt.Printf("⚠️ Error identifying episode with proxy %s: %v\n", proxyURL, err)
					continue
				} else {
					// Retry logic for direct connections
					for retry := 1; retry <= 3; retry++ {
						ei.clientLocks[client].Lock()
						info, similarity, err = ei.IdentifyEpisode(frame, threshold, client, proxyURL)
						ei.clientLocks[client].Unlock()

						if err == nil && similarity > 0 {
							fmt.Println(info)
							break
						}

						fmt.Printf("⚠️ Retry %d/3 failed for direct connection: %v\n", retry, err)
					}

					if err != nil || similarity == 0 {
						fmt.Printf("⚠️ Dropping frame after repeated failures with direct connection: %s\n", frame)
						continue
					}
				}
			}

			if similarity == 0 {
				fmt.Printf("🔍 [DEBUG] No similar episode found for frame: %s\n", frame)
				continue
			}

			if similarity > 0 {
				fmt.Println(info)
			}

			ei.mu.Lock()
			ei.frameCounts[proxyURL]++
			ei.mu.Unlock()

		case <-ticker.C:
			// Periodically check if there are frames left to process
			if len(frames) == 0 {
				return
			}

		case <-ei.done:
			return // Processing is complete, exit the goroutine
		}
	}
}

func (ei *EpisodeIdentifier) handleProxyFailure(proxyURL string, frames chan string, frame string) {
	ei.mu.Lock()
	defer ei.mu.Unlock()

	ei.failCounts[proxyURL]++
	if ei.failCounts[proxyURL] >= 3 {
		// Mark the proxy as broken, remove it from the pool, and requeue the current frame
		ei.brokenProxies[proxyURL] = true
		delete(ei.httpClients, ei.getClientByProxyURL(proxyURL)) // Remove the proxy completely
		fmt.Printf("⚠️ Proxy %s has failed 3 times and will be removed from the pool.\n", proxyURL)

		// Requeue the current frame for processing by other working proxies
		ei.SafeSend(frames, frame)
	} else {
		fmt.Printf("⚠️ Proxy %s failed %d/3 times.\n", proxyURL, ei.failCounts[proxyURL])
	}
}

// getClientByProxyURL finds the client associated with the given proxy URL
func (ei *EpisodeIdentifier) getClientByProxyURL(proxyURL string) *http.Client {
	for client, url := range ei.httpClients {
		if url == proxyURL {
			return client
		}
	}
	return nil
}

// IdentifyEpisode identifies the episode by sending a frame to trace.moe using a specific client
func (ei *EpisodeIdentifier) IdentifyEpisode(imagePath string, threshold float64, client *http.Client, proxyURL string) (string, float64, error) {
	// Check if the proxy is flagged as broken, if so, skip using it
	ei.mu.Lock()
	if ei.brokenProxies[proxyURL] {
		ei.mu.Unlock()
		return "", 0, fmt.Errorf("proxy %s is marked as broken, skipping", proxyURL)
	}
	ei.mu.Unlock()

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
		if ei.aniListID != 0 && ei.aniListID != match.Anilist.ID {
			// Collect mismatch reason and skip to next result
			reasons = append(reasons, fmt.Sprintf(
				"❌ AniList ID Mismatch:\n   - Expected: %d\n   - Found: %d\n   - Video: %s\n   - Frame: %s",
				ei.aniListID, match.Anilist.ID, videoFilename, filepath.Base(imagePath)))
			continue
		}

		// Check if the extracted timestamp is within the range or threshold
		if (timestampSec >= match.From && timestampSec <= match.To) || // within range
			(timestampSec >= match.From-threshold && timestampSec < match.From) || // within threshold before `from`
			(timestampSec > match.To && timestampSec <= match.To+threshold) { // within threshold after `to`

			// Format episode number as string
			episodeStr := match.Episode.String()

			// Save match details
			matchInfo := MatchInfo{
				AnilistID:    match.Anilist.ID,
				MalID:        match.Anilist.IDMal,
				TitleNative:  match.Anilist.Title.Native,
				TitleRomaji:  match.Anilist.Title.Romaji,
				TitleEnglish: match.Anilist.Title.English,
				Synonyms:     match.Anilist.Synonyms,
				IsAdult:      match.Anilist.IsAdult,
				Episode:      match.Episode,
				Similarity:   match.Similarity * 100,
				Timestamp:    timestampSec,
				From:         match.From,
				To:           match.To,
				VideoName:    videoFilename,
				FrameName:    filepath.Base(imagePath),
				MatchedRange: fmt.Sprintf("%.2f to %.2f", match.From, match.To),
				ProxyUsed:    proxyURL,
				VideoURL:     match.Video,
				ImageURL:     match.Image,
			}

			// Add the match information to the EpisodeIdentifier's matches slice
			ei.mu.Lock()
			ei.Matches = append(ei.Matches, matchInfo)
			ei.mu.Unlock()

			// Check for English title; if empty, fall back to Romaji or Native title
			title := match.Anilist.Title.English
			if title == "" {
				title = match.Anilist.Title.Romaji
				if title == "" {
					title = match.Anilist.Title.Native
				}
			}

			// Display match info, including proxy used
			info := fmt.Sprintf(
				"\n✅ Match Found!\n"+
					"   - Title: %s\n"+ // Only the title will be shown
					"   - Episode: %s\n"+
					"   - Similarity: %.2f%%\n"+
					"   - Timestamp: %.2f (matches range %.2f to %.2f)\n"+
					"   - Video: %s\n"+
					"   - Frame: %s\n"+
					"   - Proxy Used: %s\n",
				title, episodeStr,
				match.Similarity*100, timestampSec, match.From, match.To,
				videoFilename, filepath.Base(imagePath), proxyURL,
			)
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

// WaitForCompletion waits for the identification process to complete
func (ei *EpisodeIdentifier) WaitForCompletion() {
	<-ei.completionChan
}

// displayFrameProcessingSummary prints the summary of frames processed by each proxy
func (ei *EpisodeIdentifier) displayFrameProcessingSummary() {
	fmt.Println("\n📊 Frame Processing Summary:")
	for proxy, count := range ei.frameCounts {
		fmt.Printf("   - %s processed %d frames\n", proxy, count)
	}
	fmt.Println(strings.Repeat("=", 50))
}
