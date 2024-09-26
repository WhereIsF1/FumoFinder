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
An AniList ID can be specified to improve filtering and more accurately determine the episode numbers.

### File Renaming
Currently, the rename function appends `_EPxx` to the filename; custom naming might be implemented in the future (maybe or maybe not, lol).

Example usage can be seen when running the tool with the `--help` command.
