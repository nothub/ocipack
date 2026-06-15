package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"codeberg.org/fhuebner/ocipack"
)

type multiFlag []string

func (f *multiFlag) String() string     { return strings.Join(*f, ", ") }
func (f *multiFlag) Set(v string) error { *f = append(*f, v); return nil }

func parseMode(s string, def int64) (int64, error) {
	if s == "" {
		return def, nil
	}
	m, err := strconv.ParseInt(s, 8, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid mode %q: want octal e.g. 0755", s)
	}
	return m, nil
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "")
	var tag string
	flag.StringVar(&tag, "tag", "", "")
	var user string
	flag.StringVar(&user, "user", "", "")
	var entrypoint multiFlag
	flag.Var(&entrypoint, "entrypoint", "")
	var cmd multiFlag
	flag.Var(&cmd, "cmd", "")
	var workdir string
	flag.StringVar(&workdir, "workdir", "", "")
	var envVars multiFlag
	flag.Var(&envVars, "env", "")
	var labels multiFlag
	flag.Var(&labels, "label", "")
	var addFiles multiFlag
	flag.Var(&addFiles, "add-file", "")
	var addDirs multiFlag
	flag.Var(&addDirs, "add-dir", "")
	var addLinks multiFlag
	flag.Var(&addLinks, "add-link", "")
	var created string
	flag.StringVar(&created, "created", "", "")
	var cacertsPath string
	flag.StringVar(&cacertsPath, "cacerts-path", "", "")
	var nocacerts bool
	flag.BoolVar(&nocacerts, "no-cacerts", false, "")
	var notmp bool
	flag.BoolVar(&notmp, "no-tmp", false, "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage: ocipack [flags] <binary> <output>
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
`)
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("ocipack %s\n", buildVersion())
		os.Exit(0)
	}

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}
	binary, output := flag.Arg(0), flag.Arg(1)

	img := ocipack.New(ocipack.Platform{OS: "linux"})

	if err := img.AddBinary(binary); err != nil {
		if errors.Is(err, ocipack.ErrDynamicallyLinked) {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		} else {
			log.Fatalln(err)
		}
	}

	for _, spec := range addFiles {
		parts := strings.SplitN(spec, ":", 3)
		if len(parts) < 2 {
			log.Fatalf("-add-file %q: want container:host or container:host:mode", spec)
		}
		modeStr := ""
		if len(parts) == 3 {
			modeStr = parts[2]
		}
		mode, err := parseMode(modeStr, 0755)
		if err != nil {
			log.Fatalf("-add-file %q: %v", spec, err)
		}
		if err := img.AddFile(parts[0], parts[1], mode); err != nil {
			log.Fatalln(err)
		}
	}

	for _, spec := range addDirs {
		parts := strings.SplitN(spec, ":", 2)
		modeStr := ""
		if len(parts) == 2 {
			modeStr = parts[1]
		}
		mode, err := parseMode(modeStr, 0755)
		if err != nil {
			log.Fatalf("-add-dir %q: %v", spec, err)
		}
		if err := img.AddDir(parts[0], mode); err != nil {
			log.Fatalln(err)
		}
	}

	for _, spec := range addLinks {
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 {
			log.Fatalf("-add-link %q: want path:target", spec)
		}
		if err := img.AddSymlink(parts[0], parts[1]); err != nil {
			log.Fatalln(err)
		}
	}

	if len(entrypoint) > 0 {
		img.SetEntrypoint(entrypoint...)
	}
	if len(cmd) > 0 {
		img.SetCmd(cmd...)
	}
	if user != "" {
		img.SetUser(user)
	}
	if workdir != "" {
		img.SetWorkdir(workdir)
	}

	ts := time.Now()
	if created != "" {
		var err error
		ts, err = time.Parse(time.RFC3339, created)
		if err != nil {
			log.Fatalf("invalid -created %q: want RFC3339 e.g. 2024-01-15T12:00:00Z", created)
		}
	}
	img.SetCreated(ts)

	if !nocacerts {
		if err := img.AddCABundle(cacertsPath); err != nil {
			log.Fatalln(err)
		}
	}

	if !notmp {
		img.AddTmp()
	}

	for _, e := range envVars {
		k, v, _ := strings.Cut(e, "=")
		img.AddEnv(k, v)
	}

	for _, l := range labels {
		k, v, _ := strings.Cut(l, "=")
		img.SetLabel(k, v)
	}

	if err := img.WriteTar(output, tag); err != nil {
		log.Fatalln(err)
	}

	fmt.Println(output)
}
