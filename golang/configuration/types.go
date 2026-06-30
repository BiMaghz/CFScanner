package config

// Transport constants
const (
	TransportWS          = "ws"
	TransportGRPC        = "grpc"
	TransportHTTPUpgrade = "httpupgrade"
	TransportXHTTP       = "xhttp"
)

type Configuration struct {
	Config    ConfigStruct
	Worker    Worker
	Shuffling bool
	LogLevel  string
}

type Worker struct {
	Download Download
	Upload   Upload
	Threads  int
	Vpn      bool
}

type ConfigStruct struct {
	// Core VPN settings
	LocalPort   int
	AddressPort string
	UserId      string
	ServerName  string

	// Transport settings
	Transport string // ws | grpc | httpupgrade | xhttp
	TLS       bool

	// WS / HTTPUpgrade / xhttp specific
	Path string
	Host string

	// TLS fingerprint
	Fingerprint string

	// Subnets from config file (fallback when --subnets is not set)
	SubnetsList string

	// Timing / test config
	FrontingTimeout float64 // seconds
	NTries          int

	TestBool
}

type TestBool struct {
	DoUploadTest   bool
	DoFrontingTest bool
}

type Download struct {
	MinDlSpeed   float64 // kilobytes per second
	MaxDlTime    float64 // seconds
	MaxDlLatency float64 // seconds
}

type Upload struct {
	MinUlSpeed   float64 // kilobytes per second
	MaxUlTime    float64 // seconds
	MaxUlLatency float64 // seconds
}
