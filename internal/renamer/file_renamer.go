package renamer

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/WhereIsF1/FumoFinder/internal/identifier" // Import the identifier package for MatchInfo
)

// FileRenamer handles renaming MKV files based on the majority episode result.
type FileRenamer struct {
	results     map[string][]string // Map of MKV file name to a list of episode numbers
	inputFolder string              // Path to the folder where the MKV files are located
}

// NewFileRenamer creates a new FileRenamer with the given input folder.
func NewFileRenamer(inputFolder string) *FileRenamer {
	return &FileRenamer{
		results:     make(map[string][]string),
		inputFolder: strings.TrimSpace(inputFolder), // Trim spaces from the folder path
	}
}

// AddResult adds an identification result for an MKV file using MatchInfo.
func (fr *FileRenamer) AddResult(match identifier.MatchInfo) {
	// Add the episode number to the list associated with the MKV file name
	fr.results[match.VideoName] = append(fr.results[match.VideoName], match.Episode.String())
}

// RenameFiles renames the MKV files based on the majority episode number.
func (fr *FileRenamer) RenameFiles() {
	fmt.Println()
	fmt.Println("📝 Ready to rename files based on identified episodes.")
	fmt.Println("⚠️ Confirm renaming each file or choose to skip.")
	fmt.Println()

	for mkvFile, episodes := range fr.results {
		if len(episodes) == 0 {
			fmt.Printf("❌ No episode results found for file: %s\n", mkvFile)
			continue
		}

		// Determine the most common episode number
		majorityEpisode, confidence := findMajorityEpisode(episodes)
		if majorityEpisode == "" {
			fmt.Printf("❌ Failed to determine majority episode for file: %s\n", mkvFile)
			continue
		}

		// Warn the user if the confidence level is below 75%
		if confidence < 0.90 {
			fmt.Printf("⚠️ The confidence level for episode %s is only %.0f%%. Results may not be reliable.\n", majorityEpisode, confidence*100)
		}

		// Construct the full path to the original MKV file
		fullPath := filepath.Join(fr.inputFolder, strings.TrimSpace(mkvFile))

		// Check if the file exists before renaming
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			log.Printf("❌ File does not exist: %s\n", fullPath)
			continue
		}

		// Construct the new file name for the original MKV
		newFileName := constructNewFileName(fullPath, majorityEpisode)
		fmt.Println()
		fmt.Printf("📍	Renaming File:\n")
		fmt.Printf("➡️	Original:  %s\n", filepath.Base(fullPath))
		fmt.Printf("➡️	New Name:  %s\n", filepath.Base(newFileName))

		// Prompt user for confirmation
		if confirmRename() {
			// Rename the original MKV file
			err := os.Rename(fullPath, newFileName)
			if err != nil {
				log.Printf("❌ Failed to rename file %s: %v", fullPath, err)
				fmt.Println()
				fmt.Println()
			} else {
				fmt.Printf("✅ Successfully renamed file to: %s\n", newFileName)
				fmt.Println()
				fmt.Println()
			}
		} else {
			fmt.Printf("⏭️ Skipped renaming for file: %s\n", mkvFile)
			fmt.Println()
			fmt.Println()
		}
	}
}

// findMajorityEpisode finds the most frequently occurring episode in the list and calculates the confidence level.
func findMajorityEpisode(episodes []string) (string, float64) {
	episodeCount := make(map[string]int)
	for _, episode := range episodes {
		episodeCount[episode]++
	}

	// Find the episode with the highest count
	var majorityEpisode string
	maxCount := 0
	totalCount := len(episodes)

	for episode, count := range episodeCount {
		if count > maxCount || (count == maxCount && episode < majorityEpisode) {
			majorityEpisode = episode
			maxCount = count
		}
	}

	// Calculate confidence as the percentage of the majority episode count over total episodes
	confidence := float64(maxCount) / float64(totalCount)
	return majorityEpisode, confidence
}

// constructNewFileName constructs a new file name with the episode number.
func constructNewFileName(originalPath, episode string) string {
	// Format the episode number
	if len(episode) == 1 {
		episode = "0" + episode
	}

	ext := filepath.Ext(originalPath)
	baseName := strings.TrimSuffix(originalPath, ext)
	newFileName := fmt.Sprintf("%s_E%s%s", baseName, episode, ext)
	return newFileName
}

// confirmRename prompts the user to confirm the renaming action using basic text input.
func confirmRename() bool {
	fmt.Printf("↪️ Do you want to rename (y/n): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(input)) == "y"
}
