# Security Policy

## Reporting a Vulnerability

Please report security issues privately through GitHub's
[private vulnerability reporting](https://github.com/DiAndEn0/tunnel-snoop/security/advisories/new)
rather than opening a public issue.

Include the version or commit, your Linux distribution and kernel version, and
the steps needed to reproduce. A proof of concept helps but is not required.

You can expect an acknowledgement within 7 days and a status update within 30.

## Supported Versions

Fixes are issued against the latest release only. The project is pre-1.0 in
practice and there are no maintained release branches.

## Scope

tunnelsnoop reads `/proc` and, with `-kill-idle`, sends signals to processes.
The following are in scope:

- Discovering or terminating a process **not** owned by the invoking user
- Terminating a process other than the tunnel that was discovered — for example
  by exploiting PID reuse between discovery and signalling
- Escalating privileges beyond those of the invoking user
- Crashes or resource exhaustion triggered by hostile `/proc` contents, such as
  a process controlling its own `comm` or command line

Out of scope:

- Findings that require the tool to already be running as root. Running as root
  is not a supported configuration; discovery is scoped to the invoking UID
  precisely so that an elevated run does not reach other users' processes.
- Inaccurate client counts or idle durations. These are correctness bugs —
  please open a normal issue.
- Exposure of data already world-readable in `/proc`, such as another user's
  command line, unless tunnelsnoop widens the audience for it.

## Known Considerations

`-kill-idle` sends `SIGTERM` and then `SIGKILL`. Process identity is verified
immediately before each signal, by comparing `/proc/<pid>/comm` against the
discovered binary name and confirming the listening socket is still held. This
narrows the window between check and signal but does not eliminate it; the
kernel offers no way to close it entirely short of `pidfd_open(2)`.

JSON output includes each tunnel's full command line, which is read from
`/proc/<pid>/cmdline`. That file is world-readable, so this discloses nothing
new on the host — but the output aggregates it into one artifact. Command lines
routinely carry tokens and internal hostnames, so treat JSON output as
sensitive before pasting it into an issue or shipping it to a log collector.
