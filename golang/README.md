# CFScanner GoLang

![go]

CFScanner is a powerful tool written in Golang specifically designed to scan Cloudflare's edge IPs and identify viable options for use with V2Ray/Xray.

Its main objective is to locate edge IPs that are accessible and not blocked. With its built-in xray-core, CFScanner leverages xray+vmess+websocket+tls by default when the VPN flag is enabled.

If you prefer to use it behind your Cloudflare proxy, you will need to set up a vmess account. However, if no specific configuration is provided, the program will automatically use the default settings.
# Requirements

- Golang v1.20+

# Installation

### Getting the latest version from release page
Latest release version of golang CFScanner are available in [releases](https://github.com/BiMaghz/CFScanner/releases)
section 


### Build instructions

If you prefer to build CFScanner from source, you can follow these instructions:

Clone the repository by running the following command in your terminal:
```bash
git clone https://github.com/BiMaghz/CFScanner.git
```
Navigate to the "golang" directory within the cloned repository:

```bash
cd CFScanner/golang
```

Build the binary using the "go build" command with additional flags for trimming the path and setting linker flags for smaller binary size:
```bash
go build -o CFScanner -trimpath -ldflags "-s -w -buildid=" .
```

## Configuration

CFScanner requires a JSON config file that describes your VLESS server.

### Quick start
```bash
cp config.json.example config.json
# Edit config.json with your own values
```

### `config.json` format

```json
{
  "id": "YOUR_UUID",
  "host": "your-domain.com",
  "port": "443",
  "path": "/api",
  "serverName": "your-domain.com",
  "transport": "ws",
  "tls": true,
  "fingerprint": "chrome",
  "subnetsList": "https://raw.githubusercontent.com/MortezaBashsiz/CFScanner/main/bash/cf.local.iplist"
}
```

| Field         | Description                                                                                  |
|---------------|----------------------------------------------------------------------------------------------|
| `id`          | Your VLESS UUID.                                                                             |
| `host`        | The `Host` header / CDN domain.                                                              |
| `port`        | Destination port (typically `443`).                                                          |
| `path`        | WebSocket / xhttp endpoint path.                                                             |
| `serverName`  | TLS SNI (Server Name Indication).                                                            |
| `transport`   | Transport protocol: `ws`, `httpupgrade`, `grpc`, or `xhttp`.                                |
| `tls`         | `true` to enable TLS, `false` for plain connections.                                         |
| `fingerprint` | TLS client fingerprint: `chrome`, `firefox`, `safari`, `randomized`, etc.                   |
| `subnetsList` | Default IP list — a URL or local file path. Overridden by `--subnets` on the command line.  |

---

## Usage

### Simple Mode (fast, no VPN overhead)
Tests IPs using a lightweight TLS fronting check directly against the Cloudflare edge — no download or upload tests, no heavy traffic. This is what you want for a first-pass scan to find live IPs quickly.

```bash
./CFScanner -c config.json
```

### VPN Mode (full speed test)
Spins up an embedded Xray-core VLESS connection for each IP and measures real download (and optionally upload) speed through your proxy config.

```bash
./CFScanner -c config.json --vpn
./CFScanner -c config.json --vpn --upload   # also test upload speed
```

### Using the default `config.json`
If `config.json` exists in the same directory as the binary, the `-c` flag can be omitted:
```bash
./CFScanner
```

---

## Flags Reference

| Flag                   | Short | Default  | Description                                                                     |
|------------------------|-------|----------|---------------------------------------------------------------------------------|
| `--config`             | `-c`  | —        | Path to config JSON. Falls back to `config.json` in the binary's directory.    |
| `--vpn`                |       | `false`  | Enable VPN mode (VLESS + Xray-core connection test with speed measurement).     |
| `--threads`            | `-t`  | `4`      | Number of parallel scan threads.                                                |
| `--subnets`            | `-s`  | —        | Target: subnet file path, CIDR, single IP, or URL. Overrides `subnetsList`.    |
| `--shuffle`            |       | `false`  | Randomise the IP order before scanning.                                         |
| `--tries`              | `-n`  | `1`      | Times to test each IP. IP is marked OK only if **all** tries succeed.           |
| `--upload`             |       | `false`  | (VPN mode only) Run upload speed test in addition to download.                  |
| `--fronting`           |       | `false`  | (VPN mode only) Perform an extra domain-fronting check before the speed test.   |
| `--skip-time`          |       | `0`      | Move to the next subnet after N minutes (0 = disabled).                         |
| `--skip-count`         |       | `0`      | Move to the next subnet after finding N successful IPs (0 = disabled).          |
| `--download-speed`     |       | `50`     | Minimum acceptable download speed (KB/s). VPN mode only.                        |
| `--upload-speed`       |       | `50`     | Minimum acceptable upload speed (KB/s). VPN mode only.                          |
| `--download-time`      |       | `2`      | Maximum duration for the download test (s). VPN mode only.                      |
| `--upload-time`        |       | `2`      | Maximum duration for the upload test (s). VPN mode only.                        |
| `--download-latency`   |       | `3.0`    | Maximum allowed download latency (s). VPN mode only.                            |
| `--upload-latency`     |       | `3.0`    | Maximum allowed upload latency (s). VPN mode only.                              |
| `--fronting-timeout`   |       | `1.0`    | Timeout for the fronting / simple test (s).                                     |
| `--loglevel`           | `-l`  | `none`   | Xray-core internal log level: `debug`, `info`, `warning`, `error`, `none`.     |

---

## Interactive Controls

While a scan is running you can control it with single key-presses (no Enter needed):

| Key           | Action                                                              |
|---------------|---------------------------------------------------------------------|
| `P`           | Pause scanning.                                                     |
| `R`           | Resume a paused scan.                                               |
| `S`           | Skip the current subnet and immediately start the next one.         |
| `Esc` / `Ctrl+C` | Cancel the entire scan and shut down cleanly.                   |

---

## Output

All result files are created inside a `result/` directory automatically.

| File                             | Content                                                                 |
|----------------------------------|-------------------------------------------------------------------------|
| `YYYY-MM-DD_HH:MM:SS_result.txt` | Live interim file — appended as IPs are found during the scan.         |
| `YYYY-MM-DD_HH:MM:SS_final.txt`  | Final file — same IPs sorted ascending by latency, written at the end. |

**Simple mode** output line:
```
[OK] 172.67.187.19
```

**VPN mode** output line:
```
[OK] 172.67.187.19     913 ms  dl:   0.351 mbps  ul:   0.000 mbps
```

---

## Examples

```bash
# Fast simple scan using subnets embedded in config
./CFScanner -c config.json

# Simple scan with a specific CIDR, 8 threads, shuffled
./CFScanner -c config.json -s 172.67.160.0/20 -t 8 --shuffle

# VPN speed test, skip each subnet after finding 3 good IPs
./CFScanner -c config.json --vpn --skip-count 3

# VPN test with upload, try each IP 2 times
./CFScanner -c config.json --vpn --upload -n 2

# Test a single IP in VPN mode
./CFScanner -c config.json --vpn -s 172.67.187.19
```

# License

CFScanner is released under the [GPL-3](../LICENSE) license.

# Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](../CONTRIBUTING.md) for more information.

[go]: https://img.shields.io/badge/Go-cyan?logo=go
