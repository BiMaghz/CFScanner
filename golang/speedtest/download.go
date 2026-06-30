package speedtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DownloadSpeedTest conducts a download speed test and returns (speed_mbps, latency_s, error).
// It respects ctx cancellation so it aborts immediately on quit.
func DownloadSpeedTest(ctx context.Context, nBytes int, proxies map[string]string, timeout time.Duration) (float64, float64, error) {
	startTime := time.Now()

	data := make([]byte, nBytes)

	var proxy *url.URL
	for _, v := range proxies {
		proxy, _ = url.Parse(v)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://speed.cloudflare.com/__down", nil)
	if err != nil {
		return 0, 0, fmt.Errorf("error creating request: %v", err)
	}

	q := req.URL.Query()
	q.Add("bytes", strconv.Itoa(nBytes))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxy)},
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	_, err = io.ReadFull(resp.Body, data)
	if err != nil {
		return 0, 0, fmt.Errorf("error reading response body: %v", err)
	}

	totalTime := time.Since(startTime).Seconds()
	cfTime := float64(0)
	serverTiming := resp.Header.Get("Server-Timing")
	if serverTiming != "" {
		timings := strings.Split(serverTiming, "=")
		if len(timings) > 1 {
			if cfTiming, err := strconv.ParseFloat(timings[1], 64); err == nil {
				cfTime = cfTiming / 1000.0
			}
		}
	}
	downloadTime := totalTime - cfTime
	downloadSpeed := float64(nBytes) * 8 / (downloadTime * 1000000)

	return downloadSpeed, downloadTime, nil
}
