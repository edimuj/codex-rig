package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edimuj/codex-rig/internal/rig"
)

func TestRCSetAndClear(t *testing.T) {
	repoRoot, cleanup := setupTempRepo(t)
	defer cleanup()

	if err := run([]string{"create", "default"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := run([]string{"rc", "set", "default"}); err != nil {
		t.Fatalf("rc set failed: %v", err)
	}

	markerPath := filepath.Join(repoRoot, ".codex-rig")
	markerRaw, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if strings.TrimSpace(string(markerRaw)) != "rig=default" {
		t.Fatalf("unexpected marker content: %q", string(markerRaw))
	}

	if err := run([]string{"rc", "clear"}); err != nil {
		t.Fatalf("rc clear failed: %v", err)
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("expected marker file to be removed, err=%v", err)
	}
}

func TestRCInitUsesCurrentRigWhenUnspecified(t *testing.T) {
	repoRoot, cleanup := setupTempRepo(t)
	defer cleanup()

	if err := run([]string{"create", "build"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := run([]string{"use", "--no-marker", "build"}); err != nil {
		t.Fatalf("use failed: %v", err)
	}
	if err := run([]string{"rc", "clear"}); err != nil {
		t.Fatalf("rc clear failed: %v", err)
	}

	if err := run([]string{"rc", "init"}); err != nil {
		t.Fatalf("rc init failed: %v", err)
	}

	markerPath := filepath.Join(repoRoot, ".codex-rig")
	markerRaw, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if strings.TrimSpace(string(markerRaw)) != "rig=build" {
		t.Fatalf("unexpected marker content: %q", string(markerRaw))
	}
}

func TestRCInitFailsWithoutRigOrCurrent(t *testing.T) {
	_, cleanup := setupTempRepo(t)
	defer cleanup()

	if err := run([]string{"rc", "init"}); err == nil {
		t.Fatal("expected rc init to fail when no rig specified and no current rig")
	}
}

func TestInstructionsSyncCreatesGeneratedOverride(t *testing.T) {
	_, cleanup := setupTempRepo(t)
	defer cleanup()

	if err := run([]string{"create", "default"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	globalHome := os.Getenv("CODEX_HOME")
	if err := os.MkdirAll(globalHome, 0o755); err != nil {
		t.Fatalf("mkdir global home: %v", err)
	}
	globalAgents := filepath.Join(globalHome, "AGENTS.md")
	if err := os.WriteFile(globalAgents, []byte("GLOBAL-INSTRUCTIONS"), 0o644); err != nil {
		t.Fatalf("write global AGENTS: %v", err)
	}

	if err := run([]string{"instructions", "sync", "--rig", "default"}); err != nil {
		t.Fatalf("instructions sync failed: %v", err)
	}

	rigRoot := os.Getenv("CODEX_RIG_ROOT")
	rigFragmentPath := filepath.Join(rigRoot, "rigs", "default", "AGENTS.rig.md")
	if _, err := os.Stat(rigFragmentPath); err != nil {
		t.Fatalf("expected rig fragment file, err=%v", err)
	}

	overridePath := filepath.Join(rigRoot, "rigs", "default", "codex-home", "AGENTS.override.md")
	overrideRaw, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read generated override: %v", err)
	}
	if !strings.Contains(string(overrideRaw), "GLOBAL-INSTRUCTIONS") {
		t.Fatalf("expected generated override to include global instructions")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	_, cleanup := setupTempRepo(t)
	defer cleanup()

	if err := run([]string{"create", "default"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	sourceRigRoot := os.Getenv("CODEX_RIG_ROOT")
	sourceHistory := filepath.Join(sourceRigRoot, "rigs", "default", "codex-home", "history", "session.log")
	if err := os.MkdirAll(filepath.Dir(sourceHistory), 0o755); err != nil {
		t.Fatalf("mkdir source history: %v", err)
	}
	if err := os.WriteFile(sourceHistory, []byte("backup-data"), 0o644); err != nil {
		t.Fatalf("write source history: %v", err)
	}

	bundlePath := filepath.Join(t.TempDir(), "default.codex-rig.tgz")
	if err := run([]string{"export", "--rig", "default", "--output", bundlePath}); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	targetRoot := filepath.Join(t.TempDir(), "imported-rig-root")
	targetGlobal := filepath.Join(t.TempDir(), "imported-global-codex")
	t.Setenv("CODEX_RIG_ROOT", targetRoot)
	t.Setenv("CODEX_HOME", targetGlobal)

	if err := run([]string{"import", "--set-current", bundlePath}); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	importedConfig := filepath.Join(targetRoot, "rigs", "default", rig.ConfigFileName)
	if _, err := os.Stat(importedConfig); err != nil {
		t.Fatalf("expected imported rig config, err=%v", err)
	}

	importedHistory := filepath.Join(targetRoot, "rigs", "default", "codex-home", "history", "session.log")
	importedRaw, err := os.ReadFile(importedHistory)
	if err != nil {
		t.Fatalf("read imported history: %v", err)
	}
	if string(importedRaw) != "backup-data" {
		t.Fatalf("unexpected imported history data: %q", string(importedRaw))
	}

	currentRaw, err := os.ReadFile(filepath.Join(targetRoot, rig.CurrentRigFileName))
	if err != nil {
		t.Fatalf("read current rig file: %v", err)
	}
	if strings.TrimSpace(string(currentRaw)) != "default" {
		t.Fatalf("unexpected current rig value: %q", string(currentRaw))
	}

	authPath := filepath.Join(targetRoot, "rigs", "default", "codex-home", "auth.json")
	authInfo, err := os.Lstat(authPath)
	if err != nil {
		t.Fatalf("lstat imported auth: %v", err)
	}
	if authInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected imported auth.json to be symlink")
	}
	authTarget, err := os.Readlink(authPath)
	if err != nil {
		t.Fatalf("readlink imported auth: %v", err)
	}
	if !filepath.IsAbs(authTarget) {
		authTarget = filepath.Clean(filepath.Join(filepath.Dir(authPath), authTarget))
	}
	expectedTarget := filepath.Join(targetGlobal, "auth.json")
	if authTarget != expectedTarget {
		t.Fatalf("unexpected imported auth target: got %s want %s", authTarget, expectedTarget)
	}
}

func setupTempRepo(t *testing.T) (repoRoot string, cleanup func()) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmp := t.TempDir()
	repoRoot = filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}

	t.Setenv("CODEX_RIG_ROOT", filepath.Join(tmp, "rig-root"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "global-codex"))

	cleanup = func() {
		_ = os.Chdir(oldWd)
	}
	return repoRoot, cleanup
}
