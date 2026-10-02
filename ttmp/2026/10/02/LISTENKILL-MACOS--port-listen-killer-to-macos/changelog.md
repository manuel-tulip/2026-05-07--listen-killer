# Changelog

## 2026-10-02

- Initial workspace created

## 2026-10-02

Create LISTENKILL-MACOS ticket and author the intern-facing macOS port guide (analysis, design, implementation plan) plus investigation diary; identify gaps G1-G9 with source-cited root causes.

### Related Files

- /Users/manuel.odendahl/code/wesen/2026-05-07--listen-killer/pkg/listener/scanner.go — Primary porting surface
- /Users/manuel.odendahl/code/wesen/2026-05-07--listen-killer/pkg/tui/model.go — IPv6 URL bug

## 2026-10-02

Uploaded the intern guide bundle (design doc + diary) to reMarkable at /ai/2026/10/02/LISTENKILL-MACOS.

## 2026-10-02

Implement core port (commit faa0b4c): Darwin lsof socket source, portable Scanner + delta CPU sampler, cwd/exe degradation warnings, IPv6 URL fix; add --cpu-sample; document macOS notes in README.

### Related Files

- /Users/manuel.odendahl/code/wesen/2026-05-07--listen-killer/README.md — macOS notes and new flags
- /Users/manuel.odendahl/code/wesen/2026-05-07--listen-killer/pkg/listener/cpu.go — CPU rate sampler
- /Users/manuel.odendahl/code/wesen/2026-05-07--listen-killer/pkg/listener/socket_source_darwin.go — Darwin socket enumeration via lsof
