package speedtest

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// UploadSpeedTest conducts an upload speed test and returns (speed_mbps, latency_s, error).
// It respects ctx cancellation so it aborts immediately on quit.
func UploadSpeedTest(ctx context.Context, nBytes int, proxies map[string]string, timeout time.Duration) (float64, float64, error) {
	startTime := time.Now()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://speed.cloudflare.com/__up",
		strings.NewReader(strings.Repeat("0", nBytes)))
	if err != nil {
		return 0, 0, err
	}

	var proxy *url.URL
	for _, v := range proxies {
		proxy, _ = url.Parse(v)
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxy)},
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing upload response body: %v", err)
		}
	}()

	// Drain response body so connection can be reused
	_, _ = io.Copy(io.Discard, resp.Body)

	totalTime := time.Since(startTime).Seconds()
	cfTime := float64(0)

	// Server-Timing comes from the response, not the request
	serverTiming := resp.Header.Get("Server-Timing")
	if serverTiming != "" {
		timings := strings.Split(serverTiming, "=")
		if len(timings) > 1 {
			if cfTiming, err := strconv.ParseFloat(timings[1], 64); err == nil {
				cfTime = cfTiming / 1000.0
			}
		}
	}
	latency := totalTime - cfTime
	uploadSpeed := float64(nBytes) * 8 / (1000000.0 * latency)

	return uploadSpeed, latency, nil
}
