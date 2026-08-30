# tunnelsnoop

[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](#license)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20WSL2-orange.svg)](#requirements)

**tunnelsnoop** is a lightweight, zero-privilege CLI monitor and idle reaper for developer workstations. It automatically tracks active port-forwards (`kubectl`, `ssh`, `cloudflared`, and `ngrok`), audits local network exposure (flagging `0.0.0.0` wildcard bindings), monitors client connection activity and I/O throughput, and safely terminates abandoned tunnels.

---

## Overview

Developers routinely bridge remote VPC services and production databases to local workstations:

```bash
kubectl port-forward svc/prod-postgres 5432:5432 -n prod
ssh -L 6379:redis.internal.vpc:6379 bastion.example.com
cloudflared access tcp --hostname db.internal --url 127.0.0.1:5432
```

While convenient, local port-forwarding introduces real operational and security risks:

- **Accidental Wildcard Exposure**: Specifying `--address 0.0.0.0` or `-L 0.0.0.0:...` binds forwarded endpoints to all interfaces, exposing private databases to everyone on the local Wi-Fi or LAN.
- **Unauthenticated Loopback Access**: Local databases bound to `127.0.0.1` rarely require authentication. Rogue scripts, compromised dependencies (`npm`, `pip`), or browser-based local port scans can query production resources.
- **Zombie Tunnels**: Background port-forwards left lingering in forgotten terminal tabs, `tmux` sessions, or IDEs maintain long-lived reverse conduits into production networks.

`tunnelsnoop` runs without root privileges, scanning `/proc` to deliver continuous visibility, risk detection, and optional automatic reaping.

---

## Features

- **Zero-Privilege Auditing**: Runs entirely in unprivileged user space by inspecting `/proc` entries matching the current user's UID (`os.Getuid()`). No `sudo`, kernel modules, or `eBPF` prerequisites.
- **Multi-Tool Detection**: Out-of-the-box discovery for `kubectl`, `ssh`, `cloudflared`, and `ngrok`.
- **Security Exposure Badging**: Instantly identifies non-loopback bindings (`0.0.0.0`, `::`, or external IP addresses) and marks them with high-visibility alerts.
- **Activity & Idle Tracking**: Correlates active client connections (`ESTABLISHED` sockets) and process byte counters (`/proc/<pid>/io`) to track exact idle duration.
- **Safe Automated Reaper**: Optional `--kill-idle` mode terminates inactive tunnels using progressive signal escalation (`SIGTERM` followed by `SIGKILL`), protected against PID recycling via command-line and socket inode verification.
- **Dual Output Modes**:
  - **Interactive Terminal UI**: ANSI-colored dashboard with real-time status updates.
  - **Structured JSON Streaming**: Machine-readable JSON output for scripting, SIEM integration, and audit pipelines.

---

## Architecture & How It Works

`tunnelsnoop` operates via a low-overhead periodic reconciliation loop:

```
                      ┌─────────────────────────────────────────┐
                      │            tunnelsnoop CLI              │
                      └────────────────────┬────────────────────┘
                                           │
                                           ▼
                      ┌─────────────────────────────────────────┐
                      │              Engine Loop                │
                      │  - Ticker Cadence (default: 5s)         │
                      │  - In-Memory Tunnel State Cache         │
                      └─────────────┬─────────────┬─────────────┘
                                    │             │
              ┌─────────────────────┘             └─────────────────────┐
              ▼                                                         ▼
┌───────────────────────────┐                             ┌───────────────────────────┐
│       procfs Scanner      │                             │    Presenter & Reaper     │
│ - Read /proc/net/tcp{,6}  │                             │ - ANSI Table / JSON Stream│
│ - Match LISTEN & ESTAB    │                             │ - Calculate Idle Duration │
│ - Match PID via /proc/*/fd│                             │ - Safe Termination        │
│ - Sample /proc/<pid>/io   │                             │   (SIGTERM -> SIGKILL)    │
└───────────────────────────┘                             └───────────────────────────┘
```

1. **Socket Inspection**: Reads `/proc/net/tcp` and `/proc/net/tcp6` to extract sockets in `LISTEN` (`0A`) and `ESTABLISHED` (`01`) states.
2. **Process Correlation**: Scans `/proc/[pid]` directories owned by the current user UID, checking binary names in `/proc/<pid>/comm` against supported tools.
3. **Descriptor Mapping**: Resolves symbolic links under `/proc/<pid>/fd/*` (`socket:[<inode>]`) to map listening sockets to their owning processes while deduplicating multi-threaded descriptors.
4. **Activity Telemetry**:
   - Tallies established client connections targeting the tunnel's listening port and protocol.
   - Reads process byte counters from `/proc/<pid>/io` (`read_bytes`, `write_bytes`).
   - Resets the tunnel's idle timer whenever active clients or byte delta increases are observed.
5. **Safe Reaper Verification**: When `--kill-idle` is enabled and a tunnel exceeds the duration threshold:
   - Verifies `/proc/<pid>/comm` still matches the target binary.
   - Verifies `/proc/<pid>/fd` still holds the recorded socket inode (preventing PID reuse attacks).
   - Issues `SIGTERM`, monitors for process exit during a 5-second grace window, and escalates to `SIGKILL` only if the process remains alive.

---

## Requirements

- **Operating System**: Linux kernel 2.6.32+ or Windows Subsystem for Linux (WSL2).
- **Go Toolchain**: Go 1.22 or later (standard library only; zero third-party dependencies).
- **Permissions**: Standard unprivileged user account.

---

## Installation

### From Source

Clone the repository and build the binary:

```bash
git clone https://github.com/DiAndEn0/tunnel-snoop.git
cd tunnel-snoop
go build -o bin/tunnelsnoop ./cmd/tunnelsnoop
```

Optionally copy or link the binary into your `$PATH`:

```bash
sudo install -m 755 bin/tunnelsnoop /usr/local/bin/
```

---

## Usage

```bash
tunnelsnoop [flags]
```

### Command-Line Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-interval` | `duration` | `5s` | Polling cadence for continuous monitoring (e.g., `2s`, `10s`). |
| `-kill-idle` | `duration` | `0` | Automatically terminate tunnels idle longer than duration (e.g., `15m`, `1h`). `0` disables reaping. |
| `-json` | `bool` | `false` | Emit structured JSON output instead of interactive ANSI table. |
| `-once` | `bool` | `false` | Perform a single scan and exit immediately. |

---

## Examples

### 1. Interactive Terminal Monitor

Monitor active port-forwards in real time with continuous refresh:

```bash
tunnelsnoop
```

Output:

```text
tunnelsnoop - Active Port-Forward Monitor [14:22:05]

PID     PROCESS      LOCAL BINDING        CLIENTS  IDLE       SECURITY  
---------------------------------------------------------------------------
14021   kubectl      127.0.0.1:5432       1        active     [SAFE]
15882   ssh          0.0.0.0:6379         0        4m12s      [EXPOSED 0.0.0.0]
```

### 2. Single-Pass Security Audit

Inspect active tunnels once and exit without clearing the terminal (ideal for login shell profiles or shell startup scripts):

```bash
tunnelsnoop -once
```

### 3. Automatically Terminate Idle Tunnels

Monitor continuously and terminate any tunnel that has been idle for more than 15 minutes:

```bash
tunnelsnoop -kill-idle 15m
```

When an idle tunnel exceeds the threshold, `tunnelsnoop` logs the termination to `stderr`:

```text
Killing idle tunnel PID 15882 (0.0.0.0:6379)...
```

### 4. Structured JSON Output

Emit JSON snapshots for scripting, alerting, or forwarding into log aggregators:

```bash
tunnelsnoop -once -json
```

Output:

```json
[
  {
    "pid": 14021,
    "process_name": "kubectl",
    "command_line": "kubectl port-forward svc/postgres 5432:5432",
    "local_address": "127.0.0.1",
    "local_port": 5432,
    "protocol": "tcp",
    "socket_inode": 481029,
    "is_wildcard": false,
    "first_seen": "2026-08-31T14:15:00Z",
    "last_active": "2026-08-31T14:22:00Z",
    "active_clients": 1,
    "bytes_read": 1048576,
    "bytes_written": 2097152,
    "idle_duration": 5000000000
  }
]
```

Filter exposed tunnels using `jq`:

```bash
tunnelsnoop -once -json | jq '.[] | select(.is_wildcard == true)'
```

---

## Security Model & Protections

### Wildcard Exposure Detection
A socket bound to `0.0.0.0` or `::` accepts connections on all network interfaces. `tunnelsnoop` checks the local socket address and tags any wildcard listener with `[EXPOSED 0.0.0.0]`.

### PID Recycling Prevention
Process IDs on Linux are reused over time. To avoid terminating an unrelated process that acquired a stale tunnel's PID, the reaper performs two pre-flight checks before signaling:
1. **Binary Name Match**: Verifies `/proc/<pid>/comm` matches the expected executable name.
2. **Socket Inode Invariance**: Re-scans `/proc/<pid>/fd` to ensure the process still owns the exact socket inode associated with the port-forward.

If either check fails, termination is aborted immediately.

---

## Project Structure

```
tunnel-snoop/
├── cmd/
│   └── tunnelsnoop/
│       └── main.go           # CLI entrypoint, flag parsing, signal lifecycle
├── internal/
│   ├── model/
│   │   └── tunnel.go         # Core data structures (Tunnel, SocketEntry)
│   ├── monitor/
│   │   ├── engine.go         # State engine, reconciliation loop, activity tracker
│   │   └── engine_test.go    # Engine reconciliation unit tests
│   ├── procfs/
│   │   ├── io.go             # /proc/<pid>/io byte counter reader
│   │   ├── io_test.go        # Unit tests for procfs I/O parsing
│   │   ├── parser.go         # /proc/net/tcp and /proc/net/tcp6 parser
│   │   ├── parser_test.go    # Socket table mock tests
│   │   ├── process.go        # Process table scanner & socket descriptor correlation
│   │   └── process_test.go   # Mock process correlation tests
│   ├── reaper/
│   │   ├── reaper.go         # Process verification and SIGTERM/SIGKILL escalation
│   │   └── reaper_test.go    # Process termination unit tests
│   └── ui/
│       ├── json.go           # JSON formatting and serialization
│       ├── table.go          # ANSI terminal table rendering
│       └── ui_test.go        # Table and JSON formatting tests
├── tests/
│   └── integration_test.go   # End-to-end integration tests
├── go.mod                    # Module definition (Go 1.22+)
└── README.md
```

---

## Development & Testing

### Running Tests

Execute all unit and integration tests:

```bash
go test -v ./...
```

### Code Style & Formatting

Format Go code and verify tidy module dependencies:

```bash
go fmt ./...
go vet ./...
```

---

## License

This project is licensed under the [MIT License](LICENSE).
