package main

import (
	configuration "CFScanner/configuration"
	"CFScanner/scanner"
	"CFScanner/utils"
	"bufio"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

func run() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     os.Args[0],
		Short:   codename,
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(VersionStatement())

			// Create required directories
			if Vpn {
				utils.CreateDir(configuration.DIR)
			}
			utils.CreateDir(configuration.RESULTDIR)

			// Initialize results file
			if err := scanner.InitResultsFile(configuration.ResultsPath); err != nil {
				log.Fatalf("Error creating results file: %v", err)
			}

			// Build base configuration struct (CLI flags only — no config file yet)
			cfg := configuration.Configuration{
				Config: configuration.ConfigStruct{
					FrontingTimeout: frontingTimeout,
					NTries:          nTries,
					TestBool: configuration.TestBool{
						DoUploadTest:   doUploadTest,
						DoFrontingTest: fronting,
					},
				},
				Worker: configuration.Worker{
					Threads: threads,
					Vpn:     Vpn,
					Download: configuration.Download{
						MinDlSpeed:   minDLSpeed,
						MaxDlTime:    maxDLTime,
						MaxDlLatency: maxDLLatency,
					},
					Upload: configuration.Upload{
						MinUlSpeed:   minULSpeed,
						MaxUlTime:    maxULTime,
						MaxUlLatency: maxULLatency,
					},
				},
				Shuffling: shuffle,
				LogLevel:  Loglevel,
			}

			// Load config file — this populates transport, ID, SubnetsList, etc.
			cfg = cfg.CreateTestConfig(configPath)

			// Resolve subnet source: CLI flag takes priority, then config file's subnetsList
			subnetSource := subnets
			if subnetSource == "" {
				subnetSource = cfg.Config.SubnetsList
			}

			if subnetSource == "" {
				log.Fatal("No subnet source provided. Use --subnets or set 'subnetsList' in config.")
			}

			// Load IP list
			rawList := loadSubnets(subnetSource)

			// Parse and expand CIDRs → individual IPs
			bigIPList = utils.IPParser(rawList)

			if len(bigIPList) == 0 {
				log.Fatal("No IPs parsed from input. Check --subnets or 'subnetsList' in config.")
			}

			// Optional shuffle
			if shuffle {
				rand.Shuffle(len(bigIPList), func(i, j int) {
					bigIPList[i], bigIPList[j] = bigIPList[j], bigIPList[i]
				})
			}

			fmt.Printf("Scanning %v%d%v IPs...\n\n",
				utils.Colors.OKGREEN, len(bigIPList), utils.Colors.ENDC)

			skipCfg := scanner.SkipConfig{
				AfterMinutes:   skipAfterMinutes,
				AfterSuccesses: skipAfterSuccesses,
			}

			start := time.Now()

			scanner.Start(cfg, cfg.Worker, bigIPList, threads, skipCfg,
				configuration.ResultsPath, configuration.FinalResultsPathSorted, start)

			fmt.Println("\n-------------------------------------")
			fmt.Println("Results     :", configuration.ResultsPath)
			fmt.Println("Sorted IPs  :", configuration.FinalResultsPathSorted)
			fmt.Println("Elapsed     :", time.Since(start).Round(time.Second))
		},
	}
	return rootCmd
}

// loadSubnets reads IPs/CIDRs from a file path, a URL, or a single CIDR/IP string.
func loadSubnets(source string) []string {
	var list []string

	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return loadSubnetsFromURL(source)
	}

	// Check if it's a file
	isFile, _ := utils.Exists(source)
	if isFile {
		f, err := os.Open(source)
		if err != nil {
			log.Fatalf("Failed to open subnet file: %v", err)
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				list = append(list, line)
			}
		}
		if err := sc.Err(); err != nil {
			log.Fatalf("Error reading subnet file: %v", err)
		}
		return list
	}

	// Treat as a literal CIDR or IP
	if source != "" {
		list = append(list, source)
	}
	return list
}

// loadSubnetsFromURL fetches a newline-separated list of CIDRs/IPs from a URL.
func loadSubnetsFromURL(url string) []string {
	fmt.Printf("Fetching subnet list from: %s\n", url)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch subnet list from URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failed to fetch subnet list: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read subnet list response: %v", err)
	}

	var list []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			list = append(list, line)
		}
	}

	fmt.Printf("Loaded %d subnet entries.\n\n", len(list))
	return list
}
