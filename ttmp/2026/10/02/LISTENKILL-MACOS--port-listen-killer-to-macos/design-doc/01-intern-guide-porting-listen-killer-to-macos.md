---
Title: Intern Guide - Porting Listen Killer to macOS
Ticket: LISTENKILL-MACOS
Status: active
Topics:
    - go
    - bubbletea
    - macos
    - porting
    - networking
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/listen-killer/cmds/tui/kill.go
      Note: Glazed kill command and target resolution
    - Path: repo://cmd/listen-killer/cmds/tui/tui.go
      Note: Glazed list command and RunTUI entry
    - Path: repo://pkg/listener/process.go
      Note: Signalling via syscall.Kill; portable to Darwin
    - Path: repo://pkg/listener/scanner.go
      Note: Primary OS-dependent discovery surface; uses gopsutil net+process
    - Path: repo://pkg/listener/types.go
      Note: Central ListenerInfo contract shared by TUI/CLI
    - Path: repo://pkg/tui/model.go
      Note: listenerURL + browser open; IPv6 bug site
ExternalSources:
    - https://github.com/shirou/gopsutil
    - https://github.com/charmbracelet/bubbletea
    - https://github.com/go-go-golems/glazed
Summary: 'Complete intern-facing analysis, design and implementation guide for porting Listen Killer from Linux to macOS: system overview, architecture, macOS-specific behavior of process/socket discovery, gaps, and a phased implementation plan with pseudocode.'
LastUpdated: 2026-10-02T10:26:55.174465-04:00
WhatFor: Onboarding a new intern to the Listen Killer codebase and to the specific work of making it correct and distributable on macOS.
WhenToUse: Before making any macOS-related change to Listen Killer; also as a general architecture tour of the app.
---

# Intern Guide - Porting Listen Killer to macOS

> **Audience.** A new intern who knows basic Go (structs, interfaces, goroutines,
> `go build`, `go test`) but has never built a Bubbletea TUI and has never
> written OS-specific Go code. No prior knowledge of macOS internals is assumed.
>
> **Goal.** After reading this document you should be able to (a) explain every
> layer of Listen Killer, (b) explain exactly how the app discovers sockets and
> processes on macOS versus Linux, (c) name every known macOS gap and why it
> exists, and (d) implement a phased port without breaking the existing Linux
> behavior.
>
> **Evidence.** All commands and outputs in this document were produced on:
> `Darwin 24.6.0 (macOS 15.x, arm64, Apple M1 Pro)`, `go1.27.1 darwin/arm64`,
> repository revision `ef03a2c`. Where a claim comes from third-party source,
> the exact file in the Go module cache is cited.

---

## Table of Contents

1. [What Listen Killer Is](#1-what-listen-killer-is)
2. [Repository Map and Every File](#2-repository-map-and-every-file)
3. [The Three-Layer Architecture](#3-the-three-layer-architecture)
4. [The Central Data Type: `ListenerInfo`](#4-the-central-data-type-listenerinfo)
5. [How Discovery Works on Linux](#5-how-discovery-works-on-linux)
6. [How Discovery Works on macOS (the important part)](#6-how-discovery-works-on-macos-the-important-part)
7. [Runtime Evidence from a Real macOS Box](#7-runtime-evidence-from-a-real-macos-box)
8. [The macOS Gap Register -- What Actually Breaks](#8-the-macos-gap-register----what-actually-breaks)
9. [Design: A Platform Abstraction for the Scanner](#9-design-a-platform-abstraction-for-the-scanner)
10. [Alternatives Considered](#10-alternatives-considered)
11. [Implementation Plan (Phased)](#11-implementation-plan-phased)
12. [API Reference](#12-api-reference)
13. [Testing and Verification Playbook](#13-testing-and-verification-playbook)
14. [Packaging, Signing and Distribution](#14-packaging-signing-and-distribution)
15. [Glossary](#15-glossary)
16. [Open Questions](#16-open-questions)

---

## 1. What Listen Killer Is

Every developer machine slowly accumulates **stale listening daemons**: Vite dev
servers that were never shut down, database proxies, API mocks, editor plugins,
Docker port-forwarders, and language servers. Each one holds a TCP port open.
After a few days you cannot remember which port belongs to which checkout, and
`kill <pid>` requires you to find the PID first.

**Listen Killer** is a small Go program that solves this one problem:

- It enumerates every TCP socket that is in the `LISTEN` state.
- It enriches each socket with the owning process's metadata: PID, name,
  command line, executable path, working directory, user, uptime, CPU %,
  and resident memory.
- It presents that data in two ways:
  1. an **interactive terminal dashboard** (a Bubbletea TUI) where you can
     navigate, mark multiple rows, open a listener in a browser, and send
     `TERM`/`KILL`/`INT` to one or many processes; and
  2. a **scriptable CLI** (built on the Glazed command framework) that emits
     the same data as Markdown, JSON, YAML, CSV, or a table, and offers a safe
     `kill` verb that is dry-run-first and requires `--yes` to actually signal.

Think of it as "`ss -tlnp` meets `htop`", with a machine-readable mode for
agents and scripts.

The reason this ticket exists is that the program was originally written and
tested on Linux. The code *compiles* on macOS and *mostly* works, but the
underlying discovery mechanism is completely different on Darwin, and several
fields silently degrade. "Porting to macOS" is therefore less about making it
compile and more about (a) understanding the Darwin discovery path, (b) fixing
the degraded fields, and (c) packaging the binary so it can be distributed
without Gatekeeper rejecting it.

---

## 2. Repository Map and Every File

```
listen-killer/
├── go.mod                       module github.com/wesen/listen-killer, go 1.25.5
├── go.sum
├── README.md                    user-facing docs + screenshots
├── .ttmp.yaml                   docmgr config (docs live under ttmp/)
├── screenshots/                 PNGs referenced by README
├── cmd/
│   └── listen-killer/
│       ├── main.go              Cobra root; wires list/tui/kill subcommands
│       └── cmds/tui/
│           ├── tui.go           Glazed "list" command + RunTUI()
│           ├── kill.go          Glazed "kill" command + target resolution
│           └── helpers.go       shared parsing/filter helpers + row builder
├── pkg/
│   ├── listener/                ← OS-dependent layer
│   │   ├── types.go             ListenerInfo, FormatUptime, FormatBytes
│   │   ├── scanner.go           ScanListeners() via gopsutil  ← PORT TARGET
│   │   └── process.go           KillProcess() via syscall.Kill
│   └── tui/                     ← OS-independent UI layer
│       ├── model.go             Model struct, messages, commands, URL builder
│       ├── update.go            Elm-style Update: keys, messages, state machine
│       ├── view.go              Elm-style View: title/table/detail/kill dialog
│       ├── keymap.go            Bubbletea help KeyMap
│       └── styles.go            Lipgloss styles
└── ttmp/                        docmgr tickets (docs, not code)
```

The most important thing to internalize: **only two files in `pkg/listener/`
touch the operating system** — `scanner.go` (discovery) and `process.go`
(signalling). Everything in `pkg/tui/` and the Glazed command layer is
deliberately OS-independent. Your port should preserve that boundary. If you
find yourself writing `runtime.GOOS` inside `pkg/tui/`, stop and reconsider.

There is exactly **one** OS check in the whole TUI already, and it is a good
model for how to do it:

```go
// pkg/tui/model.go — openBrowserCmd
switch runtime.GOOS {
case "linux":
    cmd = exec.Command("xdg-open", url)
case "darwin":
    cmd = exec.Command("open", url)   // ← macOS path already present
default:
    return BrowserOpenedMsg{URL: url, Err: fmt.Errorf("unsupported OS: %s", runtime.GOOS)}
}
```

So "open in browser" is *already* ported. The work is elsewhere.

### Directories that matter to a porter

| Path | Why it matters for the macOS port |
|------|-----------------------------------|
| `pkg/listener/scanner.go` | The only real porting surface. Uses `gopsutil/net` + `gopsutil/process`. |
| `pkg/listener/process.go` | `syscall.Kill`; signal constants are identical on Darwin, so this is already correct. |
| `pkg/listener/types.go` | Pure formatting; no OS calls. Safe on every platform. |
| `cmd/listen-killer/cmds/tui/*.go` | Filters, target resolution, row building. Pure logic. |
| `pkg/tui/*.go` | Bubbletea UI. Pure logic + the one `runtime.GOOS` browser switch. |

---

## 3. The Three-Layer Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│ LAYER 3 — CLI / PROCESS ENTRY        cmd/listen-killer/            │
│                                                                    │
│  main.go : cobra root, help system, default-to-list                │
│      ├── "list"  → tui.NewListCommand()  (Glazed GlazeCommand)     │
│      ├── "tui"   → apptui.NewModel() + tea.NewProgram              │
│      └── "kill"  → tui.NewKillCommand()  (Glazed GlazeCommand)     │
│                                                                    │
│  Responsibility: parse flags, choose output format, gate destruction│
├────────────────────────────────────────────────────────────────────┤
│ LAYER 2 — INTERACTIVE UI             pkg/tui/                      │
│                                                                    │
│  Model (state) ──Update(msg)──▶ (Model, Cmd) ──View()──▶ string    │
│      table.Model (bubbles/table), help.Model, marks map, kill mode │
│                                                                    │
│  Responsibility: render the dashboard, map keys to state changes   │
├────────────────────────────────────────────────────────────────────┤
│ LAYER 1 — SYSTEM DISCOVERY           pkg/listener/                 │
│                                                                    │
│  ScanListeners()  ──▶ []ListenerInfo   ← OS-SPECIFIC               │
│  KillProcess()    ──▶ syscall.Kill     ← OS-SPECIFIC               │
│                                                                    │
│  Responsibility: turn the OS's socket + process tables into        │
│                  a portable []ListenerInfo                         │
└────────────────────────────────────────────────────────────────────┘
```

**Data flow for `listen-killer list --output json`:**

```
main.go
  └─ rootCmd.Execute() ─▶ Glazed list command
        └─ ListCommand.RunIntoGlazeProcessor(ctx, vals, gp)
              └─ runCLI(ctx, gp, settings)
                    ├─ listener.ScanListeners()          ── OS calls ──┐
                    ├─ filter (port/name/user/path)                     │
                    └─ gp.AddRow(ctx, listenerRow(l)) ──▶ JSON/YAML/CSV │
                                                                        │
                       ┌────────────────────────────────────────────────┘
                       ▼
        gopsutil/net.Connections("inet")      [≈ lsof on macOS]
              └─ for each LISTEN socket:
                    gopsutil/process.NewProcess(pid)
                          ├─ Name()     [ps / kinfo]
                          ├─ Cmdline()  [ps]
                          ├─ Exe()      [libproc or lsof]
                          ├─ Cwd()      [libproc or unsupported]
                          ├─ Username() [kinfo]
                          ├─ Percent(0) [cpu times delta]
                          ├─ MemoryInfo() [ps rss]
                          └─ CreateTime() [kinfo]
```

The key insight for the port: **`ScanListeners` is an N×M fan-out of shell
commands and syscalls.** On Linux the socket table is read directly from the
kernel (`/proc`); on macOS it is read by *shelling out to `lsof`*, and process
enrichment shells out to `ps` or calls `libproc`. That difference drives almost
every gap in Section 8.

---

## 4. The Central Data Type: `ListenerInfo`

Everything in the app is built around one struct in `pkg/listener/types.go`:

```go
type ListenerInfo struct {
    PID           int32   `json:"pid"            glazed:"pid"`
    Name          string  `json:"name"           glazed:"name"`
    Cmdline       string  `json:"cmdline"        glazed:"cmdline"`
    Exe           string  `json:"exe"            glazed:"exe"`
    Cwd           string  `json:"cwd"            glazed:"cwd"`
    Username      string  `json:"username"       glazed:"username"`
    Port          uint32  `json:"port"           glazed:"port"`
    Address       string  `json:"address"        glazed:"address"`
    Protocol      string  `json:"protocol"       glazed:"protocol"`
    Uptime        string  `json:"uptime"         glazed:"uptime"`
    UptimeSeconds int64   `json:"uptime_seconds" glazed:"uptime_seconds"`
    CPUPercent    float64 `json:"cpu_percent"    glazed:"cpu_percent"`
    RSSBytes      uint64  `json:"rss_bytes"      glazed:"rss_bytes"`
    RSSHuman      string  `json:"rss_human"      glazed:"rss_human"`
}
```

Two invariants you must preserve when porting:

1. **`ListenerInfo` is OS-agnostic.** It must contain only plain Go types. Never
   leak `unix.KinfoProc`, a C struct, or a `lsof` line into it. The macOS
   scanner's job is to fill this struct from whatever Darwin gives us.
2. **The struct is a contract with three consumers**: the TUI table
   (`listenersToRows`), the Glazed row builder (`listenerRow`), and the kill
   target resolver. Adding a field is cheap; *removing* or *renaming* one
   breaks all three and the README.

`FormatUptime` and `FormatBytes` are pure functions and need no changes. Note
that `FormatUptime` takes **Unix milliseconds**, which is exactly what
gopsutil's `CreateTime()` returns on both Linux and Darwin (Darwin computes
`P_starttime.Sec*1000 + Usec/1000`), so uptime is already correct on macOS.

### The scanner contract, in pseudocode

```text
FUNCTION ScanListeners() -> []ListenerInfo, error:
    conns, err := net.Connections("inet")      # ALL inet sockets
    IF err: RETURN nil, err

    result = []
    FOR EACH conn IN conns:
        IF conn.Type != SOCK_STREAM: CONTINUE   # drop UDP
        IF conn.Status != "LISTEN":  CONTINUE   # keep only listeners

        info, err := gatherProcessInfo(conn)
        IF err: CONTINUE                        # skip un-inspectable procs
        APPEND info TO result

    RETURN result, nil
```

Notice the silent failure policy: `gatherProcessInfo` failures are swallowed
with `continue`. On macOS this is *load-bearing*, because permission failures
are common (see Section 8) — but it also means fields can vanish without any
user-visible error. Part of the port is making degradation visible.

---

## 5. How Discovery Works on Linux

For contrast, and because the existing code was written against Linux:
`net.Connections("inet")` on Linux reads `/proc/net/tcp` and `/proc/net/tcp6`
directly. Each row has a local address encoded as hex, a port, an inode, and a
state. gopsutil then maps the socket inode to a PID by scanning `/proc/*/fd`
symlinks. There is no external process involved. This is why the code comment in
`scanner.go` says:

```go
// Bound address (prefer IPv4 string; IPv6 addresses come from /proc as hex)
address := conn.Laddr.IP
```

That comment is **Linux-specific and misleading on macOS**. On Darwin there is
no `/proc`; `conn.Laddr.IP` is a textual address parsed out of `lsof` output,
which we will see next.

---

## 6. How Discovery Works on macOS (the important part)

This section is the heart of the document. Read it twice.

### 6.1 `net.Connections("inet")` on Darwin shells out to `lsof`

gopsutil picks the implementation by build tag. The Darwin build tag is
`//go:build freebsd || darwin` in:

```
$GOMODCACHE/github.com/shirou/gopsutil/v3@v3.24.5/net/net_unix.go
```

Its `ConnectionsPidWithContext` translates the `kind` argument into `lsof`
arguments. Our code passes `"inet"`, which maps to:

```
lsof -a -n -P -i tcp -i udp
```

The `-a`, `-n`, `-P` prefix comes from
`internal/common/common_unix.go:CallLsofWithContext`; the `-i tcp -i udp` comes
from the `kind` switch:

```go
// net_unix.go
switch strings.ToLower(kind) {
case "inet":
    args = append(args, "tcp", "-i", "udp")   // => -i tcp -i udp
...
}
r, err := common.CallLsofWithContext(ctx, invoke, pid, args...)
```

`CallLsofWithContext` runs `invoke.CommandWithContext(ctx, "lsof", cmd...)`,
i.e. `exec.Command("lsof", ...)`. `exec.Command` resolves `lsof` through
`PATH` *at call time*. On stock macOS `lsof` lives at `/usr/sbin/lsof` and is
not necessarily on a minimal `PATH`.

**Consequences you must remember:**

- If `lsof` is missing → `exec.ErrNotFound` → `ScanListeners` returns an error
  and the whole app shows nothing.
- If `lsof` exists but the process cannot see other users' sockets, those rows
  are simply absent — no error.
- Scanning spawns a **child process** every refresh (default TUI refresh is on
  `r`; auto-refresh is every 3s when enabled).

### 6.2 Each connection line is parsed by `parseNetLine`

Verbatim from `net_unix.go`:

```go
func parseNetLine(line string) (ConnectionStat, error) {
    f := strings.Fields(line)
    if len(f) < 8 {
        return ConnectionStat{}, fmt.Errorf("wrong line,%s", line)
    }
    ...
    pid, _ := strconv.Atoi(f[1])
    fd, _  := strconv.Atoi(strings.Trim(f[3], "u"))
    netFamily, ok := constMap[f[4]]   // IPv4/IPv6
    netType,   ok := constMap[f[7]]   // TCP/UDP
    laddr, raddr, _ = parseNetAddr(f[8])
    n := ConnectionStat{Fd: ..., Family: ..., Type: netType, Laddr: laddr,
                        Raddr: raddr, Pid: int32(pid)}
    if len(f) == 10 {
        n.Status = strings.Trim(f[9], "()")   // "LISTEN"
    }
    return n, nil
}
```

This explains three behaviors we observe:

- **`Type` is set to `syscall.SOCK_STREAM` for TCP**, so the scanner's
  `conn.Type != uint32(syscall.SOCK_STREAM)` filter works identically to Linux.
- **`Status` is only populated when the line has exactly 10 whitespace
  fields.** A listener line like
  `node 3142 manuel 53u IPv6 0x... 0t0 TCP *:5184 (LISTEN)` has 10 fields, so
  `Status == "LISTEN"`. A non-listening socket line has no trailing
  `(LISTEN)` and is filtered out — but only because `Status` stays empty, not
  because of a state field.
- **`Laddr.IP` is now a textual host** (e.g. `127.0.0.1`, `*`, `::1`), because
  `parseNetAddr` calls `net.SplitHostPort` on lsof's `NAME` column.

### 6.3 Process enrichment on macOS: `ps`, `libproc`, and `sysctl`

`pkg/listener/scanner.go` calls, per connection:

| Method | Darwin implementation | What it shells out to / uses |
|--------|----------------------|------------------------------|
| `NewProcess(pid)` | `process_darwin.go` | `sysctl("kern.proc.all")` lookup via `getKProc()` |
| `Name()` | `NameWithContext` | `kinfo_proc.P_comm`; if ≥ 15 bytes falls back to `ps -p PID -o command` |
| `Cmdline()` | `CmdlineWithContext` (nocgo) | `ps -p PID -o command` (`callPsWithContext`) |
| `Exe()` | **cgo**: `proc_pidpath()`; **nocgo**: `lsof -p PID -Fpfn` |
| `Cwd()` | **cgo**: `proc_pidinfo(PROC_PIDVNODEPATHINFO)`; **nocgo**: `ErrNotImplementedError` |
| `Username()` | `UidsWithContext` + user lookup | `sysctl` kinfo + `/etc/passwd`/DirectoryService |
| `Percent(0)` | generic `process.go` | needs two `Times()` samples; first call returns 0 |
| `MemoryInfo()` | `MemoryInfoWithContext` (nocgo) | `ps -p PID -o rss,vsize,pagein` |
| `CreateTime()` | `createTimeWithContext` | `kinfo_proc.P_starttime` |

The two Darwin files that select this behavior are:

```
process/process_darwin_cgo.go    //go:build darwin && cgo
process/process_darwin_nocgo.go  //go:build darwin && !cgo
```

**This cgo/nocgo split is the single most important macOS fact for a porter.**
When you build on a Mac normally, `CGO_ENABLED=1` by default, so `Exe` and
`Cwd` use Apple's `libproc`. But when you **cross-compile from Linux to
Darwin** (a very common release-workflow choice), `CGO_ENABLED=0` is forced,
and then `Cwd` is silently never available. The value of `Cwd` in JSON becomes
`""` with no error surfaced to the user.

`Cwd` with cgo is also permission-bound. The source comment is explicit:

```go
// CwdWithContext retrieves the Current Working Directory for the given process.
// It uses proc_pidinfo from libproc and will only work for processes the
// EUID can access. Otherwise "operation not permitted" will be returned.
```

And the error is mapped to a sentinel:

```go
if err == syscall.EPERM {
    return "", ErrorNotPermitted
}
```

Because `scanner.go` does `cwd, _ := proc.Cwd()` and throws the error away, you
cannot tell "this process has no cwd" from "macOS refused to tell us the cwd".

### 6.4 Visualizing the two stacks

```
                    LINUX                                 DARWIN (macOS)
   ┌──────────────────────────────┐        ┌──────────────────────────────────┐
   │ gopsutil/net.Connections     │        │ gopsutil/net.Connections         │
   │  read /proc/net/tcp{,6}      │        │  exec lsof -a -n -P -i tcp -i udp│
   │  inode → PID via /proc/*/fd  │        │  parse NAME col with             │
   └───────────────┬──────────────┘        │  net.SplitHostPort               │
                   │                       └───────────────┬──────────────────┘
                   ▼                                       ▼
   ┌──────────────────────────────┐        ┌──────────────────────────────────┐
   │ process (per conn)           │        │ process (per conn)               │
   │  Name/Exe/Cwd  ← /proc/PID   │        │  Name/Cmd/User ← sysctl + ps     │
   │  RSS           ← /proc/PID   │        │  Exe/Cwd       ← libproc (cgo)   │
   │  Percent(0)    ← /proc stat  │        │                or lsof (nocgo)   │
   └──────────────────────────────┘        │  RSS           ← ps              │
                                            │  Percent(0)    ← needs 2 samples │
                                            └──────────────────────────────────┘
                 direct kernel reads                     subprocess + syscall
```

---

## 7. Runtime Evidence from a Real macOS Box

The app **builds and runs** on macOS today:

```
$ go version
go version go1.27.1 darwin/arm64
$ uname -a
Darwin manuel.local 24.6.0 ... RELEASE_ARM64_T6000 arm64

$ go build -o /tmp/listen-killer ./cmd/listen-killer/   # BUILD_OK
```

A trimmed real `list --output json` row:

```json
{
  "address": "*",
  "cmdline": "node ./node_modules/.bin/../vite/bin/vite.js --port 5184 --strictPort",
  "cpu_percent": "",
  "cwd": "/Users/manuel.odendahl/code/tulip/playback/.worktrees/canvas-demo-director/app",
  "exe": "/opt/homebrew/Cellar/node/26.8.1/bin/node",
  "name": "node",
  "pid": 3142,
  "port": 5184,
  "protocol": "TCP",
  "rss_bytes": 41467904,
  "rss_human": "39.5 MB",
  "uptime": "19h36m",
  "uptime_seconds": 70564,
  "username": "manuel.odendahl"
}
```

And `kill --dry-run` resolves targets correctly:

```json
{
  "addresses": "*", "dry_run": true, "error": "", "killed": false,
  "matched_by": "port", "name": "node", "pid": 3142, "ports": "5184",
  "signal": "TERM", "username": "manuel.odendahl"
}
```

Raw `lsof` for orientation (this is literally what gopsutil parses):

```
COMMAND     PID            USER   FD   TYPE  ...  NODE NAME
jetbrains  1473 manuel.odendahl  510u  IPv6  ...  TCP 127.0.0.1:49896 (LISTEN)
node       3142 manuel.odendahl   53u  IPv6  ...  TCP *:5184 (LISTEN)
com.docke  6405 manuel.odendahl    5u  IPv4  ...  TCP 127.0.0.1:5432 (LISTEN)
```

Timing on this machine (fast path, 19 listeners):

```
listen-killer list --output json   →  real 0.11s   (3 runs, stable)
lsof -a -n -P -i tcp -i udp        →  real 0.06s
```

So performance is acceptable here, but remember each refresh re-runs `lsof`
**plus** a `ps`/`sysctl` probe per connection. On a busy CI mac with hundreds
of open sockets this grows linearly.

Two immediate observations from the evidence:

1. `"cpu_percent": ""` — CPU is always empty (gap G3).
2. `"address": "*"` — wildcard bind is preserved from lsof and normalized later
   by `normalizeAddress`; `::1` appears for IPv6 loopback. This is fine, but
   the `scanner.go` comment about `/proc` hex is wrong for macOS.

---

## 8. The macOS Gap Register — What Actually Breaks

Each gap has an ID, a symptom, a root cause with a source citation, and a
suggested fix. Prioritize G1–G4; G5–G9 are correctness/polish/packaging.

### G1 — Hard dependency on the `lsof` binary (correctness, high)

- **Symptom:** On a machine where `lsof` is not on `PATH`, `ScanListeners`
  returns `exec.ErrNotFound`; `list` errors out and the TUI shows
  "Scanning..." forever / an error status.
- **Root cause:** `net_unix.go` calls `exec.Command("lsof", ...)` via
  `exec.LookPath`, which honors `$PATH`. macOS ships `lsof` at
  `/usr/sbin/lsof`, which is on a normal login shell's `PATH` but **not**
  guaranteed for GUI-launched or minimal-environment processes.
- **Fix options:**
  - Resolve `lsof` explicitly: prefer `/usr/sbin/lsof`, then `$PATH`.
  - Detect the error and surface a clear, actionable message ("install/repair
    lsof or grant Full Disk Access").
  - Longer term, replace with a native implementation (G-native).

### G2 — Other users' sockets are invisible (permissions, high)

- **Symptom:** Running without elevated privileges or Full Disk Access shows
  only your own processes' listeners. Root-owned daemons (e.g. some system
  services) do not appear at all.
- **Root cause:** `lsof` must be able to read the file descriptors/socket
  tables of the target processes. On modern macOS this is also gated by the
  **Privacy & Security → Full Disk Access** TCC permission for the *terminal
  application* (or the binary, if launched standalone).
- **Fix options:**
  - Document the permission requirement prominently (this is the #1 support
    question for any macOS sysinfo tool).
  - Optionally detect an "all processes" run mode and suggest granting FDA or
    running with `sudo` (note: `sudo` changes the *owning* semantics; the app
    would report sockets owned by root).
  - Never silently pretend the list is complete; show a hint when the count is
    suspiciously low or when `lsof` stderr indicates permission errors.

### G3 — `cpu_percent` is always empty/zero (correctness, high)

- **Symptom:** `"cpu_percent": ""` in every row, on every refresh, on macOS
  (and in fact on Linux too).
- **Root cause:** `scanner.go` calls `proc.Percent(0)`. The generic
  implementation in `process/process.go` returns `0` on the **first** call for
  a given `*Process` because it has no previous sample:

  ```go
  } else { // interval == 0
      if p.lastCPUTimes == nil {
          p.lastCPUTimes = cpuTimes
          p.lastCPUTime = now
          return 0, nil          // ← always taken on a fresh Process
      }
  }
  ```

  Because `ScanListeners` constructs a **new** `process.NewProcess(pid)` on
  every scan, the cache is always empty and CPU is always 0.
- **Fix:** sample twice with a small interval (`proc.Percent(interval)`), or
  keep a `map[pid]{cpuTimes,time}` cache in the scanner and compute the delta
  between successive scans. The first approach is simpler but blocks the scan
  for the interval; the second is non-blocking but produces a value only from
  the second scan onward. For a TUI that auto-refreshes every 3s, the cache
  approach is ideal: keep the previous sample across refreshes.

  ```text
  // cache-based CPU sampling (recommended)
  scanner holds: lastSample map[int32]{times cpu.TimesStat, at time.Time}
  FOR EACH pid:
      cur := Times(pid)
      prev, ok := lastSample[pid]
      IF ok AND cur.at > prev.at:
          cpu = (cur.total - prev.total) / (cur.at - prev.at).Seconds() * 100 / ncpu
      lastSample[pid] = cur
  ```

### G4 — `cwd` disappears without cgo, and is permission-bound with cgo (correctness, medium)

- **Symptom:** Cross-compiled (`CGO_ENABLED=0`) macOS binaries always report
  `cwd: ""`. Natively built binaries report `cwd` only for processes the user
  owns (EPERM otherwise).
- **Root cause:** `process_darwin_nocgo.go`:

  ```go
  //go:build darwin && !cgo
  func (p *Process) CwdWithContext(ctx context.Context) (string, error) {
      return "", common.ErrNotImplementedError
  }
  ```

  With cgo, `process_darwin_cgo.go` uses `proc_pidinfo(PROC_PIDVNODEPATHINFO)`
  which returns `syscall.EPERM` for processes outside the EUID's reach.
- **Fix options:**
  - Decide the release build policy: publish a **cgo** arm64/amd64 binary for
    Darwin (recommended; `CGO_ENABLED=1` with the native toolchain) so `cwd`
    and `exe` are accurate.
  - Add a fallback chain: `libproc → lsof -p PID -a -d cwd -Fn → empty`, and
    distinguish "unsupported/denied" from "genuinely empty" in the UI.

### G5 — Address presentation differs from Linux (correctness, low)

- **Symptom:** Addresses can be `*`, `127.0.0.1`, `::1`, `[::1]`-derived, etc.
- **Root cause:** lsof prints textual hosts; gopsutil parses them with
  `net.SplitHostPort`. Linux gets hex from `/proc` and formats differently.
- **Fix:** Keep `normalizeAddress` (`""`/`"::"` → `"*"`) and make the browser
  URL builder (`listenerURL`) strip IPv6 brackets and map wildcards to
  `127.0.0.1`. That logic already exists; just audit it against real rows
  (`::1` must become `http://[::1]:port` or `http://127.0.0.1:port` — currently
  it produces `http://::1:port`, which is invalid). **This is a real bug on
  IPv6 loopback and should be fixed during the port.**

### G6 — Kill semantics and permission (correctness, medium)

- **Symptom:** `kill` fails with `operation not permitted` for processes owned
  by another user or protected by SIP; succeeds for your own processes.
- **Root cause:** `syscall.Kill(pid, sig)` is correct and portable; the OS
  enforces ownership/SIP. There is no code bug here.
- **Fix:** Document that `TERM`/`KILL`/`INT` behave identically on Darwin, that
  SIP-protected processes cannot be killed, and that PID reuse is a race
  (resolve → signal is not atomic). Consider re-validating that the PID still
  owns the port immediately before signalling.

### G7 — "Open in browser" is already ported (no action, informational)

- **Status:** correct. `open` on Darwin is the analogue of `xdg-open`.

### G8 — Build, signing, notarization, architecture (packaging, high for release)

- **Symptom:** A downloaded unsigned binary is quarantined by Gatekeeper and
  refuses to run; a `linux/amd64` CI cannot produce a fully-featured Darwin
  binary.
- **Root cause:** macOS Gatekeeper + notarization; cgo cross-compilation
  limitations (G4).
- **Fix:** Build Darwin artifacts on a Darwin runner (GitHub Actions
  `macos-latest`) with `CGO_ENABLED=1`; ad-hoc codesign for local use; Developer
  ID + notarization for public distribution; publish both `arm64` and `amd64`.
  See Section 14.

### G9 — TUI/terminal behavior (validation, low)

- **Status:** Bubbletea works on macOS Terminal.app and iTerm2. `WithAltScreen`
  is supported. Emoji (🎧, ●) render fine. Validate on a fresh terminal and on
  a small window; the detail pane height math (`m.height - 5 - 12`) is
  OS-independent but worth re-checking on retina/large fonts.

### Gap summary table

| ID | Area | Severity | Root cause file |
|----|------|----------|-----------------|
| G1 | lsof dependency | high | `net_unix.go` / `common_unix.go` |
| G2 | permission/FDA | high | OS (TCC), `lsof` |
| G3 | cpu_percent | high | `process/process.go` + `scanner.go` |
| G4 | cwd/exe cgo split | medium | `process_darwin_{cgo,nocgo}.go` |
| G5 | IPv6 URL bug | low/med | `pkg/tui/model.go:listenerURL` |
| G6 | kill permission/SIP | medium | OS (no code bug) |
| G7 | browser | none | already correct |
| G8 | packaging/signing | high (release) | build/release config |
| G9 | TUI rendering | low | validate only |

---

## 9. Design: A Platform Abstraction for the Scanner

### 9.1 Problem

`ScanListeners` currently mixes three concerns: (1) enumerating sockets,
(2) enriching with process data, and (3) Darwin/Linux quirks. We want to keep
one portable `[]ListenerInfo` output while allowing platform-specific
implementations and testability without a Mac.

### 9.2 Proposed package layout

Do **not** fork `pkg/listener` into `listener_linux.go` /
`listener_darwin.go` wholesale — that duplicates `gatherProcessInfo` and
`ListenerInfo` assembly. Instead, isolate only the truly platform-varying
pieces behind small interfaces, with `_darwin.go` / `_linux.go` build-tagged
files providing the implementations.

```
pkg/listener/
├── types.go              unchanged (ListenerInfo, formatting)
├── process.go            unchanged (syscall.Kill) — verify on darwin
├── scanner.go            portable orchestration ONLY
├── socket_source.go      interface SocketSource { List() ([]RawSocket, error) }
├── socket_source_darwin.go   //go:build darwin   (lsof-backed, explicit path)
├── socket_source_linux.go    //go:build linux    (gopsutil /proc-backed)
├── process_source.go     interface ProcessSource { Enrich(pid) (ProcessFacts, error) }
├── process_source_darwin.go  //go:build darwin   (libproc/ps + cwd fallback)
├── process_source_linux.go   //go:build linux    (gopsutil)
└── cpu_sampler.go        portable delta-based CPU sampling cache
```

The orchestrator becomes platform-free:

```go
type RawSocket struct {
    PID     int32
    Port    uint32
    Address string
    Proto   string
}

func ScanListeners() ([]ListenerInfo, error) {
    socks, err := newSocketSource().List()
    if err != nil { return nil, err }

    ps := newProcessSource()
    cpu := newCPUSampler()

    var out []ListenerInfo
    for _, s := range socks {
        facts, err := ps.Enrich(s.PID)
        if err != nil { continue }
        out = append(out, ListenerInfo{
            PID: s.PID, Port: s.Port, Address: s.Address, Protocol: s.Proto,
            Name: facts.Name, Cmdline: facts.Cmdline, Exe: facts.Exe,
            Cwd: facts.Cwd, Username: facts.Username,
            Uptime: ..., UptimeSeconds: ..., CPUPercent: cpu.Observe(s.PID),
            RSSBytes: facts.RSS, RSSHuman: FormatBytes(facts.RSS),
        })
    }
    return out, nil
}
```

### 9.3 Darwin socket source: control `lsof` explicitly

Rather than relying on gopsutil's implicit `exec.Command("lsof")`, own the
lookup so we can (a) pin `/usr/sbin/lsof`, (b) parse stderr for permission
hints, and (c) produce a friendly error. Keep the same field parsing contract
that `parseNetLine` uses.

```text
FUNCTION DarwinSocketSource.List() -> []RawSocket:
    lsof := resolveLsof()            # /usr/sbin/lsof, else LookPath("lsof")
    IF lsof == "": RETURN error("lsof not found; install Xcode CLT or grant FDA")

    out := run(lsof, "-a","-n","-P","-i","tcp","-i","udp")
    rows := []
    FOR EACH line AFTER header:
        f := Fields(line)
        IF len(f) < 8: CONTINUE
        IF f[7] != "TCP": CONTINUE                 # keep TCP only
        pid := Atoi(f[1])
        host, port := SplitHostPort(f[8])          # "127.0.0.1:49896", "*:5184"
        IF len(f) == 10 AND Trim(f[9],"()") == "LISTEN":
            rows = APPEND(RawSocket{PID: pid, Port: port, Address: host, Proto: "TCP"})
    RETURN rows
```

This duplicates ~30 lines of gopsutil logic but buys full control and
testability. **Alternative (lower effort):** keep calling
`net.Connections("inet")` and only wrap it to add lsof-path detection and
better error messages.

### 9.4 Darwin process source: facts with explicit degradation

```text
STRUCT ProcessFacts { Name, Cmdline, Exe, Cwd, Username string; RSS uint64; CreateMS int64 }
STRUCT ProcErr { Field string; Kind enum{Denied,Unsupported,NotFound} }

FUNCTION DarwinProcessSource.Enrich(pid) -> (ProcessFacts, error):
    p, err := process.NewProcess(pid); IF err: RETURN err
    f.Name, _  = p.Name()
    f.Cmdline,_= p.Cmdline()
    f.Exe, errExe := p.Exe()
    f.Cwd, errCwd := p.Cwd()          # may be ErrNotImplementedError or EPERM
    f.Username,_ = p.Username()
    f.CreateMS,_ = p.CreateTime()
    mem,_ := p.MemoryInfo(); IF mem: f.RSS = mem.RSS

    # Explicitly classify degradation instead of discarding it:
    IF errCwd != nil: RECORD ProcErr{Field:"cwd", Kind: classify(errCwd)}
    IF errExe != nil: RECORD ProcErr{Field:"exe", Kind: classify(errExe)}
    RETURN f, nil
```

`classify` maps `ErrNotImplementedError` → `Unsupported`,
`ErrorNotPermitted`/`EPERM` → `Denied`. The TUI can then show `cwd: (denied)`
or `cwd: (needs cgo build)` instead of a blank, which is the difference between
a mysterious bug and a documented limitation.

### 9.5 CPU sampler (portable, fixes G3 everywhere)

```text
STRUCT sample { total float64; wall time.Time }
CPUSampler := map[int32]sample

FUNCTION (c *CPUSampler) Observe(p *Process) float64:
    ts, err := p.Times(); IF err: RETURN 0
    cur := sample{total: ts.User + ts.System, wall: now()}
    prev, ok := c.m[pid]; c.m[pid] = cur
    IF NOT ok: RETURN 0                       # first observation
    dt := cur.wall.Sub(prev.wall).Seconds(); IF dt <= 0: RETURN 0
    RETURN (cur.total - prev.total) / dt * 100 / NumCPU()
```

Reset entries for PIDs that disappeared each scan to bound memory.

### 9.6 Testability

With `SocketSource`/`ProcessSource` as interfaces you can write a fake source
that returns canned sockets, so `scanner_test.go` runs on Linux CI and
validates the whole pipeline without a Mac. Darwin-specific parsers
(`parseLsofLine`) get table-driven unit tests using **text fixtures** captured
from a real Mac — this is why you must not let the `lsof` call be buried inside
gopsutil.

---

## 10. Alternatives Considered

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| **A. Keep gopsutil + lsof as-is; only document limits** | Zero code, works today | G3/G4/G5 remain silent bugs; G1 fragile; not really a "port" | Insufficient |
| **B. Native `libproc`/`sysctl` only (no lsof)** | No external binary; fast; accurate `cwd`/`exe` | Requires cgo; no socket→PID enumeration without a helper; more code; loses Linux parity | Too much for v1; revisit |
| **C. Own the `lsof` invocation + portable interfaces (recommended)** | Full control, friendly errors, testable, keeps gopsutil process helpers | ~150 LOC new code; still depends on lsof | **Do this** |
| **D. Shell out to `lsof`/`netstat` + `ps` entirely, drop gopsutil** | Minimal deps; explicit | Re-implements process parsing; brittle across macOS versions | Not now |
| **E. Use `sudo` internally** | Sees more processes | Terrible UX; security; changes ownership semantics | Reject |

Recommended path: **C**, with a documented, deferred option to migrate the
Darwin socket source to native `libproc` (B) once v1 is stable and signed.

---

## 11. Implementation Plan (Phased)

Each phase is independently shippable and testable. Update the ticket
`tasks.md`, and keep the diary current at each major boundary.

### Phase 0 — Baseline and harness (0.5 day)

- [ ] Confirm `go build ./...` and `go vet ./...` on `darwin/arm64` and
  `linux/amd64`.
- [ ] Capture a real macOS fixture: `lsof -a -n -P -i tcp -i udp > testdata/lsof_tcp.txt`.
- [ ] Add `pkg/listener/scanner_test.go` skeleton with a fake `SocketSource`.

### Phase 1 — Platform abstraction (1–2 days)

- [ ] Introduce `RawSocket`, `SocketSource`, `ProcessSource`, `ProcessFacts`.
- [ ] Implement `socket_source_linux.go` using current gopsutil code.
- [ ] Implement `socket_source_darwin.go` using explicit `lsof` lookup +
  `parseLsofLine` (unit-tested against the fixture).
- [ ] Rewrite `scanner.go` to pure orchestration; keep `ListenerInfo` identical.
- [ ] Verify byte-for-byte JSON parity on Linux before/after.

### Phase 2 — Fix CPU sampling (G3) (0.5 day)

- [ ] Add `cpu_sampler.go`; wire into orchestration; prune vanished PIDs.
- [ ] Verify `cpu_percent` becomes non-empty after the second scan in the TUI.

### Phase 3 — Degradation visibility and IPv6 URL (G4, G5) (1 day)

- [ ] Classify `cwd`/`exe` errors; surface "(denied)"/"(unsupported)" in the
  detail pane and CLI.
- [ ] Fix `listenerURL` for IPv6: bracket `::1` → `http://[::1]:port`;
  document wildcard→`127.0.0.1`.
- [ ] Unit-test `listenerURL` for `*`, `0.0.0.0`, `::`, `::1`, `127.0.0.1`.

### Phase 4 — Errors, permissions, docs (G1, G2) (0.5 day)

- [ ] Friendly message when `lsof` is missing/denied.
- [ ] README "macOS notes" section: lsof, Full Disk Access, SIP, statically
  built binaries lose `cwd`.
- [ ] Add a `doctor`-style check (optional): print resolved lsof path + whether
  FDA appears granted.

### Phase 5 — Packaging (G8) (1 day)

- [ ] GoReleaser/Makefile: build Darwin `arm64`+`amd64` on a macOS runner with
  `CGO_ENABLED=1`; ad-hoc sign; document notarization path.
- [ ] Homebrew tap formula or `go install` instructions.

### Definition of done

- `listen-killer list` on macOS reports `cwd`, `exe`, `cpu_percent`, and IPv6
  URL correctly for user-owned processes, and clearly labels degraded fields
  for the rest.
- Linux output is unchanged (regression test).
- A downloaded Darwin binary runs without Gatekeeper blocking it (or the
  README documents the ad-hoc workaround).

---

## 12. API Reference

### 12.1 Our own API (must stay stable)

| Function | File | Signature / contract |
|----------|------|----------------------|
| `listener.ScanListeners` | `pkg/listener/scanner.go` | `() ([]ListenerInfo, error)` — portable output |
| `listener.KillProcess` | `pkg/listener/process.go` | `(pid int32, signal string) error` — `TERM`/`KILL`/`INT` |
| `listener.FormatUptime` | `pkg/listener/types.go` | `(createTimeMS int64) (string, int64)` |
| `listener.FormatBytes` | `pkg/listener/types.go` | `(bytes uint64) string` |
| `tui.NewModel` / `Model.Update` / `Model.View` | `pkg/tui/*.go` | Bubbletea contract |
| `listenerURL` | `pkg/tui/model.go` | `(*ListenerInfo) string` — **fix IPv6** |

### 12.2 gopsutil (Darwin-relevant)

| Symbol | Darwin behavior | Source file |
|--------|-----------------|-------------|
| `net.Connections("inet")` | exec `lsof -a -n -P -i tcp -i udp` | `net/net_unix.go` |
| `net.Connections("tcp")` | exec `lsof ... -i tcp` (fewer rows, faster) | `net/net_unix.go` |
| `ConnectionStat.Type` | `SOCK_STREAM` for TCP | `net/net_unix.go:constMap` |
| `ConnectionStat.Status` | set only when lsof line has 10 fields | `net/net_unix.go:parseNetLine` |
| `process.NewProcess` | `sysctl kern.proc.all` lookup | `process/process_darwin.go` |
| `Process.Exe` | cgo `proc_pidpath`; nocgo `lsof -p PID -Fpfn` | `process_darwin_{cgo,nocgo}.go` |
| `Process.Cwd` | cgo `proc_pidinfo(PROC_PIDVNODEPATHINFO)`; nocgo unsupported | `process_darwin_{cgo,nocgo}.go` |
| `Process.MemoryInfo` (nocgo) | `ps -p PID -o rss,vsize,pagein` | `process_darwin_nocgo.go` |
| `Process.Percent(0)` | generic; 0 on first call per `*Process` | `process/process.go` |

### 12.3 macOS / Unix APIs

| API | Use | Notes |
|-----|-----|-------|
| `syscall.Kill(pid, sig)` | send signals | identical semantics to Linux; `SIGTERM=15`, `SIGKILL=9`, `SIGINT=2` |
| `proc_pidpath` (libproc) | executable path | cgo; `PROC_PIDPATHINFO_MAXSIZE` buffer |
| `proc_pidinfo(PROC_PIDVNODEPATHINFO)` | cwd | cgo; `EPERM` for other users |
| `sysctl kern.proc.all` | process table | via `golang.org/x/sys/unix.SysctlKinfoProcSlice` |
| `lsof -a -n -P -i tcp` | socket enumeration | `-a` AND, `-n` no DNS, `-P` no port names |
| `open <url>` | browser | already used |

### 12.4 Environment variables that matter

| Var | Effect on macOS build |
|-----|-----------------------|
| `CGO_ENABLED=1` | enables libproc `Exe`/`Cwd` (default for native builds) |
| `CGO_ENABLED=0` | disables cgo → `Cwd` unsupported, `Exe` via lsof |
| `GOOS=darwin`, `GOARCH=arm64\|amd64` | target platform |
| `PATH` | must contain `/usr/sbin` for `lsof` resolution |
| `TCC` (Full Disk Access) | gates visibility of other users' processes |

---

## 13. Testing and Verification Playbook

```bash
# 1. Build for the host and for both Darwin arches
go build ./...
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o /tmp/lk-arm64 ./cmd/listen-killer/
GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -o /tmp/lk-amd64 ./cmd/listen-killer/

# 2. Confirm the raw discovery layer
/usr/sbin/lsof -a -n -P -i tcp -i udp | awk 'NR==1 || /LISTEN/'

# 3. Compare our JSON against raw lsof
/tmp/lk-arm64 list --output json | jq -r '.[] | "\(.pid) \(.name) \(.port) \(.address) cwd=\(.cwd) cpu=\(.cpu_percent)"'

# 4. Verify the IPv6 URL fix (loopback listener)
/tmp/lk-arm64 list --output json | jq -r '.[] | select(.address=="::1") | "http://[\(.address)]:\(.port)"'

# 5. Kill safety
/tmp/lk-arm64 kill --port 5184 --dry-run --output json
# never run --yes against a real service you care about during testing

# 6. Linux regression (in CI)
GOOS=linux GOARCH=amd64 go build ./...
go test ./...
```

Manual checks on macOS:

1. Launch the TUI in iTerm2 and Terminal.app; verify table, detail pane, `o`,
   `d`, `r`, mark/kill dialog.
2. Run with auto-refresh on (`a`) and confirm `cpu_percent` populates on the
   second tick (after the Phase 2 fix).
3. Run without Full Disk Access and confirm the app still lists your own
   processes; then grant FDA and confirm the count increases.

---

## 14. Packaging, Signing and Distribution

This is where "it works on my Mac" becomes "it works on other people's Macs".

- **Build on Darwin.** Cross-compiling from Linux forces `CGO_ENABLED=0`,
  which drops `cwd` (G4). Use a `macos-latest` GitHub Actions runner.
- **Two architectures.** Apple Silicon (`arm64`) and Intel (`amd64`). Ship
  both, or a universal binary via `lipo`.
- **Codesigning.** For local use an ad-hoc signature (`codesign -s -`) stops
  the "damaged/can't be opened" dialog. For public distribution you need a
  Developer ID certificate and **notarization** (`notarytool`), then staple
  the ticket.
- **Quarantine.** Downloaded binaries get the `com.apple.quarantine` xattr;
  Gatekeeper blocks unsigned ones. The README should tell users the ad-hoc
  workaround: `xattr -d com.apple.quarantine ./listen-killer`.
- **Homebrew.** A tap formula or core formula is the friendliest install path;
  it also handles the architecture selection for the user.
- **`go install`.** Still works for developers and needs no signing.

Sketch of a release matrix:

```yaml
# .github/workflows/release.yml (sketch)
jobs:
  darwin:
    runs-on: macos-latest
    env: { CGO_ENABLED: "1" }
    steps:
      - uses: actions/checkout@v4
      - run: |
          go build -o dist/lk-darwin-arm64 ./cmd/listen-killer/
          GOARCH=amd64 go build -o dist/lk-darwin-amd64 ./cmd/listen-killer/
      - run: |
          for b in dist/lk-darwin-*; do codesign --force -s - "$b"; done
          # notarization would follow here for public releases
```

---

## 15. Glossary

| Term | Meaning |
|------|---------|
| **Listener** | A TCP socket in the `LISTEN` state, waiting for connections. |
| **`lsof`** | "list open files"; on macOS also lists sockets and maps them to PIDs. |
| **`libproc`** | Apple's C library (`proc_pidinfo`, `proc_pidpath`) for process introspection. |
| **`kinfo_proc`** | The BSD struct returned by `sysctl` describing one process. |
| **cgo** | Go's C interop; required for `libproc`-based `cwd`/`exe`. |
| **TCC / Full Disk Access** | macOS privacy gate; without it, other users' process info is hidden. |
| **SIP** | System Integrity Protection; prevents killing/modifying protected processes. |
| **Gatekeeper / notarization** | macOS code-signing and review pipeline for downloaded apps. |
| **Bubbletea** | Charmbracelet's Elm-architecture TUI framework. |
| **Glazed** | go-go-golems framework for structured/scriptable CLI output. |
| **Elm architecture** | Model → Update(msg) → (Model, Cmd) → View() loop. |

---

## 16. Open Questions

1. **Build policy:** Do we ship cgo Darwin binaries only (accurate `cwd`) or
   also offer a `CGO_ENABLED=0` fallback for portability? Recommendation: cgo
   primary, nocgo as an explicitly-labeled "lite" build.
2. **Elevation:** Should we support a `--all-users` mode that re-runs `lsof`
   with `sudo`, clearly labeling that it reports root-owned sockets?
3. **Native socket source:** Is it worth replacing `lsof` with a native
   `libproc` + `sysctl` socket enumeration in v2? (Depends on whether lsof
   remains shipped in future macOS releases and on performance at scale.)
4. **Degradation UX:** Show degraded fields inline (`cwd: (denied)`) or hide
   them and add a banner? Recommendation: inline, so the data model stays
   complete and scripts can detect the sentinel.
5. **PID-reuse race:** Should `kill` re-verify port ownership immediately
   before signalling? Recommendation: yes, cheap insurance.

---

## References

- Repository: `github.com/wesen/listen-killer` (revision `ef03a2c`).
- gopsutil: `github.com/shirou/gopsutil/v3@v3.24.5` — especially
  `net/net_unix.go`, `internal/common/common_unix.go`,
  `process/process_darwin.go`, `process_darwin_cgo.go`,
  `process_darwin_nocgo.go`, `process/process.go`.
- Bubbletea: `github.com/charmbracelet/bubbletea` v1.3.10.
- Glazed: `github.com/go-go-golems/glazed` v1.2.7.
- Apple `libproc(3)`, `sysctl(3)`, `lsof(8)` man pages.
- Companion docs (original ticket `LISTENKILL`):
  - `ttmp/2026/05/07/LISTENKILL--.../design/01-intern-guide-architecture-and-implementation.md`
  - `ttmp/2026/05/07/LISTENKILL--.../analysis/01-system-analysis-and-requirements.md`
