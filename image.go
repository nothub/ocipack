package ocipack

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"path"
	"time"
)

// Platform identifies the target OS and CPU architecture.
type Platform struct {
	OS           string // required: "linux"
	Architecture string // required: "amd64", "arm64"
	Variant      string // optional: "v7", "v8"
}

// BuildResult is returned by Build and exposes all digests and raw blobs.
type BuildResult struct {
	ConfigDigest string
	ConfigSize   int64
	Config       []byte

	ManifestDigest string
	ManifestSize   int64
	Manifest       []byte

	// LayerDigest and Layer are empty/nil for zero-layer images.
	LayerDigest string
	LayerSize   int64
	Layer       []byte

	// DiffIDs holds the uncompressed tar digest for each layer.
	// Empty slice for zero-layer images.
	DiffIDs []string
}

// Image is the builder for an OCI scratch image.
type Image struct {
	platform        Platform
	created         *time.Time
	user            string
	workdir         string
	env             []string
	entrypoint      []string
	cmd             []string
	labels          map[string]string
	forceEmptyLayer bool
	files           []File
	fileIdx         map[string]int // normalized path → index in files slice
}

// New creates an empty image for the given platform.
// The default user is "65534" (numeric nobody), which works in scratch images
// that have no /etc/passwd.
func New(p Platform) *Image {
	return &Image{platform: p, user: "65534", fileIdx: make(map[string]int)}
}

// SetCreated sets the image creation timestamp. If not called, the field is
// omitted from the config and two builds of the same input produce identical
// digests. Use a fixed value in build scripts to keep that property.
func (img *Image) SetCreated(t time.Time) {
	img.created = &t
}

// SetUser sets the user (and optional group) the entrypoint runs as.
// Use a numeric uid like "65534" or "1000:1000" — scratch images have no
// /etc/passwd, so name-based users won't work.
// Pass "0" to run as root. Default is "65534" (nobody).
func (img *Image) SetUser(user string) {
	img.user = user
}

// SetWorkdir sets the working directory for the entrypoint process.
func (img *Image) SetWorkdir(dir string) {
	img.workdir = dir
}

// AddEnv adds an environment variable (KEY=value) to the image config.
func (img *Image) AddEnv(key, value string) {
	img.env = append(img.env, key+"="+value)
}

// SetEntrypoint sets the image entrypoint.
func (img *Image) SetEntrypoint(args ...string) {
	img.entrypoint = args
}

// SetCmd sets the default command arguments.
func (img *Image) SetCmd(args ...string) {
	img.cmd = args
}

// SetLabel adds a label to the image config.
func (img *Image) SetLabel(key, value string) {
	if img.labels == nil {
		img.labels = make(map[string]string)
	}
	img.labels[key] = value
}

// ForceEmptyLayer makes Build emit a deterministic empty tar layer
// even when no files have been added.
func (img *Image) ForceEmptyLayer() {
	img.forceEmptyLayer = true
}

// AddFile reads hostPath from the local filesystem and adds it to the image
// layer at containerPath (e.g. "/usr/local/bin/app").
func (img *Image) AddFile(containerPath, hostPath string, mode int64) error {
	norm, err := normalizePath(containerPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(hostPath)
	if err != nil {
		return err
	}
	img.addEntry(File{Path: norm, Type: FileRegular, Data: data, Mode: mode})
	return nil
}

// ErrDynamicallyLinked is returned by AddBinary when the binary has a dynamic
// linker interpreter (PT_INTERP). Scratch images have no libc, so the process
// will fail to start at runtime. The image is still fully assembled; callers
// can treat this as a warning by checking errors.Is(err, ErrDynamicallyLinked).
var ErrDynamicallyLinked = errors.New("binary is dynamically linked; use CGO_ENABLED=0 for a static binary")

// AddBinary detects the architecture from the ELF header, sets the image
// platform and entrypoint, and adds the binary as basename(hostPath) at /
// with mode 0755. Returns ErrDynamicallyLinked if the binary requires a
// dynamic linker — the image is still assembled, so callers may warn and continue.
func (img *Image) AddBinary(hostPath string) error {
	arch, dynamic, err := elfInfo(hostPath)
	if err != nil {
		return err
	}
	img.platform.Architecture = arch
	containerPath := "/" + path.Base(hostPath)
	img.SetEntrypoint(containerPath)
	if err := img.AddFile(containerPath, hostPath, 0755); err != nil {
		return err
	}
	if dynamic {
		return ErrDynamicallyLinked
	}
	return nil
}

func elfInfo(hostPath string) (arch string, dynamic bool, err error) {
	f, err := elf.Open(hostPath)
	if err != nil {
		return "", false, fmt.Errorf("open %s: %w", hostPath, err)
	}
	defer f.Close()
	for _, prog := range f.Progs {
		if prog.Type == elf.PT_INTERP {
			dynamic = true
			break
		}
	}
	switch f.Machine {
	case elf.EM_X86_64:
		return "amd64", dynamic, nil
	case elf.EM_AARCH64:
		return "arm64", dynamic, nil
	default:
		return "", dynamic, fmt.Errorf("unsupported ELF machine type: %s", f.Machine)
	}
}

// AddDir adds a directory entry to the image layer.
func (img *Image) AddDir(path string, mode int64) error {
	norm, err := normalizePath(path)
	if err != nil {
		return err
	}
	img.addEntry(File{Path: norm, Type: FileDirectory, Mode: mode})
	return nil
}

// AddSymlink adds a symbolic link to the image layer.
func (img *Image) AddSymlink(path, linkname string) error {
	norm, err := normalizePath(path)
	if err != nil {
		return err
	}
	img.addEntry(File{Path: norm, Type: FileSymlink, Linkname: linkname})
	return nil
}

func (img *Image) addEntry(f File) {
	if idx, ok := img.fileIdx[f.Path]; ok {
		img.files[idx] = f
	} else {
		img.fileIdx[f.Path] = len(img.files)
		img.files = append(img.files, f)
	}
}

// Build assembles the OCI config, manifest, and optional layer blob.
// It does not write anything to disk.
func (img *Image) Build() (*BuildResult, error) {
	if img.platform.OS == "" {
		return nil, fmt.Errorf("platform OS is required")
	}
	if img.platform.Architecture == "" {
		return nil, fmt.Errorf("platform Architecture is required")
	}

	diffIDs := []string{}
	var layerDescriptors []descriptor
	var layerBytes []byte
	var layerDigest string
	var layerSize int64

	if len(img.files) > 0 || img.forceEmptyLayer {
		lr, err := buildLayer(img.files)
		if err != nil {
			return nil, err
		}
		diffIDs = []string{lr.DiffID}
		layerDescriptors = []descriptor{{
			MediaType: MediaTypeLayerGzip,
			Digest:    lr.Digest,
			Size:      lr.Size,
		}}
		layerBytes = lr.Compressed
		layerDigest = lr.Digest
		layerSize = lr.Size
	}

	cfg := imageConfig{
		Created:      img.created,
		Architecture: img.platform.Architecture,
		OS:           img.platform.OS,
		Variant:      img.platform.Variant,
		Config: runtimeConfig{
			User:       img.user,
			WorkingDir: img.workdir,
			Env:        img.env,
			Entrypoint: img.entrypoint,
			Cmd:        img.cmd,
			Labels:     img.labels,
		},
		RootFS: rootFS{
			Type:    "layers",
			DiffIDs: diffIDs,
		},
	}

	configBytes, err := marshalConfig(cfg)
	if err != nil {
		return nil, err
	}
	configDigest := digestSHA256(configBytes)

	m := manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypeImageManifest,
		Config: descriptor{
			MediaType: MediaTypeImageConfig,
			Digest:    configDigest,
			Size:      int64(len(configBytes)),
		},
		Layers: layerDescriptors,
	}

	manifestBytes, err := marshalManifest(m)
	if err != nil {
		return nil, err
	}
	manifestDigest := digestSHA256(manifestBytes)

	return &BuildResult{
		ConfigDigest:   configDigest,
		ConfigSize:     int64(len(configBytes)),
		Config:         configBytes,
		ManifestDigest: manifestDigest,
		ManifestSize:   int64(len(manifestBytes)),
		Manifest:       manifestBytes,
		LayerDigest:    layerDigest,
		LayerSize:      layerSize,
		Layer:          layerBytes,
		DiffIDs:        diffIDs,
	}, nil
}
