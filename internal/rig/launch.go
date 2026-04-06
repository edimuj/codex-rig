package rig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

func ResolveLaunchRig(store *Store, cwd, explicitRig string) (rigName string, markerPath string, err error) {
	if strings.TrimSpace(explicitRig) != "" {
		if _, loadErr := store.LoadRig(explicitRig); loadErr != nil {
			return "", "", loadErr
		}
		return explicitRig, "", nil
	}

	markerPath, marker, found, markerErr := FindMarker(cwd)
	if markerErr != nil {
		return "", "", markerErr
	}
	if found {
		if _, loadErr := store.LoadRig(marker.Rig); loadErr != nil {
			return "", "", fmt.Errorf("marker references unknown rig %q", marker.Rig)
		}
		return marker.Rig, markerPath, nil
	}

	current, currentErr := store.CurrentRig()
	if currentErr != nil {
		return "", "", currentErr
	}
	if current == "" {
		return "", "", errors.New("no rig selected (use `codex-rig use <name>` or pass --rig)")
	}
	if _, loadErr := store.LoadRig(current); loadErr != nil {
		return "", "", loadErr
	}
	return current, "", nil
}

func BuildLaunchEnv(baseEnv []string, rigRoot, rigName, rigCodexHome string) []string {
	pairs := map[string]string{
		"CODEX_RIG":      rigName,
		"CODEX_RIG_ROOT": filepath.Clean(rigRoot),
		"CODEX_HOME":     filepath.Clean(rigCodexHome),
	}

	env := make([]string, 0, len(baseEnv)+len(pairs))
	seen := make(map[string]bool, len(pairs))
	for _, kv := range baseEnv {
		key := kv
		if idx := strings.Index(kv, "="); idx >= 0 {
			key = kv[:idx]
		}
		if value, ok := pairs[key]; ok {
			env = append(env, key+"="+value)
			seen[key] = true
			continue
		}
		env = append(env, kv)
	}

	for key, value := range pairs {
		if !seen[key] {
			env = append(env, key+"="+value)
		}
	}

	return env
}
