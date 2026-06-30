# CFScanner

![go]
![version]
![license]

**CFScanner** is a fast, standalone Cloudflare edge IP scanner written in Go. It identifies Cloudflare edge IPs that are accessible and performant for use with VLESS/Xray-based proxies.

It ships with a built-in **xray-core** engine, so no external binaries are required. In simple mode it tests IPs with a lightweight TLS connection. In VPN mode it tunnels full download and upload speed tests through your VLESS proxy to accurately measure real-world performance.

---

## Table of Contents
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Flags Reference](#flags-reference)
- [Interactive Controls](#interactive-controls)
- [Output](#output)
- [License](#license)

---

## Features
- **Two scan modes**: fast simple test (TLS fronting check) and full VPN speed test (download + upload via Xray-core).
- **Multiple transports**: `ws`, `httpupgrade`, `grpc`, `xhttp` — each with optional TLS.
- **Built-in xray-core**: fully self-contained, no external `xray` binary needed.
- **Real-time output**: clean, live progress bar with immediate `[OK]` results printed above it.
- **Subnet grouping**: automatically groups IPs by `/24` and respects per-subnet skip rules.
- **Interactive controls**: pause, resume, and skip subnets mid-scan without restarting.
- **Fast shutdown**: pressing `Esc`/`Ctrl+C` cancels all in-flight network requests immediately.
- **Sorted results**: final output file is sorted ascending by latency.

---

## Requirements
- **Go 1.20+** — only required if building from source.
- **Linux / macOS / Windows**.

---

## Installation

### Option 1 — Pre-compiled Binary (Recommended)
1. Go to the [Releases](https://github.com/MortezaBashsiz/CFScanner/releases) page and download the archive for your OS/architecture.
2. Extract and place the `CFScanner` executable somewhere convenient.
3. On Linux/macOS, make it executable:
   ```bash
   chmod +x CFScanner
   ```

### Option 2 — Build from Source
```bash
git clone https://github.com/MortezaBashsiz/CFScanner.git
cd CFScanner/golang
go build -o CFScanner -trimpath -ldflags "-s -w -buildid=" .
```

---

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

---

## License

Released under the [GPL-3.0](LICENSE) license.

[go]: https://img.shields.io/badge/Go-1.20+-cyan?logo=go
[version]: https://img.shields.io/badge/Version-2.0-blue
[license]: https://img.shields.io/badge/License-GPL--3.0-green
