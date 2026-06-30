package speedtest

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FrontingTest conducts a fronting test on an IP and returns true and the latency if HTTP 200 is received.
// It respects ctx cancellation.
func FrontingTest(ctx context.Context, ip string, proxies map[string]string, timeout time.Duration) (bool, int) {
	startTime := time.Now()
	var proxy *url.URL
	for _, v := range proxies {
		proxy, _ = url.Parse(v)
	}

	compatibleIP := ip
	if strings.Contains(ip, ":") {
		compatibleIP = fmt.Sprintf("[%s]", ip)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s", compatibleIP), nil)
	if err != nil {
		fmt.Printf("Error creating fronting request for %s: %v\n", ip, err)
		return false, 0
	}
	req.Host = "speed.cloudflare.com"

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{
				ServerName:         "speed.cloudflare.com",
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// Fail silently to keep the console clean
		return false, 0
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	latency := int(time.Since(startTime).Milliseconds())
	return resp.StatusCode == http.StatusOK, latency
}
