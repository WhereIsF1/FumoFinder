// internal/proxy/proxy_loader.go
package proxy

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ProxyLoader handles loading and validating proxies from a file
type ProxyLoader struct {
	proxyList []ProxyDetails // List of validated working proxies
	mu        sync.Mutex     // Mutex to safely update the proxy list
}

// ProxyDetails holds information about a proxy, including its URL and quota status.
type ProxyDetails struct {
	URL       *url.URL
	Quota     int // Quota is the maximum number of requests allowed by the proxy
	QuotaUsed int // QuotaUsed is the number of requests made using the proxy
}

// NewProxyLoader creates a new ProxyLoader
func NewProxyLoader() *ProxyLoader {
	return &ProxyLoader{}
}

// LoadProxies loads proxies from a given file path concurrently, supporting authentication
func (pl *ProxyLoader) LoadProxies(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open proxy file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var wg sync.WaitGroup
	results := make(chan *url.URL, 100) // Buffered channel to store results

	// Function to handle each proxy check in a separate goroutine
	checkProxyConcurrently := func(proxyStr string) {
		defer wg.Done()

		if !strings.HasPrefix(proxyStr, "http://") && !strings.HasPrefix(proxyStr, "https://") {
			proxyStr = "http://" + proxyStr
		}

		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			fmt.Printf("❌ Invalid proxy format: %s\n", proxyStr)
			return
		}

		// Check the proxy and retrieve its details
		isWorking, details := pl.checkProxy(proxyURL)
		if isWorking || details != nil {
			pl.mu.Lock()
			pl.proxyList = append(pl.proxyList, *details)
			pl.mu.Unlock()
			results <- details.URL
		} else {
			fmt.Printf("❌ Proxy not responding: %s\n", proxyStr)
		}
	}

	var total int
	// Scan each proxy and spawn a goroutine for checking
	for scanner.Scan() {
		proxyStr := strings.TrimSpace(scanner.Text())
		total++
		wg.Add(1)
		go checkProxyConcurrently(proxyStr)
	}

	// Close the results channel once all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results from the channel
	var valid int
	for range results {
		valid++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read proxy file: %v", err)
	}

	// Summary of proxy checks
	fmt.Printf("\n📝 Proxy Check Summary:\n")
	fmt.Printf("   - Checked: %d proxies\n", total)
	fmt.Printf("   - Working: %d proxies\n", valid)

	if valid == 0 {
		fmt.Println("⚠️ No working proxies found. Proceeding without proxy.")
	} else {
		fmt.Println("✅ Ready to use working proxies.")
	}

	return nil
}

// checkProxy tests the connectivity of a proxy URL and checks the quota status from the /me endpoint.
func (pl *ProxyLoader) checkProxy(proxyURL *url.URL) (bool, *ProxyDetails) {
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	// Set up authentication if the proxy URL contains username and password
	if proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		transport.ProxyConnectHeader = http.Header{
			"Proxy-Authorization": []string{auth},
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second, // Adjust the timeout if necessary
	}

	// Create a new request with headers
	req, err := http.NewRequest("GET", "https://api.trace.moe/me", nil)
	if err != nil {
		fmt.Printf("Request creation error: %v\n", err)
		return false, nil
	}

	// Set headers to mimic a browser request for better compatibility - not necessary for api.trace.moe but may be useful for other "APIs" that require it (soontm)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3")

	// Send the request and check the response
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Proxy error with %s: %v\n", proxyURL.String(), err)
		return false, nil
	}
	defer resp.Body.Close()

	// Check if the status code is OK
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Proxy responded with status code: %d for %s\n", resp.StatusCode, proxyURL.String())
		return false, nil
	}

	// Parse the JSON response from /me endpoint
	var result struct {
		ID          string `json:"id"`
		Priority    int    `json:"priority"`
		Concurrency int    `json:"concurrency"`
		Quota       int    `json:"quota"`
		QuotaUsed   int    `json:"quotaUsed"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Printf("Failed to parse /me endpoint response for %s: %v\n", proxyURL.String(), err)
		return false, nil
	}

	// Calculate the remaining quota
	remainingQuota := result.Quota - result.QuotaUsed

	// Create a ProxyDetails struct to store the proxy and its quota info
	proxyDetails := &ProxyDetails{
		URL:       proxyURL,
		Quota:     result.Quota,
		QuotaUsed: result.QuotaUsed,
	}

	if remainingQuota <= 0 {
		fmt.Printf("⚠️ Proxy %s has exceeded its quota. Remaining quota: 0. It will not be used but is flagged as working.\n", proxyURL.String())
		return false, proxyDetails
	}

	fmt.Printf("✅ Proxy is working: %s (Quota used: %d/%d, Remaining quota: %d)\n", proxyURL.String(), result.QuotaUsed, result.Quota, remainingQuota)
	return true, proxyDetails
}

// GetProxyList returns the list of validated working proxies as URLs.
func (pl *ProxyLoader) GetProxyList() []*url.URL {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	// Extract URLs from ProxyDetails
	var urlList []*url.URL
	for _, proxy := range pl.proxyList { // Iterate over the list of validated proxies
		urlList = append(urlList, proxy.URL)
	}

	return urlList // Return the list of proxy URLs
}
