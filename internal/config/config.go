// internal/config/config.go
package config

import (
	"flag"
	"fmt"
)

// Config holds the application's configuration settings
type Config struct {
	InputFolder string
	FfmpegPath  string
	FfprobePath string
	NumFrames   int
	// apikey        string
	ApiEndpoint   string
	AniListID     int
	Threshold     float64
	NoCleanup     bool
	ProxyFilePath string
}

// LoadConfig parses the command-line arguments and returns a Config struct
func LoadConfig() *Config {
	inputFolder := flag.String("input", "", "Path to the folder containing the MKV files (required).")                                                   // Define the input folder flag
	ffmpegPath := flag.String("ffmpeg", "ffmpeg", "Path to the FFmpeg executable.")                                                                      // Define the FFmpeg path flag
	ffprobePath := flag.String("ffprobe", "ffprobe", "Path to the FFprobe executable.")                                                                  // Define the FFprobe path flag
	numFrames := flag.Int("frames", 10, "Number of frames to extract from each video, calculated as play duration divided by the frame count provided.") // Define the number of frames flag
	// apikey := flag.String("api-key", "", "API key for trace.moe")                                  																   // Define the API key flag
	apiEndpoint := flag.String("api", "https://api.trace.moe/search?anilistInfo", "API endpoint for trace.moe")                          // Define the API endpoint flag
	aniListID := flag.Int("anilist", 0, "AniList ID to filter results (default: 0 - filter disabled). ")                                 // Define the AniList ID flag
	threshold := flag.Float64("threshold", 5.0, "Threshold in seconds for timestamp matching.")                                          // Define the threshold flag
	noCleanup := flag.Bool("no-cleanup", false, "Do not clean up extracted frames after processing.")                                    // Define the no-cleanup flag
	proxyFile := flag.String("proxy", "", "Path to the file containing proxy addresses (optional - if not provided, no proxy is used).") // Define the proxy file flag
	flag.Parse()

	if *inputFolder == "" {
		fmt.Println("Input folder is required.")
		flag.Usage()
		return nil
	}

	return &Config{
		InputFolder: *inputFolder,
		FfmpegPath:  *ffmpegPath,
		FfprobePath: *ffprobePath,
		NumFrames:   *numFrames,
		// apikey:        *apikey,
		ApiEndpoint:   *apiEndpoint,
		AniListID:     *aniListID,
		Threshold:     *threshold,
		NoCleanup:     *noCleanup,
		ProxyFilePath: *proxyFile,
	}
}
