# ocipack

[![Go Reference](https://pkg.go.dev/badge/codeberg.org/fhuebner/ocipack.svg)](https://pkg.go.dev/codeberg.org/fhuebner/ocipack)

Zero-dependency Go library for packaging static binaries in minimal OCI image tarballs (OCI Image Spec 1.0).

Ideal for:

- Loading into **Docker**: `docker load -i image.tar.gz`
- Loading into **Podman**: `podman load -i image.tar.gz`
- Importing into **containerd**: `ctr images import image.tar.gz`
- Preloading with **k3s**: `cp image.tar.gz /var/lib/rancher/k3s/agent/images/`
- Copying with **skopeo**: `skopeo copy oci-archive:image.tar.gz docker://registry.example.com/myapp:latest`

## CLI

Add as a [Go tool](https://tip.golang.org/doc/go1.24#tools) dependency:

```sh
go get -tool codeberg.org/fhuebner/ocipack/cmd/ocipack
```

Turn static binary into oci image:

```sh
CGO_ENABLED=0 go build -o myapp .
go tool ocipack myapp image.tar.gz
```

```
usage: ocipack [flags] <binary> <output>
  -version               print version and exit
  -tag ref               image reference
  -user user[:group]     (default "65534")
  -entrypoint arg        entrypoint (repeatable)
  -cmd arg               cmd (repeatable)
  -workdir path          working directory
  -env KEY=VALUE         set env var (repeatable)
  -label KEY=VALUE       set image label (repeatable)
  -add-file c:h[:mode]   add file: container-path:host-path:mode (repeatable)
  -add-dir path[:mode]   add directory, optional octal mode (repeatable)
  -add-link path:target  add symlink (repeatable)
  -created rfc3339       image timestamp; defaults to now
  -cacerts-path path     CA bundle path (auto-detect when unset)
  -no-cacerts            skip CA bundle
  -no-tmp                skip /tmp
```

## Library

```go
import "codeberg.org/fhuebner/ocipack"

...

img := ocipack.New(ocipack.Platform{OS: "linux"})

if err := img.AddBinary("./myapp-amd64"); err != nil {
    log.Fatal(err)
}

img.AddTmp()

if err := img.WriteTar("dist/image-amd64.tar.gz", ""); err != nil {
    log.Fatal(err)
}
```

`AddBinary` detects the architecture from the ELF header, sets the entrypoint, and adds the file.
It returns `ErrDynamicallyLinked` if the binary has a dynamic linker interpreter:

```go
if err := img.AddBinary("./myapp-amd64"); err != nil {
    if errors.Is(err, ocipack.ErrDynamicallyLinked) {
        log.Println("warning:", err)
    }
    log.Fatal(err)
}
```

Or set everything explicitly:

```go
img := ocipack.New(ocipack.Platform{OS: "linux", Architecture: "amd64"})

img.SetUser("1000:1000")

img.SetEntrypoint("/usr/local/bin/myapp")

if err := img.AddFile("/usr/local/bin/myapp", "./myapp-amd64", 0755); err != nil {
    log.Fatal(err)
}

if err := img.AddCABundle(""); err != nil {
    log.Fatal(err)
}

if err := img.WriteTar("dist/image-amd64.tar.gz", ""); err != nil {
    log.Fatal(err)
}
```

## Reproducible Builds

To get rid of all volatile data, make sure to use all these settings:

### ocipack

```
go tool ocipack \
  -created="1970-01-01T00:00:00Z" \ # pin image timestamp to epoch instead of now
  -no-cacerts \                     # skip CA bundle (host bundle may change between builds)
  myapp \
  image.tar.gz
```

### Go Build

The `go` directive in `go.mod` sets the minimum toolchain version. With the default
`GOTOOLCHAIN=auto`, Go downloads that version if the local installation is older — no
action required on the build machine.

```
CGO_ENABLED=0 \ # disable cgo; pure Go produces a static binary
GOOS=linux \    # target OS
GOARCH=amd64 \  # target architecture
go build \
  -trimpath \       # strip local file paths from the binary
  -buildvcs=false \ # omit git commit hash and dirty flag
  -ldflags="-s -w -buildid=" \ # strip symbol table, DWARF, and build ID
  -o myapp \
  .
```

## Testing

The e2e test requires a container runtime and is gated behind the `integration` build tag:

```sh
go test -tags integration -v -race -vet=all -count=1 ./...
```
