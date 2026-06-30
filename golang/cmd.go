package main

import "github.com/spf13/cobra"

var (
	threads int
	nTries  int

	configPath string
	subnets    string

	Vpn      bool
	Loglevel string

	doUploadTest bool
	fronting     bool
	shuffle      bool

	minDLSpeed float64
	minULSpeed float64
	maxDLTime  float64
	maxULTime  float64

	frontingTimeout float64
	maxDLLatency    float64
	maxULLatency    float64

	// Skip configuration
	skipAfterMinutes   float64
	skipAfterSuccesses int

	bigIPList []string
)

func RegisterCommands(rootCmd *cobra.Command) {
	f := rootCmd.PersistentFlags()

	f.IntVarP(&threads, "threads", "t", 4, "Number of parallel scan threads")
	f.StringVarP(&configPath, "config", "c", "", "Path to config.json file (required)")
	f.BoolVar(&Vpn, "vpn", false, "Test via xray-core VLESS connections")
	f.StringVarP(&Loglevel, "loglevel", "l", "none", "Xray-core log level: debug|info|warning|error|none")
	f.StringVarP(&subnets, "subnets", "s", "", "Subnet file path, CIDR, or single IP")
	f.BoolVar(&shuffle, "shuffle", false, "Shuffle IPs before scanning")
	f.BoolVar(&doUploadTest, "upload", false, "Also run upload speed test")
	f.BoolVar(&fronting, "fronting", false, "Run domain-fronting connectivity test")
	f.IntVarP(&nTries, "tries", "n", 1, "Number of test attempts per IP (all must succeed)")

	f.Float64Var(&minDLSpeed, "download-speed", 50, "Minimum acceptable download speed (KB/s)")
	f.Float64Var(&minULSpeed, "upload-speed", 50, "Minimum acceptable upload speed (KB/s)")
	f.Float64Var(&maxDLTime, "download-time", 2, "Maximum effective download time (s)")
	f.Float64Var(&maxULTime, "upload-time", 2, "Maximum effective upload time (s)")
	f.Float64Var(&frontingTimeout, "fronting-timeout", 1.0, "Fronting test timeout (s)")
	f.Float64Var(&maxDLLatency, "download-latency", 3.0, "Maximum allowed download latency (s)")
	f.Float64Var(&maxULLatency, "upload-latency", 3.0, "Maximum allowed upload latency (s)")

	// Skip functionality
	f.Float64Var(&skipAfterMinutes, "skip-time", 0, "Skip subnet after N minutes (0 = disabled)")
	f.IntVar(&skipAfterSuccesses, "skip-count", 0, "Skip subnet after N successful IPs (0 = disabled)")
}
