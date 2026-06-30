package vpn

import (
	configuration "CFScanner/configuration"
	"CFScanner/utils"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var validLogLevels = []string{"debug", "info", "warning", "error", "none"}

func loglevel(level string) string {
	for _, l := range validLogLevels {
		if strings.ToLower(level) == l {
			return l
		}
	}
	return "none"
}

func createInbound() []Inbound {
	localPort := utils.GetFreePort()
	ib := Inbound{
		Port:     localPort,
		Listen:   "127.0.0.1",
		Tag:      "socks-inbound",
		Protocol: "socks",
	}
	ib.Settings.Auth = "noauth"
	ib.Settings.UDP = false
	ib.Settings.IP = "127.0.0.1"
	ib.Sniffing.Enabled = true
	ib.Sniffing.DestOverride = []string{"http", "tls"}
	return []Inbound{ib}
}

func createOutbound(C *configuration.Configuration, IP string) []Outbound {
	port, err := strconv.Atoi(C.Config.AddressPort)
	if err != nil {
		log.Fatalf("Invalid port in config: %v", err)
	}

	user := User{
		ID:         C.Config.UserId,
		Encryption: "none",
	}

	vnext := VNext{
		Address: IP,
		Port:    port,
		Users:   []User{user},
	}

	// Build security + TLS settings
	security := "none"
	var tlsSettings *TLSSettings
	if C.Config.TLS {
		security = "tls"
		tlsSettings = &TLSSettings{
			AllowInsecure: false,
			ServerName:    C.Config.ServerName,
			ALPN:          []string{"h2", "http/1.1"},
			Fingerprint:   C.Config.Fingerprint,
		}
	}

	// Build transport-specific stream settings
	stream := buildStreamSettings(C, security, tlsSettings)

	ob := Outbound{
		Tag:            "proxy",
		Protocol:       "vless",
		Settings:       OutboundSettings{VNext: []VNext{vnext}},
		StreamSettings: stream,
	}

	return []Outbound{ob}
}

func buildStreamSettings(C *configuration.Configuration, security string, tls *TLSSettings) StreamSettings {
	ss := StreamSettings{
		Network:     C.Config.Transport,
		Security:    security,
		TLSSettings: tls,
	}

	switch C.Config.Transport {
	case configuration.TransportWS:
		ss.WSSettings = &WSSettings{
			Path: C.Config.Path,
			Headers: map[string]string{
				"Host": C.Config.Host,
			},
		}

	case configuration.TransportGRPC:
		// gRPC uses service name from path field; always TLS-only in practice
		ss.GRPCSettings = &GRPCSettings{
			ServiceName: strings.TrimPrefix(C.Config.Path, "/"),
			MultiMode:   false,
		}

	case configuration.TransportHTTPUpgrade:
		ss.HTTPUpgradeSettings = &HTTPUpgradeSettings{
			Path: C.Config.Path,
			Host: C.Config.Host,
		}

	case configuration.TransportXHTTP:
		ss.Network = "splithttp" // xray-core internal network name
		ss.XHTTPSettings = &XHTTPSettings{
			Host: C.Config.Host,
			Path: C.Config.Path,
			Mode: "packet-up", // required; empty string causes xray-core to reject the config
		}
	}

	return ss
}

// XRayConfig creates the per-IP xray JSON config file and returns its path.
func XRayConfig(IP string, testConfig *configuration.Configuration) string {
	config := XRay{
		Log: Log{
			Access:   "none",
			Error:    "none",
			Loglevel: loglevel(testConfig.LogLevel),
		},
		Inbounds:  createInbound(),
		Outbounds: createOutbound(testConfig, IP),
		Other:     struct{}{},
	}

	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Fatalf("Marshal error: %v", err)
	}

	configPath := fmt.Sprintf("%s/config-%s.json", configuration.DIR, IP)
	if err := writeJSONToFile(configBytes, configPath); err != nil {
		log.Fatalf("Failed to write xray config to file: %v", err)
	}

	return configPath
}

func writeJSONToFile(jsonBytes []byte, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(jsonBytes)
	return err
}
