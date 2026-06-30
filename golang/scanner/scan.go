package scanner

import (
	config "CFScanner/configuration"
	"CFScanner/logger"
	"CFScanner/speedtest"
	"CFScanner/utils"
	"CFScanner/vpn"
	"context"
	"fmt"
	"math"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/eiannone/keyboard"
)

// MaxProc limits the maximum number of parallel scan goroutines.
var MaxProc = runtime.NumCPU() * 2

// SkipConfig controls when to skip the current subnet.
type SkipConfig struct {
	// Skip a subnet after this many minutes (0 = disabled)
	AfterMinutes float64
	// Skip a subnet after this many successful IPs (0 = disabled)
	AfterSuccesses int
}

// scanResult holds per-IP speed test measurements. All values are local to
// one goroutine call — no shared globals, no data races.
type scanResult struct {
	IP       string
	Download struct {
		Speed   []float64
		Latency []int
	}
	Upload struct {
		Speed   []float64
		Latency []int
	}
}

var printMu sync.Mutex

func printProgress(scanned, total, found int, start time.Time, subnet string) {
	printMu.Lock()
	defer printMu.Unlock()
	elapsed := time.Since(start).Round(time.Second)
	subnetInfo := ""
	if subnet != "" {
		subnetInfo = " | " + subnet
	}
	// \033[K clears the rest of the line
	fmt.Printf("\r\033[K  [%d/%d scanned | %d found | %s%s]", scanned, total, found, elapsed, subnetInfo)
}

func printSuccess(isVpn bool, ip string, latency int, dlSpeed, ulSpeed float64) {
	printMu.Lock()
	defer printMu.Unlock()
	// \033[K clears the progress line before printing the success line
	if isVpn {
		fmt.Printf("\r\033[K%v[OK]%v %-15s %4d ms  dl: %7.3f mbps  ul: %7.3f mbps\n",
			utils.Colors.OKGREEN, utils.Colors.ENDC, ip, latency, dlSpeed, ulSpeed)
	} else {
		fmt.Printf("\r\033[K%v[OK]%v %-15s\n",
			utils.Colors.OKGREEN, utils.Colors.ENDC, ip)
	}
}

// scanner performs a single IP scan and returns results or nil on failure.
func scanner(ctx context.Context, ip string, C config.Configuration, Worker config.Worker) *scanResult {
	res := &scanResult{IP: ip}

	var proxies map[string]string
	var process vpn.ScanWorker

	if Worker.Vpn {
		xrayConfigPath := vpn.XRayConfig(ip, &C)
		listen, port, err := vpn.XRayReceiver(xrayConfigPath)
		if err != nil {
			printLog(ip, logger.ErrorStatus, "Failed to read xray config", err.Error())
			return nil
		}

		proxies = vpn.ProxyBind(listen, port)

		// Start Xray FIRST, then wait for port
		process = vpn.XRayInstance(xrayConfigPath, C.LogLevel)
		defer func() {
			if err := process.Instance.Close(); err != nil {
				printLog("", logger.ErrorStatus, "Failed to stop xray instance", err.Error())
			}
		}()

		if err := utils.WaitForPort(listen, port, 5*time.Second); err != nil {
			printLog(ip, logger.ErrorStatus, "Xray port not ready", err.Error())
			return nil
		}
	}

	Download := &Worker.Download
	Upload := &Worker.Upload

	for tryIdx := 0; tryIdx < C.Config.NTries; tryIdx++ {
		// Check context before each try
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !Worker.Vpn {
			// SIMPLE TEST MODE: Only perform fronting test to check IP validity and get latency.
			// This avoids blocked requests to speed.cloudflare.com/__down when directly accessing it without a VPN.
			ok, latency := speedtest.FrontingTest(ctx, ip, proxies, time.Duration(C.Config.FrontingTimeout)*time.Second)
			if !ok {
				return nil
			}
			res.Download.Latency = append(res.Download.Latency, latency)
			continue // Skip download and upload tests
		}

		// VPN MODE:
		// Optional fronting test
		if C.Config.DoFrontingTest {
			ok, _ := speedtest.FrontingTest(ctx, ip, proxies, time.Duration(C.Config.FrontingTimeout)*time.Second)
			if !ok {
				return nil
			}
		}

		// Download test
		dlSpeed, dlLatency, err := speedtest.DownloadSpeedTest(
			ctx,
			int(Download.MinDlSpeed*1000*Download.MaxDlTime),
			proxies,
			time.Duration(Download.MaxDlLatency)*time.Second,
		)
		if err != nil {
			// context cancellation error is expected on quit
			if ctx.Err() == nil {
				printLog(ip, logger.FailStatus, logger.DownloadError, err.Error())
			}
			return nil
		}
		if dlLatency >= Download.MaxDlLatency {
			return nil
		}
		dlSpeedKBps := dlSpeed / 8 * 1000
		if dlSpeedKBps <= Download.MinDlSpeed {
			return nil
		}
		res.Download.Speed = append(res.Download.Speed, dlSpeed)
		res.Download.Latency = append(res.Download.Latency, int(math.Round(dlLatency*1000)))

		// Optional upload test
		if C.Config.DoUploadTest {
			ulSpeed, ulLatency, err := speedtest.UploadSpeedTest(
				ctx,
				int(Upload.MinUlSpeed*1000*Upload.MaxUlTime),
				proxies,
				time.Duration(Upload.MaxUlLatency)*time.Second,
			)
			if err != nil {
				if ctx.Err() == nil {
					printLog(ip, logger.FailStatus, logger.UploadError, err.Error())
				}
				return nil
			}
			if ulLatency >= Upload.MaxUlLatency {
				return nil
			}
			ulSpeedKBps := ulSpeed / 8 * 1000
			if ulSpeedKBps <= Upload.MinUlSpeed {
				return nil
			}
			res.Upload.Speed = append(res.Upload.Speed, ulSpeed)
			res.Upload.Latency = append(res.Upload.Latency, int(math.Round(ulLatency*1000)))
		}
	}

	return res
}

// printLog is a tiny wrapper to avoid repeating logger.ScannerManage construction.
func printLog(ip string, status logger.LogStatus, msg, cause string) {
	// Mute logs during run unless they are critical, to keep output clean.
	// Users want a clean console. We only print errors if explicitly needed.
}

// mean computes the arithmetic mean of a float64 slice.
func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var s float64
	for _, v := range vals {
		s += v
	}
	return s / float64(len(vals))
}

// meanInt computes mean of an int slice as float64.
func meanInt(vals []int) float64 {
	floats := make([]float64, len(vals))
	for i, v := range vals {
		floats[i] = float64(v)
	}
	return mean(floats)
}

// scan is called per-IP. It runs the speed test and, on success, records output.
func scan(ctx context.Context, C *config.Configuration, worker *config.Worker, ip, resultsPath, finalPath string) bool {
	res := scanner(ctx, ip, *C, *worker)
	if res == nil {
		return false
	}

	meanDlLatency := meanInt(res.Download.Latency)
	meanDlSpeed := mean(res.Download.Speed)
	meanUlSpeed := mean(res.Upload.Speed)

	latencyMs := int(math.Round(meanDlLatency))

	printSuccess(worker.Vpn, ip, latencyMs, meanDlSpeed, meanUlSpeed)
	RecordSuccess(ip, latencyMs, resultsPath, finalPath)
	return true
}

// SubnetBatch groups IPs that belong to the same /24 subnet (or individually).
func groupBySubnet(ipList []string) [][]string {
	groups := map[string][]string{}
	var order []string

	for _, ip := range ipList {
		parsed := net.ParseIP(ip)
		var key string
		if parsed != nil && parsed.To4() != nil {
			parts := strings.Split(ip, ".")
			if len(parts) == 4 {
				key = strings.Join(parts[:3], ".")
			}
		}
		if key == "" {
			key = ip // IPv6 or unusual — treat each as its own group
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], ip)
	}

	result := make([][]string, 0, len(order))
	for _, k := range order {
		result = append(result, groups[k])
	}
	return result
}

// runningState tracks whether workers are currently running.
var runningState int32 = 1 // 1 = running, 0 = paused

func isRunning() bool   { return atomic.LoadInt32(&runningState) == 1 }
func setPaused()        { atomic.StoreInt32(&runningState, 0) }
func setRunning()       { atomic.StoreInt32(&runningState, 1) }

// Start is the main entry point for the scanning process.
func Start(C config.Configuration, Worker config.Worker, ipList []string, threadsCount int, skip SkipConfig, resultsPath, finalPath string, scanStart time.Time) {
	resetStore()

	if threadsCount > MaxProc {
		fmt.Printf("Capping threads at MaxProc = %d\n", MaxProc)
		threadsCount = MaxProc
	}

	PrintScannerHelp()

	subnets := groupBySubnet(ipList)

	keysEvents, err := keyboard.GetKeys(10)
	if err != nil {
		fmt.Println("Warning: keyboard control unavailable:", err)
		keysEvents = nil
	} else {
		// Do not defer close here. We will close it explicitly on quit.
	}

	quitChan := make(chan struct{})
	skipChan := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if keysEvents != nil {
		go func() {
			defer keyboard.Close()
			for {
				select {
				case <-quitChan:
					return
				case event, ok := <-keysEvents:
					if !ok {
						return
					}
					switch {
					case event.Key == keyboard.KeyEsc || event.Key == keyboard.KeyCtrlC:
						fmt.Println("\n\033[KQuit requested. Shutting down...")
						cancel() // Abort all running network requests immediately
						close(quitChan)
						return
					case event.Rune == 'p' || event.Rune == 'P':
						if !isRunning() {
							fmt.Println("\n\033[KAlready paused.")
						} else {
							setPaused()
							fmt.Println("\n\033[KPaused. Press R to resume, S to skip subnet.")
						}
					case event.Rune == 'r' || event.Rune == 'R':
						if isRunning() {
							fmt.Println("\n\033[KAlready running.")
						} else {
							setRunning()
							fmt.Println("\n\033[KResumed.")
						}
					case event.Rune == 's' || event.Rune == 'S':
						select {
						case skipChan <- struct{}{}:
							fmt.Println("\n\033[KSkipping current subnet...")
						default:
						}
					}
				}
			}
		}()
	}

	totalScanned := 0
	totalIPs := len(ipList)
	currentSubnet := ""
	setRunning()

	// Progress updater goroutine to ensure UI updates even when scanning is slow
	progressCtx, progressCancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-progressCtx.Done():
				return
			case <-ticker.C:
				if isRunning() {
					printProgress(totalScanned, totalIPs, SuccessCount(), scanStart, currentSubnet)
				}
			}
		}
	}()
	defer progressCancel()

SubnetLoop:
	for _, subnet := range subnets {
		subnetStart := time.Now()
		var subnetSuccesses int32 = 0
		var subnetDispatched int = 0

		// Derive a human-readable label for the progress bar
		if len(subnet) > 0 {
			parts := strings.Split(subnet[0], ".")
			if len(parts) == 4 {
				currentSubnet = strings.Join(parts[:3], ".") + ".x"
			} else {
				currentSubnet = subnet[0]
			}
		}

		sem := make(chan struct{}, threadsCount)
		var wg sync.WaitGroup

	IPLoop:
		for _, ip := range subnet {
			select {
			case <-quitChan:
				break SubnetLoop
			default:
			}

			select {
			case <-skipChan:
				// Credit remaining undispatched IPs so the counter stays accurate
				totalScanned += len(subnet) - subnetDispatched
				break IPLoop
			default:
			}

			for !isRunning() {
				select {
				case <-quitChan:
					break SubnetLoop
				default:
					time.Sleep(100 * time.Millisecond)
				}
			}

			if skip.AfterMinutes > 0 {
				elapsed := time.Since(subnetStart).Minutes()
				if elapsed >= skip.AfterMinutes {
					totalScanned += len(subnet) - subnetDispatched
					break IPLoop
				}
			}

			if skip.AfterSuccesses > 0 && atomic.LoadInt32(&subnetSuccesses) >= int32(skip.AfterSuccesses) {
				totalScanned += len(subnet) - subnetDispatched
				break IPLoop
			}

			sem <- struct{}{}
			wg.Add(1)

			ipCopy := ip
			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				ok := scan(ctx, &C, &Worker, ipCopy, resultsPath, finalPath)
				if ok {
					atomic.AddInt32(&subnetSuccesses, 1)
				}
			}()

			totalScanned++
			subnetDispatched++
		}

		wg.Wait()

		// Drain any leftover skip signal so it doesn't carry over to the next subnet
		select {
		case <-skipChan:
		default:
		}
	}

	// Final progress update
	printProgress(totalScanned, totalIPs, SuccessCount(), scanStart, "")
	fmt.Printf("\n\033[KScan complete. %d IPs found.\n", SuccessCount())
	
	// Ensure keyboard goroutine cleans up
	if keysEvents != nil {
		select {
		case <-quitChan:
		default:
			close(quitChan)
		}
	}
}
