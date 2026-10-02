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

---

## Step 3: Implement the core port (socket sources, CPU sampling, IPv6 URL, degradation warnings)

The user asked to implement the port. I implemented the highest-value, most
verifiable parts of Phases 1-3 from the guide: a platform-split socket source
(Darwin `lsof`, everything else `/proc`), a portable `Scanner` with a
real delta-based CPU sampler, explicit degradation warnings for `cwd`/`exe`,
and the IPv6 browser-URL fix. I verified behavior on this Mac and kept the
Linux path intact (cross-build + pure unit tests).

### Prompt Context
**User prompt (verbatim):** "Implement, btw commit at appropriate intervals and
keep a detailed diary as you work (using the diary format from the skill)."
**Assistant interpretation:** Execute the implementation plan from the guide,
committing coherent checkpoints and recording each in this diary.
**Inferred user intent:** Make Listen Killer correct and complete on macOS while
preserving Linux behavior and keeping an auditable work record.
**Commit (code):** `faa0b4c` — "macOS port: platform socket sources, real CPU
sampling, IPv6 URL fix".

### What I did
- New platform socket sources:
  - `pkg/listener/socket.go` — `RawSocket` neutral type.
  - `pkg/listener/socket_source_darwin.go` (`//go:build darwin`) — resolves
    `/usr/sbin/lsof` (then PATH), runs `lsof -a -n -P -i tcp` with a 15s
    timeout, turns failures into actionable errors.
  - `pkg/listener/socket_source_other.go` (`//go:build !darwin`) — the original
    gopsutil `/proc` path, unchanged semantically.
  - `pkg/listener/lsof_parse.go` — build-tag-free `parseLsofListenLine`.
- `pkg/listener/process_facts.go` — `gatherFacts` + `appendWarning`;
  distinguishes permission-denied from not-implemented (CGO_ENABLED=0).
- `pkg/listener/cpu.go` — `CPUSampler` caching cumulative CPU seconds per PID
  and returning `(percent, sampled)`; `Prune` bounds memory.
- `pkg/listener/scanner.go` — rewritten as portable orchestration with a
  persistent `Scanner` (`NewScanner`, `Scan`, `ScanWithSample`);
  `ScanListeners` retained for compatibility; per-PID dedup of enrichment.
- `pkg/listener/types.go` — added `Warnings []string` and `CPUSampled bool`.
- TUI: `pkg/tui/model.go` holds a `*listener.Scanner`; `listenerURL` now
  brackets IPv6 hosts; `pkg/tui/view.go` shows a Warn line.
- CLI: `list` gains `--cpu-sample <duration>`; `helpers.go` emits
  `warnings` and `cpu_sampled`.
- Tests: `lsof_parse_test.go`, `cpu_test.go`, `pkg/tui/model_test.go`.

### Why
- The guide (§9-§11) recommended exactly this split; owning the `lsof` call
  fixes G1/G2 error handling and makes the parser testable off-Mac.
- CPU had to be a *rate*; a fresh `Process` per scan made `Percent(0)` always 0.

### What worked
- `go build`, `go vet`, `go test ./...` pass on `darwin/arm64`; `GOOS=linux
  GOARCH=amd64 go build ./...` succeeds.
- Runtime: `list --cpu-sample 400ms` reports real values (jetbrains 0.2%,
  docker 0.6%) and `sampled=true`; without the flag `cpu_percent` stays empty
  (`sampled=false`). Idle `node` shows `cpu=0 sampled=true` — an honest zero,
  now distinguishable from "not sampled".
- `kill --port 5184 --dry-run` still resolves correctly.

### What didn't work
- First CPU cutover rendered a true 0% as empty (the old `> 0` guard). Fixed by
  returning a `sampled` boolean from the sampler and adding `CPUSampled` to
  `ListenerInfo`; formatting now keys off `CPUSampled`, not `> 0`.
- No other failures; `gofmt -l` still flags three pre-existing untouched files
  (`process.go`, `styles.go`, `update.go`) left as-is to keep the diff focused.

### What I learned
- gopsutil's `Percent` semantics are `deltaProc/wall*100` (100% == one core);
  the sampler replicates this so values look like `top`.
- `lsof` emits `[::1]:port` and `*:port`; `net.SplitHostPort` yields `::1` and
  `*`, which is why the address field is already correct before normalization.
- We cannot import gopsutil's `internal/common` sentinel, so "not implemented"
  is matched by message and permission by the exported `process.ErrorNotPermitted`.

### What was tricky to build
- CPU sampling under multiple sockets per PID: observing per socket within one
  scan resets the sample (wall=0), so observation is done once per unique PID
  per scan, with a per-scan facts cache.
- `lsof` exits non-zero both when it finds nothing and when it cannot inspect
  some processes; only a zero-output failure is treated as fatal, matching
  gopsutil's leniency.

### What warrants a second pair of eyes
- The `Scanner` mutex + `CPUSampler` mutex interaction and the fact that
  `ScanWithSample` performs two full scans (two `lsof` invocations).
- Message-based matching of gopsutil's not-implemented sentinel is version-
  fragile; pin the gopsutil version and revisit on upgrade.
- `CPUSampled`/`Warnings` are additive output-contract changes; confirm scripts
  and the README stay accurate.

### What should be done in the future
- Phase 4: README "macOS notes" (lsof, Full Disk Access, SIP, CGO cwd).
- Phase 5: Darwin packaging/signing in release.
- Consider a Scanner test injected with a fake socket source (the build-tagged
  function currently prevents this).

### Code review instructions
- Start at `pkg/listener/scanner.go`, then `socket_source_darwin.go`,
  `lsof_parse.go`, `process_facts.go`, `cpu.go`.
- Validate: `go test ./...`; `go build -o /tmp/lk ./cmd/listen-killer/`;
  `/tmp/lk list --cpu-sample 400ms --output json | jq '.[0]'`;
  `/usr/sbin/lsof -a -n -P -i tcp | awk 'NR==1 || /LISTEN/'`.

### Technical details
- Host: Darwin 24.6.0 arm64, go1.27.1; commit `faa0b4c`.
- New env-facing surface: `--cpu-sample` (string duration, default empty),
  `cpu_sampled` JSON field, `warnings` JSON field.

---

## Step 4: Darwin packaging (Makefile, CI, ad-hoc signing)

Added reproducible Darwin build tooling so the port can actually be shipped:
a `Makefile` with `check`/`build-darwin`/`sign-darwin` targets, a two-job CI
workflow that tests on Linux and macOS and emits signed Darwin artifacts, and
README instructions for the Gatekeeper quarantine workaround. `make check`
(vet + test + gofmt) and `make build-darwin` both pass on this machine.

### Prompt Context
**User prompt (verbatim):** see Step 3.
**Assistant interpretation:** Complete the guide's Phase 4-5 (docs + packaging).
**Inferred user intent:** A port is not done until the binary can be built,
run, and distributed on the target OS.

### Evidence and commits
- Code/docs commit: (pending) `make build-darwin`, CI workflow, README, `.gitignore`.
- `make check` → vet + tests pass, gofmt clean.
- `make build-darwin` →
  `dist/listen-killer-darwin-arm64` (49.9 MB) and
  `dist/listen-killer-darwin-amd64` (53.9 MB), both `flags=0x2(adhoc)` per
  `codesign -dv`.
- Side fix: formatted three pre-existing files (`pkg/listener/process.go`,
  `pkg/tui/styles.go`, `pkg/tui/update.go`) so `make fmt` is clean; changes are
  whitespace-only (EOF newline, label indent).

### Noteworthy decisions or failures
- Chose ad-hoc signing for local/dev use; full notarization and a Homebrew tap
  remain out of scope and are tracked as a follow-up task. `CGO_ENABLED=1` is
  mandatory for accurate `cwd`/`exe`, hence building on a macOS runner.
- No failures; cross-building `darwin/amd64` with cgo on Apple Silicon worked
  with the installed Xcode CLT.

### Remaining requirements and next action
- New task: notarization + Homebrew for public distribution (G8 remainder).
- Consider a scanner test with an injected fake socket source.
- Overall port is functionally complete; remaining items are release hardening.
