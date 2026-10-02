---
Title: Diary
Ticket: LISTENKILL-MACOS
Status: active
Topics:
    - go
    - bubbletea
    - macos
    - porting
    - networking
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/listener/scanner.go
      Note: Investigated discovery path for the diary
    - Path: repo://pkg/tui/model.go
      Note: IPv6 URL bug found during investigation
ExternalSources: []
Summary: Chronological implementation diary for the macOS port ticket.
LastUpdated: 2026-10-02T10:26:55.174465-04:00
WhatFor: Record the investigation, evidence and decisions behind the macOS port guide.
WhenToUse: Read the latest checkpoint when resuming work on the macOS port.
---

# Diary

## Step 1: Investigate the existing system and produce the macOS port guide

The user asked for a new docmgr ticket covering "Port to macOS", containing a
detailed analysis/design/implementation guide for a new intern: prose, bullets,
pseudocode, diagrams, API references and file references, stored in the ticket
and uploaded to reMarkable. I began by reading the entire repository to
establish what "port" even means here — the code already compiles and runs on
macOS, so the real work is understanding and fixing the Darwin discovery path
rather than making it build.

I read all source files (`pkg/listener/*.go`, `pkg/tui/*.go`,
`cmd/listen-killer/**`), the README, and the pre-existing `LISTENKILL` ticket.
I then built the binary on this Mac, ran `list`/`kill --dry-run`, and inspected
gopsutil's build-tagged Darwin sources in the module cache to confirm exactly
how sockets and processes are discovered on Darwin. The result is the intern
guide in `design-doc/01-intern-guide-porting-listen-killer-to-macos.md`.

### Prompt Context
**User prompt (verbatim):** "Create a new docmgr ticket to Port to macos,
Create a detailed analysis / design / implementation guide that is for a new
intern, explaining all the parts of the system needed to understand what it is,
with prose paragraphs and bullet points and pseudocode and diagrams and api
references and file references. It should be very clear and technical. Store in
the ticket and upload to remarkable."
**Assistant interpretation:** Create a docmgr ticket for a macOS port and author
a comprehensive, intern-facing design/implementation document; then upload it
to reMarkable.
**Inferred user intent:** Give a new engineer everything needed to understand
Listen Killer and to execute the macOS port correctly, without re-deriving the
Darwin-specific behavior from scratch.
**Commit (code):** N/A — documentation-only step, no code changed.

### What I did
- Explored the repo: `pkg/listener/{types,scanner,process}.go`,
  `pkg/tui/{model,update,view,keymap,styles}.go`,
  `cmd/listen-killer/{main.go,cmds/tui/{tui,kill,helpers}.go}`, `go.mod`,
  `README.md`.
- Built and ran on host:
  - `go build -o /tmp/listen-killer ./cmd/listen-killer/` → BUILD_OK.
  - `/tmp/listen-killer list --output json` → rows observed, `cpu_percent` empty.
  - `/tmp/listen-killer kill --port 5184 --dry-run --output json` → resolved.
- Inspected gopsutil v3.24.5 Darwin sources in
  `$GOMODCACHE/github.com/shirou/gopsutil/v3@v3.24.5/`: `net/net_unix.go`,
  `internal/common/common_unix.go`, `process/process_darwin.go`,
  `process_darwin_cgo.go`, `process_darwin_nocgo.go`, `process/process.go`.
- Timed the scan (`~0.11s`) and raw `lsof` (`~0.06s`).
- Created ticket `LISTENKILL-MACOS` and the guide document.

### Why
- The user explicitly asked for a technical intern guide plus reMarkable
  delivery; the guide is the primary deliverable, not a code change.
- Reading third-party source was necessary because the app's macOS behavior is
  entirely delegated to gopsutil; documenting that delegation without evidence
  would have produced guesswork.

### What worked
- The app builds and runs on `darwin/arm64` (go1.27.1); JSON and dry-run kill
  both behave correctly for user-owned processes.
- `lsof -a -n -P -i tcp -i udp` reproduces exactly what gopsutil parses,
  confirming the socket source on Darwin.

### What didn't work
- No blocking failures. `cpu_percent` is always empty — confirmed to be a real
  bug (gap G3), not a macOS-only quirk: `Process.Percent(0)` returns 0 on the
  first call for a fresh `*Process`, and `ScanListeners` creates a fresh
  `Process` every scan.

### What I learned
- Darwin `net.Connections("inet")` shells out to `lsof`; Linux reads `/proc`.
  This single difference explains G1, G2 and G5.
- The cgo/nocgo build split (`process_darwin_{cgo,nocgo}.go`) means a
  cross-compiled Darwin binary silently loses `cwd`.
- `parseNetLine` sets `Status` only when the lsof line has 10 whitespace
  fields, which is why the `LISTEN` filter works.
- `listenerURL` produces `http://::1:port` for IPv6 loopback — a real bug
  (G5) found while auditing address handling.

### What was tricky to build
- Distinguishing "the code compiles and mostly works" from "the port is done":
  several fields degrade silently because `scanner.go` discards per-field
  errors. The guide therefore recommends classifying degradation
  (Denied/Unsupported) rather than swallowing it.

### What warrants a second pair of eyes
- The recommended platform-abstraction design (interfaces + build-tagged
  sources) versus the lower-effort "wrap gopsutil" option; a reviewer should
  confirm the added complexity is justified for a small tool.
- The IPv6 URL fix and the CPU-sampler cache lifetime (must prune dead PIDs).

### What should be done in the future
- Execute Phases 0–5 from the guide's implementation plan.
- Decide the cgo build policy and add Darwin to CI/release.

### Code review instructions
- Review the guide at
  `ttmp/2026/10/02/LISTENKILL-MACOS--port-listen-killer-to-macos/design-doc/01-intern-guide-porting-listen-killer-to-macos.md`.
- Reproduce key evidence:
  `/usr/sbin/lsof -a -n -P -i tcp -i udp | awk 'NR==1 || /LISTEN/'` and
  `/tmp/listen-killer list --output json | jq '.[0].cpu_percent'`.

### Technical details
- Host: Darwin 24.6.0, arm64 (Apple M1 Pro), go1.27.1.
- Repo revision: `ef03a2c`.
- Ticket root: `ttmp/2026/10/02/LISTENKILL-MACOS--port-listen-killer-to-macos`.

---

## Step 2: Commit documentation milestone and deliver to reMarkable

The documentation milestone is complete and committed as a single coherent
checkpoint: the new `LISTENKILL-MACOS` ticket, the intern guide, the diary,
tasks, changelog, index, and the docmgr topics vocabulary additions. The guide
bundle (design doc + diary) was also uploaded to reMarkable, confirming cloud
delivery.

### Prompt Context
**User prompt (verbatim):** "btw commit at appropriate intervals and keep a
detailed diary as you work (using the diary format from the skill)."
**Assistant interpretation:** Commit the completed documentation checkpoint and
keep the diary updated with commit hashes as work proceeds.
**Inferred user intent:** Durable, traceable checkpoints and an up-to-date work
record in the ticket's diary.

### Evidence and commits
- Code/docs commit: `5ffce3e` — "Add LISTENKILL-MACOS ticket: intern guide for
  macOS port" (7 files: ticket README/changelog/index/tasks/diary/design-doc +
  `ttmp/vocabulary.yaml`).
- `docmgr doctor --ticket LISTENKILL-MACOS` → `✅ All checks passed`.
- reMarkable: `OK: uploaded LISTENKILL-MACOS Port to macOS - Intern Guide.pdf ->
  /ai/2026/10/02/LISTENKILL-MACOS` (bundle = design doc + diary, `--toc-depth 2`).
- Tasks `t4ph` (author guide) and `v9ed` (upload) checked.

### Noteworthy decisions or failures
- No failures. `doctor` initially warned about unknown topics; resolved by
  seeding `go`, `bubbletea`, `macos`, `networking`, `porting`, `glazed`, `tui`
  into `ttmp/vocabulary.yaml`.

### Remaining requirements and next action
- Open implementation tasks remain (`s000`, `xj53`, `6e7o`, `y4fa`, `ausn`) per
  the guide's Phase 0-5 plan. Next concrete action: begin Phase 0 (Darwin build
  harness + captured `lsof` fixture) if code work is authorized.
- No code was changed, so no code-level validation beyond `go build` was due.
