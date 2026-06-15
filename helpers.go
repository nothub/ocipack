package ocipack

import (
	"fmt"
	"os"
	"strings"
)

// caBundlePaths is the fallback chain used by AddCABundle to find the system
// CA certificate bundle. Mirrors the list used by Go's crypto/x509 on Linux.
var caBundlePaths = []string{
	"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo
	"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
	"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
	"/etc/pki/tls/cacert.pem",                           // OpenELEC
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
	"/etc/ssl/cert.pem",                                 // Alpine Linux
	"/etc/ssl/certs/ca-bundle.crt",                      // NixOS
}

// AddCABundle adds the CA bundle at /etc/ssl/certs/ca-certificates.crt (mode 0644).
// Pass "" to auto-detect from caBundlePaths; pass a path to use a specific file.
func (img *Image) AddCABundle(path string) error {
	var (
		data []byte
		err  error
	)
	if path == "" {
		paths := caBundlePaths
		if p := os.Getenv("SSL_CERT_FILE"); p != "" {
			paths = append([]string{p}, paths...)
		}
		for _, p := range paths {
			data, err = os.ReadFile(p)
			if err == nil {
				break
			}
		}
		if data == nil {
			return fmt.Errorf("no CA bundle found; tried: %s", strings.Join(paths, ", "))
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return err
		}
	}
	img.addEntry(File{Path: "etc/ssl/certs/ca-certificates.crt", Type: FileRegular, Data: data, Mode: 0644})
	return nil
}

// AddTmp adds /tmp as a world-writable directory (mode 01777).
func (img *Image) AddTmp() {
	img.addEntry(File{Path: "tmp", Type: FileDirectory, Mode: 01777})
}
