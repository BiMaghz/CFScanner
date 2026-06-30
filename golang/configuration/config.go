package config

import (
	"CFScanner/utils"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	PROGRAMDIR = func() string {
		exe, err := os.Executable()
		if err != nil {
			// fall back to args[0] dir
			p, _ := filepath.Abs(filepath.Dir(os.Args[0]))
			return p
		}
		return filepath.Dir(exe)
	}()
	DIR                    = filepath.Join(PROGRAMDIR, "config")
	RESULTDIR              = filepath.Join(PROGRAMDIR, "result")
	StartDtStr             = time.Now().Format("2006-01-02_15-04-05")
	ResultsPath            = filepath.Join(RESULTDIR, StartDtStr+"_result.txt")
	FinalResultsPathSorted = filepath.Join(RESULTDIR, StartDtStr+"_final.txt")
)

// rawConfig is the on-disk format of config.real
type rawConfig struct {
	ID          string `json:"id"`
	Host        string `json:"host"`
	Port        string `json:"port"`
	Path        string `json:"path"`
	ServerName  string `json:"serverName"`
	Transport   string `json:"transport"`
	TLS         bool   `json:"tls"`
	Fingerprint string `json:"fingerprint"`
	SubnetsList string `json:"subnetsList"`
}

func (C Configuration) PrintInformation() {
	security := "none"
	if C.Config.TLS {
		security = "tls"
	}
	fmt.Printf(`-------------------------------------
Configuration:
  User ID       : %v%v%v
  Host          : %v%v%v
  Path          : %v%v%v
  Server Name   : %v%v%v
  Port          : %v%v%v
  Transport     : %v%v%v
  Security      : %v%v%v
  Fingerprint   : %v%v%v
  Upload Test   : %v%v%v
  Fronting Test : %v%v%v
  Min DL Speed  : %v%v%v KB/s
  Max DL Time   : %v%v%v s
  Min UL Speed  : %v%v%v KB/s
  Max UL Time   : %v%v%v s
  Fronting TO   : %v%v%v s
  Max DL Latency: %v%v%v s
  Max UL Latency: %v%v%v s
  Tries         : %v%v%v
  Xray-core     : %v%v%v
  Xray loglevel : %v%v%v
  Shuffle       : %v%v%v
  Threads       : %v%v%v
-------------------------------------
`,
		utils.Colors.OKBLUE, C.Config.UserId, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.Host, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.Path, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.ServerName, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.AddressPort, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.Transport, utils.Colors.ENDC,
		utils.Colors.OKBLUE, security, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.Fingerprint, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.DoUploadTest, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.DoFrontingTest, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Download.MinDlSpeed, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Download.MaxDlTime, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Upload.MinUlSpeed, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Upload.MaxUlTime, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.FrontingTimeout, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Download.MaxDlLatency, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Upload.MaxUlLatency, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Config.NTries, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Vpn, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.LogLevel, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Shuffling, utils.Colors.ENDC,
		utils.Colors.OKBLUE, C.Worker.Threads, utils.Colors.ENDC,
	)
}

func (C Configuration) CreateTestConfig(configPath string) Configuration {
	if configPath == "" {
		// Fallback to config.json in the executable directory
		configPath = filepath.Join(DIR, "config.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			log.Fatalf("Configuration file not provided and default 'config.json' not found. Use --config / -c flag.")
		}
	}

	jsonFile, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Error opening config file: %v", err)
	}
	defer func() {
		if cerr := jsonFile.Close(); cerr != nil {
			log.Printf("Warning: failed to close config file: %v", cerr)
		}
	}()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	var raw rawConfig
	if err := json.Unmarshal(byteValue, &raw); err != nil {
		log.Fatalf("Error parsing config JSON: %v", err)
	}

	// Validate required fields
	if raw.ID == "" {
		log.Fatal("Config error: 'id' field is required")
	}
	if raw.Port == "" {
		log.Fatal("Config error: 'port' field is required")
	}
	if raw.Transport == "" {
		raw.Transport = TransportWS // default
	}

	transport := raw.Transport
	switch transport {
	case TransportWS, TransportGRPC, TransportHTTPUpgrade, TransportXHTTP:
		// valid
	default:
		log.Fatalf("Config error: unsupported transport %q. Valid: ws, grpc, httpupgrade, xhttp", transport)
	}

	if transport == TransportGRPC && !raw.TLS {
		log.Printf("Warning: gRPC is strongly recommended to be used with TLS. Proceeding anyway.")
	}

	C.Config.UserId = raw.ID
	C.Config.Host = raw.Host
	C.Config.AddressPort = raw.Port
	C.Config.Path = raw.Path
	C.Config.ServerName = raw.ServerName
	C.Config.Transport = transport
	C.Config.TLS = raw.TLS
	C.Config.Fingerprint = raw.Fingerprint
	if C.Config.Fingerprint == "" {
		C.Config.Fingerprint = "chrome"
	}
	C.Config.SubnetsList = raw.SubnetsList

	C.PrintInformation()
	return C
}
