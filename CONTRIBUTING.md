# Contributing

## Requirements

Linux or WSL2, and Go 1.22 or newer. There are no third-party dependencies, so
`go build ./...` works on a fresh clone with no module downloads.

## Before opening a pull request

```bash
gofmt -l .          # must print nothing
go vet ./...
go test -race ./...
```

CI runs the same checks plus [golangci-lint](https://golangci-lint.run/). The
race detector needs cgo and therefore a C compiler; if you don't have one
locally, run `go test ./...` and let CI cover the race build.

## Commit messages

The project releases through
[release-please](https://github.com/googleapis/release-please), which derives
the next version from commit messages, so the
[Conventional Commits](https://www.conventionalcommits.org/) prefix is what
decides whether a release happens at all:

- `fix:` — patch release
- `feat:` — minor release
- `feat!:` or a `BREAKING CHANGE:` trailer — major release
- `docs:`, `test:`, `refactor:`, `chore:` — no release

Pull requests are squash-merged, and the squash commit takes the PR title, so
the **title** needs the prefix. A scope is welcome: `fix(reaper): …`.

## Testing against /proc

Anything that reads `/proc` should be testable without root and without racing
the real process table. Take the procfs root as a parameter and point it at a
fixture directory in tests — see `internal/procfs/testdata/` for checked-in
fixtures, and `internal/reaper/guard_test.go` for building a synthetic tree at
runtime with `t.TempDir()`.

Ownership-filtered code paths need care: unprivileged, reading another user's
`/proc/<pid>/fd` already fails with `EACCES`, so a test that merely scans real
`/proc` cannot tell a working filter from a missing one. Drive the filter
directly with an explicit UID.

## Changes affecting the reaper

`internal/reaper` terminates processes the tool does not own. Any change there
should keep the existing invariant: process identity is re-verified immediately
before every signal, and the reaper aborts with a descriptive error rather than
signalling when verification fails. New abort paths need a test that asserts the
target survives.
