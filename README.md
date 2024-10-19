

# FumoFinder

FumoFinder is an advanced anime episode identifier that extracts frames from videos and matches them using the trace.moe API. It distributes requests across multiple proxies to optimize performance and avoid rate limits.

You don’t need proxies, but you can use HTTP proxies (with or without authentication) for better load distribution.

FFmpeg & FFprobe must be installed as system variables or specified using the `--ffmpeg` and `--ffprobe` arguments.

### Proxy File Format
If using proxies, you can specify the proxy file with the `--proxy` argument. List each proxy on a new line in your `proxies.txt` file. Supported formats:
- Without authentication: `http://proxyserver:port`
- With authentication: `http://username:password@proxyserver:port`

### Proxy Checker
FumoFinder includes a built-in proxy checker that tests each proxy's ability to reach the trace.moe API. Non-working proxies are automatically dropped, allowing you to load a bulk freebie list if needed (not recommended, as free proxies often result in failed frame processing).

### AniList ID
An AniList ID can be specified to improve filtering and more accurately determine the episode numbers, especially for older anime, which may require a higher frame count due to possible imprecisions in the trace.moe database.

### Frame Extraction
FumoFinder allows you to extract frames from videos at specific intervals to match them with the trace.moe database. 
- It's recommended to extract **10 or more frames** per video for better accuracy. While you can select fewer frames, this may result in unreliable results.
- The first frame extracted skips the initial **10 seconds** of the video to avoid black or blank frames that often appear at the start.

### Bulk and Individual Renaming
FumoFinder now includes a **bulk renaming mode** that allows you to preview and confirm all file renames at once. If canceled, you can still go through the renaming process individually.

The renaming function uses the format `series.name.Exx`.

Example usage can be seen when running the tool with the `--help` command.

### Important Notes
- **Older Anime**: Results for older anime can be imprecise. Increasing the frame count and specifying an AniList ID can help improve accuracy.
- **Newly Aired Anime**: Very new anime (just aired) may be missing from the trace.moe database and therefore cannot be found.
- **Anime Specials**: Episodes labeled as specials (S00Exx) are not supported, as the trace.moe API cannot match frames to special episodes.

## License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.

