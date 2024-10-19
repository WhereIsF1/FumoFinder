package renamer

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WhereIsF1/FumoFinder/internal/identifier" // Import the identifier package for MatchInfo
)

// FileRenamer handles renaming MKV files based on the majority episode result.
type FileRenamer struct {
	results     map[string][]identifier.MatchInfo // Map of MKV file name to a list of MatchInfo structs
	inputFolder string                            // Path to the folder where the MKV files are located
}

// NewFileRenamer creates a new FileRenamer with the given input folder.
func NewFileRenamer(inputFolder string) *FileRenamer {
	return &FileRenamer{
		results:     make(map[string][]identifier.MatchInfo),
		inputFolder: strings.TrimSpace(inputFolder), // Trim spaces from the folder path
	}
}

// AddResult adds an identification result for an MKV file using MatchInfo.
func (fr *FileRenamer) AddResult(match identifier.MatchInfo) {
	// Add the MatchInfo to the list associated with the MKV file name
	fr.results[match.VideoName] = append(fr.results[match.VideoName], match)
}

// RenameFiles renames the MKV files based on the majority episode number and title.
func (fr *FileRenamer) RenameFiles() {
	fmt.Println()
	fmt.Println("📝	Ready to rename files based on identified episodes.")
	fmt.Println("⚠️	Confirm renaming each file or choose to skip.")
	fmt.Println()

	// Ask if the user wants to use bulk mode
	if ConfirmBulkRename() {
		// Bulk renaming mode
		bulkPreview := make(map[string]string) // Store old and new file names

		fmt.Println()

		// Generate preview of all renames
		for mkvFile, matches := range fr.results {
			if len(matches) == 0 {
				fmt.Printf("❌	No episode results found for file: %s\n", mkvFile)
				continue
			}

			majorityTitle, majorityEpisode, confidence := findMajorityTitleAndEpisode(matches)
			if majorityEpisode == "" || majorityTitle == "" {
				fmt.Printf("❌	Failed to determine majority episode or title for file: %s\n", mkvFile)
				continue
			}

			if confidence < 0.90 {
				fmt.Printf("⚠️	The confidence level for episode %s is only %.0f%%. Results may not be reliable.\n", majorityEpisode, confidence*100)
			}

			fullPath := filepath.Join(fr.inputFolder, strings.TrimSpace(mkvFile))
			newFileName := constructNewFileName(fullPath, majorityTitle, majorityEpisode)

			bulkPreview[fullPath] = newFileName
		}

		// Show the user the old and new names for confirmation
		fmt.Println()
		fmt.Println("📋	Bulk Rename Preview:")
		fmt.Println()
		for oldName, newName := range bulkPreview {
			fmt.Printf("➡️	Original: %s\n", filepath.Base(oldName))
			fmt.Printf("➡️	New Name: %s\n\n", filepath.Base(newName))
		}

		// Ask for confirmation to proceed with the bulk rename
		fmt.Printf("↪️	Do you want to rename all files (y to confirm, n to cancel and go back to individual renaming)? ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "y" {
			// Proceed with bulk renaming
			fmt.Println()
			for oldName, newName := range bulkPreview {
				if err := os.Rename(oldName, newName); err != nil {
					fmt.Printf("❌	Failed to rename file %s: %v\n", oldName, err)
				} else {
					fmt.Printf("✅	Successfully renamed file to: %s\n", filepath.Base(newName))
				}
			}
			return // Exit after bulk renaming
		} else {
			fmt.Println("⏭️	Bulk renaming canceled. Proceeding with individual renaming.")
		}
	}

	// Call individual renaming for each file
	fmt.Println()

	for mkvFile, matches := range fr.results {
		fr.renameSingleFile(mkvFile, matches)
	}
}

// renameSingleFile handles the renaming of individual files based on the most common title and episode.
func (fr *FileRenamer) renameSingleFile(mkvFile string, matches []identifier.MatchInfo) {
	if len(matches) == 0 {
		fmt.Printf("❌	No episode results found for file: %s\n", mkvFile)
		return
	}

	// Determine the most common title and episode number
	majorityTitle, majorityEpisode, confidence := findMajorityTitleAndEpisode(matches)
	if majorityEpisode == "" || majorityTitle == "" {
		fmt.Printf("❌	Failed to determine majority episode or title for file: %s\n", mkvFile)
		return
	}

	// Warn the user if the confidence level is below 90%
	if confidence < 0.90 {
		fmt.Printf("⚠️	The confidence level for episode %s is only %.0f%%. Results may not be reliable.\n", majorityEpisode, confidence*100)
	}

	// Construct the full path to the original MKV file
	fullPath := filepath.Join(fr.inputFolder, strings.TrimSpace(mkvFile))

	// Check if the file exists before renaming
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Printf("❌	File does not exist: %s\n", fullPath)
		return
	}

	// Construct the new file name for the original MKV
	newFileName := constructNewFileName(fullPath, majorityTitle, majorityEpisode)
	fmt.Println()
	fmt.Printf("📍	Renaming File:\n")
	fmt.Printf("➡️	Original:  %s\n", filepath.Base(fullPath))
	fmt.Printf("➡️	New Name:  %s\n", filepath.Base(newFileName))

	// Prompt user for confirmation
	if confirmRename() {
		// Rename the original MKV file
		err := os.Rename(fullPath, newFileName)
		if err != nil {
			fmt.Println()
			log.Printf("❌	Failed to rename file %s: %v", fullPath, err)
			fmt.Println()
			fmt.Println()
		} else {
			fmt.Println()
			fmt.Printf("✅	Successfully renamed file to: %s\n", newFileName)
			fmt.Println()
			fmt.Println()
		}
	} else {
		fmt.Println()
		fmt.Printf("⏭️	Skipped renaming for file: %s\n", mkvFile)
		fmt.Println()
		fmt.Println()
	}
}

// findMajorityTitleAndEpisode finds the most frequent title and episode number in the list and calculates the confidence level.
func findMajorityTitleAndEpisode(matches []identifier.MatchInfo) (string, string, float64) {
	episodeCount := make(map[string]int)
	titleCount := make(map[string]int)

	for _, match := range matches {
		// Count episode occurrences
		episodeCount[match.Episode.String()]++

		// Count title occurrences (prioritize English, fallback to Romaji or Native)
		title := match.TitleEnglish
		if title == "" {
			title = match.TitleRomaji
		}
		if title == "" {
			title = match.TitleNative
		}
		titleCount[title]++
	}

	// Find the most common episode
	var majorityEpisode string
	maxEpisodeCount := 0
	totalCount := len(matches)
	for episode, count := range episodeCount {
		if count > maxEpisodeCount || (count == maxEpisodeCount && episode < majorityEpisode) {
			majorityEpisode = episode
			maxEpisodeCount = count
		}
	}

	// Find the most common title
	var majorityTitle string
	maxTitleCount := 0
	for title, count := range titleCount {
		if count > maxTitleCount || (count == maxTitleCount && title < majorityTitle) {
			majorityTitle = title
			maxTitleCount = count
		}
	}

	// Calculate confidence as the percentage of the majority episode count over total episodes
	confidence := float64(maxEpisodeCount) / float64(totalCount)
	return majorityTitle, majorityEpisode, confidence
}

// constructNewFileName constructs a new file name with the series title and episode number.
func constructNewFileName(originalPath, seriesTitle, episode string) string {
	// Format the episode number
	if len(episode) == 1 {
		episode = "0" + episode
	}

	// Replace spaces with dots in the series title
	seriesTitle = strings.ReplaceAll(seriesTitle, " ", ".")

	// Remove special characters, only allow alphanumeric characters and dots
	re := regexp.MustCompile(`[^a-zA-Z0-9.]`)
	seriesTitle = re.ReplaceAllString(seriesTitle, "")

	// Construct the new file name using series title and episode number
	ext := filepath.Ext(originalPath)
	baseDir := filepath.Dir(originalPath)
	newFileName := fmt.Sprintf("%s.E%s%s", seriesTitle, episode, ext)

	return filepath.Join(baseDir, newFileName)
}

// confirmRename prompts the user to confirm the renaming action using basic text input.
func confirmRename() bool {
	for {
		fmt.Printf("↪️	Do you want to rename (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "y" {
			return true
		} else if input == "n" {
			return false
		} else {
			fmt.Println("❌	Invalid input. Please type 'y' for yes or 'n' for no.")
		}
	}
}

// ConfirmBulkRename prompts the user to choose bulk renaming or individual renaming.
func ConfirmBulkRename() bool {
	for {
		fmt.Printf("↪️	Do you want to start Bulkrenamer (y to confirm, n to cancel and go back to individual renaming)? \n")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "y" {
			return true
		} else if input == "n" {
			return false
		} else {
			fmt.Println("❌	Invalid input. Please type 'y' for yes or 'n' for no.")
		}
	}
}
