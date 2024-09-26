package main

//	todo:
//	implement api key usage
//	update identifier to use api key if provided
//  implement something to counterfight proxy failure/timeout - just dont use bad proxies right?

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/WhereIsF1/FumoFinder/internal/config"     // Import the config package
	"github.com/WhereIsF1/FumoFinder/internal/extractor"  // Import the extractor package
	"github.com/WhereIsF1/FumoFinder/internal/identifier" // Import the identifier package
	"github.com/WhereIsF1/FumoFinder/internal/proxy"      // Import the proxy package
	"github.com/WhereIsF1/FumoFinder/internal/renamer"    // Import the renamer package
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Check if help is needed or no arguments are provided.
	if len(os.Args) == 1 || hasHelpFlag() {
		printHelpHeader()
		printHelp()
		return
	}

	// Print the ASCII art header
	printHeader()

	// Load configurations from command line arguments
	cfg := config.LoadConfig()

	// Print the loaded configuration settings
	printConfig(cfg)

	// Extract frames from each video file in the specified folder
	frameExtractor := extractor.NewFrameExtractor(cfg.FfmpegPath, cfg.FfprobePath, cfg.NumFrames)
	frames, err := frameExtractor.ExtractFrames(cfg.InputFolder)
	if err != nil {
		log.Fatalf("Error extracting frames: %v", err)
	}

	// Initialize the proxy loader and load proxies after frame extraction
	var proxies []*url.URL
	if cfg.ProxyFilePath != "" {
		// If the proxy file path is specified, load proxies
		proxyLoader := proxy.NewProxyLoader()
		err := proxyLoader.LoadProxies(cfg.ProxyFilePath)
		if err != nil {
			log.Printf("Error loading proxies: %v", err)
		} else {
			proxies = proxyLoader.GetProxyList()
			if len(proxies) > 0 {
				fmt.Println("✅ Proxies loaded successfully.")
			} else {
				fmt.Println("⚠️ No working proxies found. Proceeding without proxies.")
			}
		}
	} else {
		fmt.Println("ℹ️ No proxy file specified.")
	}

	// Convert []*url.URL to []proxy.ProxyDetails, or use an empty list if no proxies are loaded
	var proxyDetails []proxy.ProxyDetails
	for _, p := range proxies {
		proxyDetails = append(proxyDetails, proxy.ProxyDetails{URL: p})
	}

	// Initialize the episode identifier with the loaded proxies (or direct connection if none)
	episodeIdentifier := identifier.NewEpisodeIdentifier(cfg.ApiEndpoint, cfg.AniListID, proxyDetails)

	// Initialize the file renamer
	fileRenamer := renamer.NewFileRenamer(cfg.InputFolder)

	// Process each frame and identify the episode
	fmt.Println(strings.Repeat("-", 50))                      // Separator before processing frames
	episodeIdentifier.IdentifyEpisodes(frames, cfg.Threshold) // Process frames concurrently using multiple proxies or direct connection

	// Check if matches are available and add them to the renamer
	for _, match := range episodeIdentifier.Matches {
		fileRenamer.AddResult(match) // Add MatchInfo to the file renamer
	}

	// Rename the files based on majority episode results
	fileRenamer.RenameFiles()
	fmt.Println(strings.Repeat("=", 50)) // End separator

	// Perform cleanup if the no-cleanup flag is not set
	if !cfg.NoCleanup {
		cleanupExtractedFrames(frames)
	}
}

// printHeader prints the ASCII art header
func printHeader() {
	fmt.Println(`
		⠀⢀⣒⠒⠆⠤⣀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
		⢠⡛⠛⠻⣷⣶⣦⣬⣕⡒⠤⢀⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
		⡿⢿⣿⣿⣿⣿⣿⡿⠿⠿⣿⣳⠖⢋⣩⣭⣿⣶⡤⠶⠶⢶⣒⣲⢶⣉⣐⣒⣒⣒⢤⡀⠀⠀⠀⠀⠀⠀⠀
		⣿⠀⠉⣩⣭⣽⣶⣾⣿⢿⡏⢁⣴⠿⠛⠉⠁⠀⠀⠀⠀⠀⠀⠉⠙⠲⢭⣯⣟⡿⣷⣘⠢⡀⠀⠀⠀⠀⠀
		⠹⣷⣿⣿⣿⣿⣿⢟⣵⠋⢠⡾⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠻⣿⣿⣾⣦⣾⣢⠀⠀⠀⠀
		⠀⠹⣿⣿⣿⡿⣳⣿⠃⠀⣼⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⢻⣿⣿⣿⠟⠀⠀⠀⠀
		⠀⠀⠹⣿⣿⣵⣿⠃⠀⠀⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠹⣷⡄⠀⠀⠀⠀⠀
		⠀⠀⠀⠈⠛⣯⡇⠛⣽⣦⣿⠀⠀⠀⠀⢀⠔⠙⣄⠀⠀⠀⠀⠀⠀⣠⠳⡀⠀⠀⠀⠀⢿⡵⡀⠀⠀⠀⠀
		⠀⠀⠀⠀⣸⣿⣿⣿⠿⢿⠟⠀⠀⠀⢀⡏⠀⠀⠘⡄⠀⠀⠀⠀⢠⠃⠀⠹⡄⠀⠀⠀⠸⣿⣷⡀⠀⠀⠀
		⠀⠀⠀⢰⣿⣿⣿⣿⡀⠀⠀⠀⠀⠀⢸⠒⠤⢤⣀⣘⣆⠀⠀⠀⡏⢀⣀⡠⢷⠀⠀⠀⠀⣿⡿⠃⠀⠀⠀
		⠀⠀⠀⠸⣿⣿⠟⢹⣥⠀⠀⠀⠀⠀⣸⣀⣀⣤⣀⣀⠈⠳⢤⡀⡇⣀⣠⣄⣸⡆⠀⠀⠀⡏⠀⠀⠀⠀⠀
		⠀⠀⠀⠀⠁⠁⠀⢸⢟⡄⠀⠀⠀⠀⣿⣾⣿⣿⣿⣿⠁⠀⠈⠙⠙⣯⣿⣿⣿⡇⠀⠀⢠⠃⠀⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⠀⠀⠇⢨⢞⢆⠀⠀⠀⡿⣿⣿⣿⣿⡏⠀⠀⠀⠀⠀⣿⣿⣿⡿⡇⠀⣠⢟⡄⠀⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⠀⡼⠀⢈⡏⢎⠳⣄⠀⡇⠙⠛⠟⠛⠀⠀⠀⠀⠀⠀⠘⠻⠛⢱⢃⡜⡝⠈⠚⡄⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⠘⣅⠁⢸⣋⠈⢣⡈⢷⠇⠀⠀⠀⠀⠀⣄⠀⠀⢀⡄⠀⠀⣠⣼⢯⣴⠇⣀⡀⢸⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⠀⠈⠳⡌⠛⣶⣆⣷⣿⣦⣄⣀⠀⠀⠀⠈⠉⠉⢉⣀⣤⡞⢛⣄⡀⢀⡨⢗⡦⠎⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⠀⠀⠀⠈⠑⠪⣿⠁⠀⠐⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣏⠉⠁⢸⠀⠀⠀⠄⠙⡆⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⠀⠀⠀⣀⠤⠚⡉⢳⡄⠡⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣏⠁⣠⣧⣤⣄⣀⡀⡰⠁⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⢀⠔⠉⠀⠀⠀⠀⢀⣧⣠⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣅⡀⠀⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⢸⠆⠀⠀⠀⣀⣼⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠿⠟⠋⠁⣠⠖⠒⠒⠛⢿⣆⠀⠀⠀⠀
		⠀⠀⠀⠀⠀⠀⠑⠤⠴⠞⢋⣵⣿⢿⣿⣿⣿⣿⣿⣿⠗⣀⠀⠀⠀⠀⠀⢰⠇⠀⠀⠀⠀⢀⡼⣶⣤⠀⠀
		⠀⠀⠀⠀⠀⠀⠀⠀⠀⡠⠟⢛⣿⠀⠙⠲⠽⠛⠛⠵⠞⠉⠙⠳⢦⣀⣀⡞⠀⠀⠀⠀⡠⠋⠐⠣⠮⡁⠀
		⠀⠀⠀⠀⠀⠀⠀⢠⣎⡀⢀⣾⠇⢀⣠⡶⢶⠞⠋⠉⠉⠒⢄⡀⠉⠈⠉⠀⠀⠀⣠⣾⠀⠀⠀⠀⠀⢸⡀
		⠀⠀⠀⠀⠀⠀⠀⠘⣦⡀⠘⢁⡴⢟⣯⣞⢉⠀⠀⠀⠀⠀⠀⢹⠶⠤⠤⡤⢖⣿⡋⢇⠀⠀⠀⠀⠀⢸⠀
		⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⠵⠗⠺⠟⠖⢈⡣⡄⠀⠀⠀⠀⢀⣼⡤⣬⣽⠾⠋⠉⠑⠺⠧⣀⣤⣤⡠⠟⠃
		⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠛⠷⠶⠦⠶⠞⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀

███████╗██╗░░░██╗███╗░░░███╗░█████╗░███████╗██╗███╗░░██╗██████╗░███████╗██████╗░
██╔════╝██║░░░██║████╗░████║██╔══██╗██╔════╝██║████╗░██║██╔══██╗██╔════╝██╔══██╗
█████╗░░██║░░░██║██╔████╔██║██║░░██║█████╗░░██║██╔██╗██║██║░░██║█████╗░░██████╔╝
██╔══╝░░██║░░░██║██║╚██╔╝██║██║░░██║██╔══╝░░██║██║╚████║██║░░██║██╔══╝░░██╔══██╗
██║░░░░░╚██████╔╝██║░╚═╝░██║╚█████╔╝██║░░░░░██║██║░╚███║██████╔╝███████╗██║░░██║
╚═╝░░░░░░╚═════╝░╚═╝░░░░░╚═╝░╚════╝░╚═╝░░░░░╚═╝╚═╝░░╚══╝╚═════╝░╚══════╝╚═╝░░╚═╝ V187    
	`)
	fmt.Printf("FumoFinder - Version: %s, Commit: %s, Date: %s\n", version, commit, date)
	fmt.Println()
	fmt.Println("Anime Episode Finder - Powered by trace.moe & Fumos")
	fmt.Println("==================================================")
	fmt.Println("Starting episode identification process...")
	fmt.Println("==================================================")
}

// printConfig prints the parsed configuration settings in a readable format
func printConfig(cfg *config.Config) {
	fmt.Println("\nLoaded Configuration:")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Input Folder    : %s\n", cfg.InputFolder)
	fmt.Printf("FFmpeg Path     : %s\n", cfg.FfmpegPath)
	fmt.Printf("FFprobe Path    : %s\n", cfg.FfprobePath)
	fmt.Printf("Number of Frames: %d\n", cfg.NumFrames)
	fmt.Printf("API Endpoint    : %s\n", cfg.ApiEndpoint)
	if cfg.AniListID != 0 {
		fmt.Printf("AniList ID      : %d\n", cfg.AniListID)
	} else {
		fmt.Printf("AniList ID      : Not specified\n")
	}
	fmt.Printf("Threshold       : %.2f seconds\n", cfg.Threshold)
	fmt.Printf("Cleanup         : %t\n", !cfg.NoCleanup)
	fmt.Printf("Proxy File      : %s\n", cfg.ProxyFilePath)
	fmt.Println(strings.Repeat("=", 50))
}

// CleanupExtractedFrames deletes the extracted frames after the run
func cleanupExtractedFrames(frames []string) {
	fmt.Println("\nPerforming cleanup...")
	for _, frame := range frames {
		err := os.Remove(frame)
		if err != nil {
			log.Printf("Failed to delete frame %s: %v", frame, err)
		}
	}

	// Optionally, remove empty directories if all frames are purged
	removeEmptyDirs(filepath.Join("frames"))

	// Check if the frames directory is empty and remove it if it is
	if isDirEmpty("frames") {
		err := os.Remove("frames")
		if err != nil {
			log.Printf("Failed to delete frames directory: %v", err)
		} else {
			fmt.Println("Removed empty frames directory.")
		}
	} else {
		fmt.Println("Frames directory is not empty.")
	}
	fmt.Println("Cleaned up extracted frames.")
}

// RemoveEmptyDirs deletes empty directories left after frames are deleted
func removeEmptyDirs(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && path != root {
			os.Remove(path)
		}
		return nil
	})
}

// isDirEmpty checks if a directory is empty
func isDirEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdir(1)
	return err == io.EOF
}
