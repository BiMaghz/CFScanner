package vpn

import (
	"log"
	"os"
	"strings"

	"github.com/xtls/xray-core/core"
	xraylog "github.com/xtls/xray-core/common/log"
)

// noOpHandler silences all xray logs during config parsing to suppress
// deprecation warnings emitted before the user's log level takes effect.
type noOpHandler struct{}

func (h *noOpHandler) Handle(msg xraylog.Message) {}

// severityHandler filters xray logs by severity and writes them to stderr.
type severityHandler struct {
	minLevel xraylog.Severity
}

func (h *severityHandler) Handle(msg xraylog.Message) {
	gm, ok := msg.(*xraylog.GeneralMessage)
	if !ok || gm.Severity > h.minLevel {
		return
	}
	log.Print(msg.String())
}

// parseSeverity converts a string log level name to an xray Severity.
func parseSeverity(level string) xraylog.Severity {
	switch strings.ToLower(level) {
	case "debug":
		return xraylog.Severity_Debug
	case "info":
		return xraylog.Severity_Info
	case "warning":
		return xraylog.Severity_Warning
	case "error":
		return xraylog.Severity_Error
	default:
		return xraylog.Severity_Unknown // will match nothing → silent
	}
}

// LoadConfig parses an xray JSON config file.
// It suppresses all logs during parsing (to silence deprecation warnings),
// then restores a severity-filtered logger based on the user's chosen log level.
func LoadConfig(filename string, logLevel string) (*core.Config, error) {
	// Silence xray's default stdout logger during JSON parsing
	xraylog.RegisterHandler(&noOpHandler{})

	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config, err := core.LoadConfig("json", file)
	if err != nil {
		return nil, err
	}

	// Restore a real logger now that parsing is done, respecting the user's -l flag.
	// "none" (default) keeps the noOpHandler — no runtime xray logs at all.
	if logLevel != "" && strings.ToLower(logLevel) != "none" {
		xraylog.RegisterHandler(&severityHandler{minLevel: parseSeverity(logLevel)})
	}

	return config, nil
}
