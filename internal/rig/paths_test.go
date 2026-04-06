package rig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathsDefaults(t *testing.T) {
	home := "/tmp/home"
	paths, err := ResolvePaths(func(string) string { return "" }, home)
	if err != nil {
		t.Fatalf("ResolvePaths returned error: %v", err)
	}
	if paths.RigRoot != filepath.Clean(filepath.Join(home, ".codex-rig")) {
		t.Fatalf("unexpected rig root: %s", paths.RigRoot)
	}
	if paths.GlobalCodexHome != filepath.Clean(filepath.Join(home, ".codex")) {
		t.Fatalf("unexpected codex home: %s", paths.GlobalCodexHome)
	}
}

func TestResolvePathsEnvOverrides(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	env := map[string]string{
		"CODEX_RIG_ROOT": "~/rigs",
		"CODEX_HOME":     "relative/codex",
	}
	getenv := func(key string) string { return env[key] }

	home := "/home/tester"
	paths, err := ResolvePaths(getenv, home)
	if err != nil {
		t.Fatalf("ResolvePaths returned error: %v", err)
	}

	if paths.RigRoot != filepath.Clean(filepath.Join(home, "rigs")) {
		t.Fatalf("unexpected rig root: %s", paths.RigRoot)
	}
	if paths.GlobalCodexHome != filepath.Clean(filepath.Join(temp, "relative/codex")) {
		t.Fatalf("unexpected codex home: %s", paths.GlobalCodexHome)
	}
}
