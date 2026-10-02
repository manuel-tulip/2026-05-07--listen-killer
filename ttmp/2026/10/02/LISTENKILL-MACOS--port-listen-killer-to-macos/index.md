---
Title: Port Listen Killer to macOS
Ticket: LISTENKILL-MACOS
Status: active
Topics:
    - go
    - bubbletea
    - macos
    - porting
    - networking
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-02T10:26:53.73543-04:00
WhatFor: ""
WhenToUse: ""
---

# Port Listen Killer to macOS

## Overview

Listen Killer already compiles and mostly works on macOS, but its discovery
layer was written for Linux. On Darwin, socket enumeration and process
enrichment are delegated to `lsof`, `ps`, `sysctl` and `libproc`, which changes
what data is available and how it can fail. This ticket makes the port explicit:
it documents the system for a new intern and lays out a phased plan to fix the
silent degradations (`cpu_percent` always empty, `cwd` missing without cgo or
without permissions, an IPv6 browser-URL bug) and to package/sign Darwin
binaries.

Primary deliverable:
[Intern Guide - Porting Listen Killer to macOS](./design-doc/01-intern-guide-porting-listen-killer-to-macos.md).

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- go
- bubbletea
- macos
- porting
- networking

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
