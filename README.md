# 🎧 Listen Killer

A terminal UI for discovering, inspecting, and killing TCP listeners on your machine.

Every dev machine accumulates stale daemons — dev servers, database proxies, API mocks, editor plugins — all quietly holding ports open. Listen Killer shows them all in one place and lets you kill them instantly.

## Screenshots

### Main View — Table + Detail Pane

<img src="screenshots/01-main-view.png" width="700" alt="Main view showing listener table with detail pane below">

The TUI view shows a scrollable table of all TCP listeners with a detail pane below. Navigate with `j`/`k`; the detail pane updates as you move.

### Multi-Mark — Selecting Processes

<img src="screenshots/02-with-marks.png" width="700" alt="Table with three rows marked using spacebar">

Press `space` to mark rows (shown with `●`). Marked processes can be bulk-killed together. The title bar shows the count: `7 listeners  3 marked`.

### Kill Confirmation — Bulk Kill

<img src="screenshots/03-kill-dialog.png" width="700" alt="Kill dialog showing 3 marked processes with signal selector">

Press `K` to open the kill dialog. If processes are marked, all of them are targeted. Choose a signal (TERM/KILL/INT) with `↑↓`, confirm with `y`, cancel with `n`.

### Table Only — Detail Pane Toggled Off

<img src="screenshots/04-table-only.png" width="700" alt="Full-height table with detail pane hidden">

Press `d` to toggle the detail pane. Without it, the table gets the full terminal height.

## Installation

```bash
go install github.com/wesen/listen-killer/cmd/listen-killer@latest
```

Or build from source:

```bash
git clone https://github.com/wesen/listen-killer.git
cd listen-killer
go build -o ~/bin/listen-killer ./cmd/listen-killer/
```

### macOS notes

On macOS there is no `/proc`, so Listen Killer discovers listening sockets by
shelling out to `lsof` (`/usr/sbin/lsof`) and reads process details via
`sysctl`/`libproc`/`ps` (through gopsutil). Three consequences worth knowing:

- **Permissions.** Without elevated privileges or *Full Disk Access*
  (System Settings → Privacy & Security → Full Disk Access) for your terminal,
  `lsof` can only see sockets owned by your user. Grant Full Disk Access to see
  root-owned/system listeners.
- **Working directory.** `cwd` is read via Apple's `libproc`, which requires a
  cgo build (`CGO_ENABLED=1`, the default for native macOS builds). A binary
  cross-compiled with `CGO_ENABLED=0` reports `cwd` as unsupported; the app
  now says so in a `warnings` field instead of leaving it blank.
- **SIP.** System Integrity Protection prevents signalling protected processes;
  `kill` will report `operation not permitted` for those.

```bash
# Build a fully-featured Darwin binary (keeps cwd/exe via libproc)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o listen-killer ./cmd/listen-killer/
```

For local use, ad-hoc signing avoids the Gatekeeper "damaged or can't be
opened" dialog:

```bash
make build-darwin              # dist/listen-killer-darwin-{arm64,amd64}, ad-hoc signed
xattr -d com.apple.quarantine ./listen-killer   # if downloaded via a browser
```

Public distribution additionally needs a Developer ID certificate and
notarization (`codesign` + `notarytool`); `make build-darwin` builds and
ad-hoc-signs only.

## Usage

### CLI / Scripting / Agent Mode (default)

```bash
listen-killer                                           # readable Markdown, one section per process
listen-killer list                                      # readable Markdown, one section per process
listen-killer list --output json                        # JSON output
listen-killer list --output yaml                        # YAML output
listen-killer list --output csv                         # CSV output
listen-killer list --fields pid,name,port,cwd           # select columns
listen-killer list --port 3000 --output json            # filter by port
listen-killer list --name node --fields pid,port,cwd,cmdline
listen-killer list --cpu-sample 500ms --output json      # include cpu_percent
```

`cpu_percent` is a rate, so it needs two readings. In the TUI it appears from
the second auto-refresh; for a one-shot CLI run, pass `--cpu-sample <duration>`
(e.g. `500ms`) to sample over an interval. The `cpu_sampled` field tells you
whether a value was actually measured. Rows may also carry a `warnings` field
when a field could not be read (for example `cwd: permission denied`).

### Interactive TUI

```bash
listen-killer tui       # launches the TUI dashboard
```

### Scriptable Kill Verb

```bash
listen-killer kill --port 3000 --dry-run --output json   # resolve targets safely
listen-killer kill --port 3000 --signal TERM --yes       # graceful stop
listen-killer kill 1234 pid:5678 :8080 --signal KILL --yes --output json
listen-killer kill --name node --path /path/to/project --signal TERM --yes
```

`kill` emits one structured result row per target PID with `pid`, `name`, `ports`, `addresses`, `exe`, `cwd`, `cmdline`, `signal`, `dry_run`, `killed`, `matched_by`, and `error`. Destructive execution requires `--yes`; use `--dry-run` for agent planning and review.

## Key Bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `space` | Mark/unmark current row |
| `M` | Clear all marks |
| `K` | Kill selected (or all marked) |
| `o` | Open `http://host:port` in browser |
| `d` | Toggle detail pane |
| `r` | Refresh listener list |
| `a` | Toggle auto-refresh (3s) |
| `?` | Toggle full help |
| `q` / `ctrl+c` | Quit |

### Kill Dialog Keys

| Key | Action |
|-----|--------|
| `↑` / `↓` | Change signal (TERM → KILL → INT) |
| `y` / `enter` | Confirm kill |
| `n` / `esc` | Cancel |

## Features

### Multi-Mark

Press `space` on any row to mark it (shown with `●`). The cursor auto-advances down for rapid marking. Press `K` when marks exist to kill all marked processes at once. Clear marks with `M`.

### Detail Pane

A compact detail view below the table shows all metadata for the selected process: PID, name, full command line, binary path, user, port, bind address, uptime, CPU%, memory, and the computed browser URL. It auto-updates as you navigate.

### Open in Browser

Press `o` to open the selected listener in your default browser (`xdg-open` on Linux, `open` on macOS). Bind addresses like `0.0.0.0`, `*`, or `::` are normalized to `127.0.0.1`.

### CLI Tooling Layer

The same data is available as structured output via the Glazed command framework. The output includes the owning process, bind address, port, executable path, working directory (`cwd`), command line, uptime, CPU%, and memory usage:

```bash
# Readable Markdown (default): one section per listening process
listen-killer list

# JSON for scripting
listen-killer list --output json | jq '.[] | select(.port > 10000)'

# CSV for spreadsheets
listen-killer list --output csv

# Custom columns
listen-killer list --fields pid,name,port,cwd,cmdline,uptime,rss_human

# Filters for scripts / agents
listen-killer list --port 5173 --output json
listen-killer list --name node --path my-project --fields pid,port,cwd,cmdline
```

### Structured Kill Results

The `kill` verb resolves PIDs from explicit PIDs, `pid:PID`, `:PORT`, `port:PORT`, `--pid`, `--port`, and optional process filters (`--name`, `--user`, `--path`). It is safe by default: without `--yes` it refuses to signal anything, and `--dry-run` returns the exact targets it would kill.

```bash
# Agent-friendly planning step
listen-killer kill --port 5173 --dry-run --output json

# Execute after review
listen-killer kill --port 5173 --signal TERM --yes --output json
```

## Architecture

```
┌──────────────────────────────────────────────┐
│  Title bar: 🎧 Listen Killer  N listeners    │
├──────────────────────────────────────────────┤
│  Table (scrollable)                          │
│  ●  PID    Process    Port   Uptime  Memory  │
│    585731  Discord    6463   7h19m  565 MB   │
│  ● 184551  obsidian   3456   12h37m 665 MB   │
│                                              │
├──────────────────────────────────────────────┤
│  Detail pane                                 │
│  184551 — obsidian                           │
│  PID  184551  Name   obsidian  User  manuel  │
│  Port  3456   Addr   127.0.0.1  Up   12h37m  │
│  Binary  /app/obsidian                       │
│  URL  http://127.0.0.1:3456  ● MARKED        │
├──────────────────────────────────────────────┤
│  Footer: status + key help                   │
└──────────────────────────────────────────────┘
```

### Data Flow

1. **Scanner** (`pkg/listener/`): finds all TCP LISTEN sockets and enriches
   each with process details. Socket enumeration is platform-split —
   `socket_source_other.go` uses gopsutil (`/proc` on Linux), while
   `socket_source_darwin.go` shells out to `lsof` on macOS. Process enrichment
   (`process_facts.go`) and CPU sampling (`cpu.go`) are shared.
2. **TUI** (`pkg/tui/`): Bubbletea model with three modes — table navigation, kill confirmation dialog, and detail pane. The table is the `bubbles/table` widget from Charmbracelet.
3. **CLI** (`cmd/listen-killer/`): Glazed command framework wraps the scanner for structured list and kill commands (JSON/YAML/CSV/table), including filters for scripts and LLM-agent workflows.

### Tech Stack

| Component | Library |
|-----------|---------|
| TUI framework | [Bubbletea](https://github.com/charmbracelet/bubbletea) |
| Table widget | [bubbles/table](https://github.com/charmbracelet/bubbles) |
| Styling | [Lipgloss](https://github.com/charmbracelet/lipgloss) |
| CLI framework | [Glazed](https://github.com/go-go-golems/glazed) |
| System info | [gopsutil](https://github.com/shirou/gopsutil) |

## License

MIT
