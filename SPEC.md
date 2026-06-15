# SPEC: -tzdata flag

## Objective

Add optional timezone data support to `ocipack` so that scratch images containing
any runtime (Go, C, Python, Rust, Java, …) can resolve named time zones at runtime.

The canonical Linux timezone database lives at `/usr/share/zoneinfo/` and is read
by glibc, musl, Go's `time` package, Python's `zoneinfo`, and most other runtimes.
Copying that tree into a scratch image via `ocipack` is the simplest way to support
every consumer without runtime-specific tricks.

---

## CLI changes

Two new flags, opt-in (nothing added unless at least one is passed):

```
  -tzdata                add /usr/share/zoneinfo from the build host
  -tzdata-path dir       add zoneinfo from a specific directory (implies -tzdata)
```

Behaviour:

- If `-tzdata` is passed without `-tzdata-path`, auto-detect the zoneinfo directory
  from the host (see detection list below).
- If `-tzdata-path dir` is passed, use that directory (implicitly enables tzdata).
- Passing both `-tzdata` and `-tzdata-path dir` is the same as `-tzdata-path dir`
  alone.
- Error and exit non-zero if tzdata is requested but no directory is found/readable.

Help text addition (after `-no-tmp` line):

```
  -tzdata                add /usr/share/zoneinfo (auto-detect from host)
  -tzdata-path dir       add zoneinfo from dir (implies -tzdata)
```

---

## Library API

New method on `*Image` in `helpers.go` (alongside `AddCABundle` and `AddTmp`):

```go
// AddTZData copies the zoneinfo directory tree from the build host into
// the image at /usr/share/zoneinfo/. Pass "" to auto-detect the source
// directory; pass a path to use a specific directory.
func (img *Image) AddTZData(dir string) error
```

### Auto-detection order

```
$ZONEINFO            (env var, if it is a directory path — not a zip file)
/usr/share/zoneinfo  (Debian / Ubuntu / Fedora / most distros)
/usr/lib/zoneinfo    (some older distros)
/usr/share/lib/zoneinfo  (Solaris / OpenIndiana)
/etc/zoneinfo        (musl / Alpine)
```

First directory that exists and is readable wins.

### Walk behaviour

- Walk the source directory recursively with `filepath.WalkDir`.
- Add each sub-directory as `FileDirectory` (mode `0755`).
- Add each regular file as `FileRegular` (mode `0644`), data read from host.
- Skip symlinks (zoneinfo directories sometimes contain symlinks like
  `posix/` → `.` or `right/` that duplicate the tree; ignoring them keeps
  the image lean and avoids duplicating every zone twice).
- Container destination root: `usr/share/zoneinfo` (no leading `/`, matching
  the existing convention in `AddCABundle`).
- Use `img.addEntry` so later calls can override individual entries.

---

## Project structure

No new files. Changes touch:

| File | Change |
|------|--------|
| `helpers.go` | Add `AddTZData(dir string) error` and `tzdataPaths` detection list |
| `helpers_test.go` | Unit tests for `AddTZData` |
| `cmd/ocipack/main.go` | Add `-tzdata` / `-tzdata-path` flags and wiring |
| `cmd/ocipack/cli_test.go` | CLI integration tests for the new flags |
| `README.md` | Document the new flags |

---

## Code style

Follow the conventions already in the file:

- No comments unless the why is non-obvious.
- `set -eu` / `set -eu -o pipefail` not applicable (Go).
- Prefer `filepath.WalkDir` over `filepath.Walk` (avoids extra `os.Lstat`).
- Error messages: lower-case, no trailing period, wrap with `%w` where callers
  may need to inspect.

---

## Testing strategy

### Unit tests (`helpers_test.go`)

1. **Happy path** — create a temp dir with a realistic zoneinfo-like tree
   (a couple of subdirectories and zone files), call `AddTZData(dir)`, assert
   that `img.files` contains the expected `FileDirectory` and `FileRegular`
   entries at the correct `usr/share/zoneinfo/…` paths.
2. **Symlinks are skipped** — add a symlink in the temp tree; assert it does
   not appear in `img.files`.
3. **Auto-detection** — set `$ZONEINFO` to the temp dir, call `AddTZData("")`,
   assert same result as the explicit-path case.
4. **Missing directory** — call `AddTZData("/nonexistent")` and assert an error
   is returned.
5. **No source found** — unset `$ZONEINFO`, override `tzdataPaths` to an empty
   or all-nonexistent list, call `AddTZData("")`, assert error.

### CLI tests (`cmd/ocipack/cli_test.go`)

Follow the pattern of existing tests (build a real binary via `go build`, run
it, inspect the output tar):

1. **`-tzdata` flag** — build an image with `-tzdata`; open the output tar;
   assert at least one `usr/share/zoneinfo/` entry is present.
2. **`-tzdata-path dir`** — supply an explicit small temp zoneinfo dir; assert
   entries appear.
3. **Default (no flag)** — build without `-tzdata`; assert no `usr/share/zoneinfo`
   entries in the tar.
4. **`-tzdata-path /nonexistent`** — assert non-zero exit.

---

## Boundaries

**Always do:**
- Walk the source tree recursively.
- Skip symlinks silently.
- Follow the existing `addEntry` deduplication so `-add-file` can still override
  a specific zone file if needed.

**Never do:**
- Add `/etc/localtime` automatically (selecting the active timezone is out of
  scope; users can add it with `-add-link /etc/localtime:/usr/share/zoneinfo/UTC`).
- Download tzdata from the internet.
- Embed Go's `time/tzdata` package — that only helps Go binaries and doesn't
  put files in the image.
- Add a `-no-tzdata` opt-out flag — tzdata is opt-in, no opt-out needed.
