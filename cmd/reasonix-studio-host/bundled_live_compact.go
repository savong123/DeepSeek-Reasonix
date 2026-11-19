package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"reasonix/internal/contract/config"
	"reasonix/internal/ext/installsource"
)

var liveCompactBundleDigest string

func provisionStudioBundles(logs io.Writer) {
	if runtime.GOOS != "windows" || liveCompactBundleDigest == "" {
		return
	}
	exe, err := os.Executable()
	if err == nil {
		err = provisionBundledLiveCompact(config.ReasonixHomeDir(), exe, liveCompactBundleDigest)
	}
	if err != nil {
		fmt.Fprintln(logs, "reasonix-studio-host: bundled Live Compact:", err)
	}
}

func provisionBundledLiveCompact(home, hostExe, digest string) error {
	const name = "live-compact"
	if !filepath.IsAbs(hostExe) {
		return fmt.Errorf("Studio host executable path must be absolute")
	}
	parent := filepath.Join(filepath.Dir(filepath.Dir(hostExe)), "bundled-plugins")
	root := filepath.Join(parent, name)
	for _, dir := range []string{parent, root} {
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("bundled plugin path is not a directory: %s", dir)
		}
	}
	return installsource.ProvisionBundledPlugin(home, name, root, "bin/live-compact.exe", digest)
}
