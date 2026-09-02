# tunnelsnoop

[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20WSL2-orange.svg)](#requirements)

**tunnelsnoop** is a zero-privilege CLI monitor and idle reaper for developer workstations. It audits local port-forwards (`kubectl`, `ssh`, `cloudflared`, `ngrok`), alerts on accidental `0.0.0.0` wildcard and LAN-reachable exposures, monitors client traffic via `/proc`, and automatically terminates abandoned tunnels.

```text
tunnelsnoop - Active Port-Forward Monitor [14:22:05]

PID     PROCESS      LOCAL BINDING        CLIENTS  IDLE       SECURITY          
--------------------------------------------------------------------------------
14021   kubectl      127.0.0.1:5432       1        active     [SAFE]
16104   ssh          192.168.1.40:2222    2        active     [EXPOSED LAN]
15882   ssh          0.0.0.0:6379         0        4m12s      [EXPOSED 0.0.0.0]
```

---

## Key Features

- **Zero-Privilege Auditing**: Runs entirely in unprivileged user space. Discovery is scoped to processes owned by the invoking user's UID (`os.Getuid()`), so an elevated run still reports only that account's tunnels. Requires no `sudo` or kernel modules.
- **Multi-Tool Discovery**: Automatic detection for `kubectl`, `ssh`, `cloudflared`, and `ngrok`.
- **Security Exposure Badging**: Highlights non-loopback bindings (`0.0.0.0`, `::`) exposing internal services to the local LAN.
- **Activity & Idle Tracking**: Correlates active TCP client connections with `/proc/<pid>/io` byte deltas to measure true idle duration.
- **Safe Automated Reaper**: Optional `-kill-idle` gracefully shuts down stale tunnels (`SIGTERM` with 5s grace before `SIGKILL`), verifying PID reuse and socket inodes before signaling.
- **Dual Output**: Interactive ANSI dashboard or structured streaming JSON for pipelines and SIEM logging.

---

## Quickstart

### Installation

Requires Linux or WSL2 with Go 1.22+:

```bash
git clone https://github.com/DiAndEn0/tunnel-snoop.git
cd tunnel-snoop
go build -o bin/tunnelsnoop ./cmd/tunnelsnoop
sudo install -m 755 bin/tunnelsnoop /usr/local/bin/
sudo install -m 644 man/man1/tunnelsnoop.1 /usr/local/share/man/man1/
```

### CLI Reference

```text
Usage: tunnelsnoop [flags]

  -interval duration   Polling interval for continuous monitor (default 5s)
  -kill-idle duration  Terminate tunnels idle longer than duration (e.g. 15m)
  -json                Emit output as structured JSON
  -once                Perform a single scan and exit
  -port int            Report only tunnels listening on this local port
  -process string      Report only tunnels whose process name is in this list
  -exposed-only        Report only tunnels flagged as exposed (0.0.0.0, ::)
  -min-idle duration   Report only tunnels idle at least this long (e.g. 15m)
  -fail-on-exposed     Exit 1 if any exposed tunnel is found
  -version             Print version and exit
```

**Exit status**

| Code | Meaning |
| ---- | ------- |
| `0`  | Normal completion; no exposed tunnel found, or `-fail-on-exposed` was not set |
| `1`  | `-fail-on-exposed` was set and at least one exposed tunnel was seen in the filtered set |
| `2`  | Operational error (invalid command-line arguments; a usage summary goes to stderr) |

---

## Common Workflows

```bash
# 1. Interactive real-time dashboard
tunnelsnoop

# 2. Single-pass security check (shell profile or CI)
tunnelsnoop -once

# 3. Terminate tunnels inactive for longer than 15 minutes
tunnelsnoop -kill-idle 15m

# 4. JSON pipeline: alert on wildcard-exposed tunnels
tunnelsnoop -once -json | jq '.[] | select(.is_wildcard)'

# 5. Reap only idle kubectl tunnels, leaving ssh tunnels alone
tunnelsnoop -process kubectl -kill-idle 15m

# 6. Audit the exposed tunnels on one port
tunnelsnoop -once -port 6379 -exposed-only

# 7. CI gate / pre-commit hook: fail the step on any wildcard exposure
tunnelsnoop -once -exposed-only -fail-on-exposed
```

### Filtering

`-port`, `-process`, `-exposed-only` and `-min-idle` combine with a logical AND
and are applied to the reconciled tunnel set before anything else consumes it.
The filtered set is both what gets reported **and** what `-kill-idle` reaps, so
`-process kubectl -kill-idle 15m` terminates idle `kubectl` tunnels only.

`-process` takes a comma-separated list (`kubectl,ssh`); names are compared
case-insensitively and must match in full, so `kube` selects nothing.

`-fail-on-exposed` is scoped the same way: it fails only on exposures in the
filtered set, and the report is still written in full so a red CI step shows
what tripped it. With `-once` the status is decided by the single scan; in
continuous mode any exposure seen during the run is remembered and applied when
the monitor is interrupted.

---

## How It Works

1. **Scan**: Reads `/proc/net/tcp{,6}` for sockets in `LISTEN` (`0A`) and `ESTABLISHED` (`01`) states.
2. **Correlate**: Traverses `/proc/<pid>/fd/*` symlinks (`socket:[inode]`) matching target binary names, skipping any process whose `/proc/<pid>` directory is not owned by the invoking UID.
3. **Telemetry**: Tracks client connections and reads `/proc/<pid>/io` byte counters to maintain idle timestamps.
4. **Reap**: Verifies binary name and socket inode invariance to prevent PID recycling races before issuing `SIGTERM`/`SIGKILL`.

---

## Documentation & Assets

- **Manual Page**: Detailed CLI options and security considerations available via `man tunnelsnoop` or `man -l man/man1/tunnelsnoop.1`.
- **Terminal Demo**: Reproducible demo GIF recording script defined in [`demo.tape`](demo.tape) for use with [VHS](https://github.com/charmbracelet/vhs).

---

## Testing

```bash
go test -v ./...
```

---

## Contributing

Bug reports and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md)
for the build, test, and commit-message conventions.

To report a security vulnerability, please follow [SECURITY.md](SECURITY.md)
rather than opening a public issue.

---

## License

This project is licensed under the [MIT License](LICENSE).
