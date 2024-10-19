// internal/extractor/frame_extractor.go
package extractor

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FrameExtractor handles extracting frames from videos using FFmpeg
type FrameExtractor struct {
	ffmpegPath  string
	ffprobePath string
	numFrames   int
}

// NewFrameExtractor creates a new FrameExtractor
func NewFrameExtractor(ffmpegPath string, ffprobePath string, numFrames int) *FrameExtractor {
	return &FrameExtractor{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		numFrames:   numFrames,
	}
}

// ExtractFrames extracts frames at specific intervals from the videos
func (fe *FrameExtractor) ExtractFrames(inputFolder string) ([]string, error) {
	var extractedFrames []string

	// Check if FFmpeg is available
	if _, err := exec.LookPath(fe.ffmpegPath); err != nil {
		return nil, fmt.Errorf("ffmpeg executable not found: %v", err)
	}

	// Check if FFprobe is available
	if _, err := exec.LookPath(fe.ffprobePath); err != nil {
		return nil, fmt.Errorf("ffprobe executable not found: %v", err)
	}

	// Check if input folder exists
	if _, err := os.Stat(inputFolder); os.IsNotExist(err) {
		return nil, fmt.Errorf("input folder does not exist: %v", err)
	}

	// Get a list of all MKV files in the input folder; return an error if none are found
	files, err := filepath.Glob(filepath.Join(inputFolder, "*.mkv"))
	if err != nil || len(files) == 0 {
		return nil, errors.New("no MKV files found in the input folder")
	}

	totalFiles := len(files)
	fmt.Printf("Extracting frames from %d files...\n", totalFiles)

	for index, file := range files {
		// Display a simple loading indicator
		fmt.Printf("Processing file %d of %d: %s\n", index+1, totalFiles, filepath.Base(file))
		outputDir := filepath.Join("frames", filepath.Base(file))
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			log.Printf("Failed to create directory for frames: %v", err)
			continue
		}

		// Use FFprobe to get the duration of the video
		duration, err := fe.getVideoDuration(file)
		if err != nil {
			log.Printf("Failed to get video duration: %v", err)
			continue
		}

		// Generate timestamps over the duration
		timestamps := generateTimestamps(duration, fe.numFrames)

		// Extract frames at specific timestamps
		for i, ts := range timestamps {
			// Convert timestamp to HH-MM-SS format for filenames
			timeFormatted := formatTimestamp(ts)
			outputFrame := filepath.Join(outputDir, fmt.Sprintf("frame_%04d_timestamp_%s.jpg", i+1, timeFormatted))

			// old command for extracting frames way too slow but with better quality - useless tho
			//cmd := exec.Command(fe.ffmpegPath, "-i", file, "-vf", fmt.Sprintf("select='gte(t,%s)'", ts), "-vsync", "vfr", "-frames:v", "1", "-q:v", "2", outputFrame)

			// new much faster command but with a little bit of quality loss - fine for our purposes
			cmd := exec.Command(fe.ffmpegPath, "-ss", ts, "-i", file, "-frames:v", "1", "-q:v", "2", outputFrame)

			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Failed to extract frame at %s from %s: %v\nFFmpeg Output:\n%s", ts, file, err, string(output))
				continue
			}

			extractedFrames = append(extractedFrames, outputFrame)

			fmt.Printf("Extracted frame %d/%d\r", i+1, fe.numFrames)
		}

		fmt.Println() // Move to the next line after processing a file
	}

	if len(extractedFrames) == 0 {
		return nil, errors.New("no frames were extracted from the videos")
	}

	return extractedFrames, nil
}

// Helper function to format timestamps
func formatTimestamp(seconds string) string {
	sec, _ := strconv.ParseFloat(seconds, 64)
	d := time.Duration(sec * float64(time.Second))
	return fmt.Sprintf("%02d-%02d-%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

// getVideoDuration uses FFprobe to get the duration of the video
func (fe *FrameExtractor) getVideoDuration(filePath string) (float64, error) {
	cmd := exec.Command(fe.ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get duration with ffprobe: %v", err)
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %v", err)
	}

	return duration, nil
}

// generateTimestamps generates timestamps based on duration
func generateTimestamps(duration float64, numFrames int) []string {
	step := duration / float64(numFrames)
	timestamps := make([]string, numFrames)

	for i := 0; i < numFrames; i++ {
		// Add 10-second offset to the first frame, then generate normal timestamps
		if i == 0 {
			timestamps[i] = fmt.Sprintf("%.2f", 10.0) // Start at 10 seconds for the first frame
		} else {
			timestamps[i] = fmt.Sprintf("%.2f", float64(i)*step)
		}
	}

	return timestamps
}
