package vpn

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"

	// Required for init() registrations in xray-core

	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/core"

	// Optional features
	_ "github.com/xtls/xray-core/app/dns"
	_ "github.com/xtls/xray-core/app/metrics"
	_ "github.com/xtls/xray-core/app/policy"
	_ "github.com/xtls/xray-core/app/router"
	_ "github.com/xtls/xray-core/app/stats"

	// Fix dependency cycle caused by core import in internet package
	_ "github.com/xtls/xray-core/transport/internet/tagged/taggedimpl"

	// Proxy protocols
	_ "github.com/xtls/xray-core/proxy/blackhole"
	_ "github.com/xtls/xray-core/proxy/dns"
	_ "github.com/xtls/xray-core/proxy/dokodemo"
	_ "github.com/xtls/xray-core/proxy/freedom"
	_ "github.com/xtls/xray-core/proxy/http"
	_ "github.com/xtls/xray-core/proxy/loopback"
	_ "github.com/xtls/xray-core/proxy/socks"
	_ "github.com/xtls/xray-core/proxy/vless/inbound"
	_ "github.com/xtls/xray-core/proxy/vless/outbound"
	_ "github.com/xtls/xray-core/proxy/vmess/inbound"
	_ "github.com/xtls/xray-core/proxy/vmess/outbound"

	// Transports
	_ "github.com/xtls/xray-core/transport/internet/grpc"
	_ "github.com/xtls/xray-core/transport/internet/httpupgrade"
	_ "github.com/xtls/xray-core/transport/internet/kcp"
	_ "github.com/xtls/xray-core/transport/internet/splithttp" // xhttp
	_ "github.com/xtls/xray-core/transport/internet/tcp"
	_ "github.com/xtls/xray-core/transport/internet/tls"
	_ "github.com/xtls/xray-core/transport/internet/udp"
	_ "github.com/xtls/xray-core/transport/internet/websocket"

	// Transport headers
	_ "github.com/xtls/xray-core/transport/internet/headers/http"
	_ "github.com/xtls/xray-core/transport/internet/headers/noop"

	// Config format loaders
	_ "github.com/xtls/xray-core/main/confloader/external"
	_ "github.com/xtls/xray-core/main/json"
)

// XRayInstance starts an xray-core instance from a config file and returns the worker.
func XRayInstance(configPath string, logLevel string) ScanWorker {
	config, err := LoadConfig(configPath, logLevel)
	if err != nil {
		log.Fatal(err)
	}

	instance, err := core.New(config)
	if err != nil {
		log.Fatal(err)
	}

	if err := instance.Start(); err != nil {
		log.Fatal("Failed to start XRay instance:", err)
	}

	go func() {
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt)
		<-osSignals
	}()

	return ScanWorker{Instance: instance}
}

// ProxyBind returns a proxy map for the given SOCKS5 listen address and port.
func ProxyBind(listen string, port int) map[string]string {
	addr := fmt.Sprintf("socks5://%s:%d", listen, port)
	return map[string]string{
		"http":  addr,
		"https": addr,
	}
}

// XRayVersion prints the xray-core version string.
func XRayVersion() {
	fmt.Println(core.VersionStatement())
}

// XRayReceiver reads the inbound listen/port from a generated xray JSON config file.
func XRayReceiver(configPath string) (string, int, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	var xrayConf map[string]interface{}
	if err := json.NewDecoder(f).Decode(&xrayConf); err != nil {
		return "", 0, err
	}

	inbounds, ok := xrayConf["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		return "", 0, fmt.Errorf("no inbounds in xray config")
	}
	first := inbounds[0].(map[string]interface{})
	listenAddr := first["listen"].(string)
	portVal := int(first["port"].(float64))

	return listenAddr, portVal, nil
}
