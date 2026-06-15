# Implementation Plan: -tzdata flag

## Overview

Add opt-in timezone data support to `ocipack`. A new `AddTZData(dir string) error`
library method walks a host zoneinfo directory and adds it into the image at
`usr/share/zoneinfo/`. Two new CLI flags (`-tzdata`, `-tzdata-path`) expose the
feature. Works for any runtime (Go, C, Python, Rust, …).

## Architecture Decisions

- **Opt-in only** — nothing added unless `-tzdata` or `-tzdata-path` is passed.
- **Host filesystem as source** — mirrors `AddCABundle`; reads from the build host.
- **Skip symlinks** — avoids duplicating `posix/` and `right/` subtrees.
- **`filepath.WalkDir`** — preferred over `filepath.Walk` (no extra `os.Lstat`).
- No new files — all changes land in existing files.

---

## Task List

### Phase 1: Library

- [ ] Task 1: Add `AddTZData` to `helpers.go` with unit tests

### Checkpoint: Library
- [ ] `go test ./...` passes

### Phase 2: CLI

- [ ] Task 2: Wire `-tzdata` / `-tzdata-path` flags in `cmd/ocipack/main.go` with CLI tests

### Checkpoint: CLI
- [ ] `go test ./...` passes
- [ ] `go build ./...` succeeds

### Phase 3: Docs

- [ ] Task 3: Update README with new flags

### Checkpoint: Complete
- [ ] All tests pass
- [ ] README documents both flags

---

## Task 1: Add `AddTZData` to `helpers.go`

**Description:** Add the `tzdataPaths` detection list and `AddTZData(dir string) error`
method to `helpers.go`. The method walks the source directory, adding directories
at mode 0755 and regular files at mode 0644, all under `usr/share/zoneinfo/`.
Symlinks are silently skipped. Auto-detection checks `$ZONEINFO` env first, then
`tzdataPaths` in order.

**Acceptance criteria:**
- [ ] `AddTZData(dir)` with an explicit dir adds the expected `FileDirectory` and
  `FileRegular` entries at `usr/share/zoneinfo/…` paths
- [ ] Symlinks in the source tree do not appear in `img.files`
- [ ] `AddTZData("")` auto-detects via `$ZONEINFO` env var when set to a directory
- [ ] `AddTZData("/nonexistent")` returns an error
- [ ] `AddTZData("")` with no valid path in `tzdataPaths` returns an error

**Verification:**
- [ ] `go test -run "TestHelpersTZData" ./...`
- [ ] `go build ./...`

**Dependencies:** None

**Files touched:**
- `helpers.go`
- `helpers_test.go`

**Estimated scope:** Small

---

## Task 2: Wire CLI flags

**Description:** Add `-tzdata bool` and `-tzdata-path string` flags to
`cmd/ocipack/main.go`. If either is set, call `img.AddTZData(tzdataPath)`.
Update `flag.Usage` with the two new flag lines after `-no-tmp`.

**Acceptance criteria:**
- [ ] `ocipack -tzdata binary out.tar.gz` produces an image with
  `usr/share/zoneinfo/` entries in the layer
- [ ] `ocipack -tzdata-path <dir> binary out.tar.gz` uses the supplied dir
- [ ] Running without either flag produces no `usr/share/zoneinfo` entries
- [ ] `ocipack -tzdata-path /nonexistent binary out.tar.gz` exits non-zero

**Verification:**
- [ ] `go test -run "TestCLITZData" ./cmd/ocipack/`
- [ ] `go test ./...`
- [ ] `go build ./...`

**Dependencies:** Task 1

**Files touched:**
- `cmd/ocipack/main.go`
- `cmd/ocipack/cli_test.go`

**Estimated scope:** Small

---

## Task 3: Update README

**Description:** Document `-tzdata` and `-tzdata-path` in the CLI flags section
of `README.md`, consistent with the existing flag documentation style.

**Acceptance criteria:**
- [ ] Both flags appear in the README with a one-line description each

**Verification:**
- [ ] Visual review of `README.md`

**Dependencies:** Task 2

**Files touched:**
- `README.md`

**Estimated scope:** XS

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Host has no zoneinfo dir | Low | Auto-detect returns clear error; test with temp dir |
| Large zoneinfo tree slows tests | Low | CLI test uses a tiny hand-crafted temp dir |
| `$ZONEINFO` set to a zip file path | Low | `os.Stat` check — only accept directories |
