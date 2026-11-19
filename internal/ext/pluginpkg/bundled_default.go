package pluginpkg

import (
	"fmt"
	"slices"
)

// ProvisionDefault records the one-time offer and its disabled entry together.
// An existing install satisfies the offer without replacing the user's entry.
func ProvisionDefault(home, name string, prepare func() (p InstalledPlugin, validate func() error, discard func(), err error)) error {
	if !IsValidName(name) {
		return fmt.Errorf("invalid plugin name %q", name)
	}
	if done, err := publishBundledDefault(home, name, nil, nil); done || err != nil {
		return err
	}
	p, validate, discard, err := prepare()
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			discard()
		}
	}()
	published, err = publishBundledDefault(home, name, &p, validate)
	return err
}

func publishBundledDefault(home, name string, prepared *InstalledPlugin, validate func() error) (bool, error) {
	stateMu.Lock()
	defer stateMu.Unlock()
	st, err := LoadState(home)
	if err != nil {
		return false, err
	}
	if slices.Contains(st.BundledDefaults, name) {
		return prepared == nil, nil
	}
	st.BundledDefaults = append(st.BundledDefaults, name)
	slices.Sort(st.BundledDefaults)
	for _, p := range st.Plugins {
		if p.Name == name {
			return prepared == nil, SaveState(home, st)
		}
	}
	if prepared == nil {
		return false, nil
	}
	p := *prepared
	p.Name, p.Enabled, p.Generation = name, false, 1
	st.Plugins = append(st.Plugins, p)
	if err := saveStateValidated(home, st, validate); err != nil {
		return false, err
	}
	return true, nil
}
