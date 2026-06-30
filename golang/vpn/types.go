package vpn

import (
	"github.com/xtls/xray-core/core"
)

// ScanWorker holds a running xray-core instance
type ScanWorker struct {
	Instance *core.Instance
}

// ---- Shared xray config structures ----

type Log struct {
	Access   string `json:"access"`
	Error    string `json:"error"`
	Loglevel string `json:"loglevel"`
}

type Inbound struct {
	Port     int    `json:"port"`
	Listen   string `json:"listen"`
	Tag      string `json:"tag"`
	Protocol string `json:"protocol"`
	Settings struct {
		Auth string `json:"auth"`
		UDP  bool   `json:"udp"`
		IP   string `json:"ip"`
	} `json:"settings"`
	Sniffing struct {
		Enabled      bool     `json:"enabled"`
		DestOverride []string `json:"destOverride"`
	} `json:"sniffing"`
}

type User struct {
	ID         string `json:"id"`
	Encryption string `json:"encryption,omitempty"` // for vless: "none"
}

type VNext struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Users   []User `json:"users"`
}

// ---- TLS Settings ----

type TLSSettings struct {
	AllowInsecure bool     `json:"allowInsecure"`
	ServerName    string   `json:"serverName"`
	ALPN          []string `json:"alpn"`
	Fingerprint   string   `json:"fingerprint"`
}

// ---- Transport-specific settings ----

type WSSettings struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
}

type GRPCSettings struct {
	ServiceName string `json:"serviceName"`
	MultiMode   bool   `json:"multiMode"`
}

type HTTPUpgradeSettings struct {
	Path    string            `json:"path"`
	Host    string            `json:"host"`
	Headers map[string]string `json:"headers,omitempty"`
}

type XHTTPSettings struct {
	Host string `json:"host"`
	Path string `json:"path"`
	Mode string `json:"mode,omitempty"`
}

// StreamSettings holds network + security + per-transport config.
// We use json.RawMessage-based approach via a generic map for clean omitempty.
type StreamSettings struct {
	Network             string               `json:"network"`
	Security            string               `json:"security"`
	TLSSettings         *TLSSettings         `json:"tlsSettings,omitempty"`
	WSSettings          *WSSettings          `json:"wsSettings,omitempty"`
	GRPCSettings        *GRPCSettings        `json:"grpcSettings,omitempty"`
	HTTPUpgradeSettings *HTTPUpgradeSettings `json:"httpupgradeSettings,omitempty"`
	XHTTPSettings       *XHTTPSettings       `json:"xhttpSettings,omitempty"`
}

// ---- Outbound (protocol: vless) ----

type OutboundSettings struct {
	VNext []VNext `json:"vnext"`
}

type Outbound struct {
	Tag            string           `json:"tag,omitempty"`
	Protocol       string           `json:"protocol"`
	Settings       OutboundSettings `json:"settings"`
	StreamSettings StreamSettings   `json:"streamSettings"`
}

// ---- Root xray config ----

type XRay struct {
	Log       Log        `json:"log"`
	Inbounds  []Inbound  `json:"inbounds"`
	Outbounds []Outbound `json:"outbounds"`
	Other     struct{}   `json:"other"`
}
