package installsource

import (
	"fmt"
	"os"
	"path/filepath"

	"reasonix/internal/ext/pluginpkg"
)

// ProvisionBundledPlugin registers a host-pinned release resource without
// activating it. The normal plugin switch owns activation.
func ProvisionBundledPlugin(home, name, sourceRoot, runtimeFile, digest string) error {
	return pluginpkg.ProvisionDefault(home, name, func() (pluginpkg.InstalledPlugin, func() error, func(), error) {
		p := pluginpkg.InstalledPlugin{}
		if err := verifyPluginDigest(sourceRoot, digest); err != nil {
			return p, nil, nil, err
		}
		pkg, _, err := pluginpkg.ParseDir(sourceRoot)
		if err != nil {
			return p, nil, nil, err
		}
		rt := pkg.Manifest.Runtime
		if pkg.ManifestKind != "reasonix" || pkg.Manifest.Name != name || rt == nil || rt.Command != "${REASONIX_PLUGIN_ROOT}/"+runtimeFile {
			return p, nil, nil, fmt.Errorf("unexpected manifest for bundled plugin %q", name)
		}
		info, err := os.Lstat(filepath.Join(sourceRoot, filepath.FromSlash(runtimeFile)))
		if err != nil {
			return p, nil, nil, err
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return p, nil, nil, fmt.Errorf("bundled plugin %q has no regular, nonempty runtime", name)
		}
		// An unreferenced copy must not block a retry after a crash.
		root, err := stagePluginCopy(pkg, sourceRoot, pluginpkg.InstallRoot(home, name), "", digest)
		if err != nil {
			return p, nil, nil, err
		}
		return pluginpkg.InstalledPlugin{
			Root:         pluginpkg.RelativeRoot(home, root),
			Version:      pkg.Manifest.Version,
			Description:  pkg.Manifest.Description,
			ManifestKind: pkg.ManifestKind,
		}, func() error { return verifyPluginDigest(root, digest) }, func() { _ = os.RemoveAll(root) }, nil
	})
}
