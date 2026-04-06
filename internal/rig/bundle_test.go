package rig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportImportRoundTrip(t *testing.T) {
	temp := t.TempDir()

	sourceStore := NewStore(Paths{
		RigRoot:         filepath.Join(temp, "source-rig-root"),
		GlobalCodexHome: filepath.Join(temp, "source-global-codex"),
	})
	cfg, err := sourceStore.CreateRig("alpha")
	if err != nil {
		t.Fatalf("CreateRig(source): %v", err)
	}

	historyFile := filepath.Join(sourceStore.RigCodexHome("alpha"), "history", "session.log")
	if err := os.MkdirAll(filepath.Dir(historyFile), 0o755); err != nil {
		t.Fatalf("mkdir history: %v", err)
	}
	if err := os.WriteFile(historyFile, []byte("history-data"), 0o644); err != nil {
		t.Fatalf("write history file: %v", err)
	}

	bundlePath := filepath.Join(temp, "alpha.codex-rig.tgz")
	manifest, err := ExportRig(sourceStore, cfg, bundlePath, "test")
	if err != nil {
		t.Fatalf("ExportRig: %v", err)
	}
	if manifest.RigName != "alpha" {
		t.Fatalf("unexpected manifest rig: %s", manifest.RigName)
	}

	targetStore := NewStore(Paths{
		RigRoot:         filepath.Join(temp, "target-rig-root"),
		GlobalCodexHome: filepath.Join(temp, "target-global-codex"),
	})
	importedCfg, err := ImportRig(targetStore, bundlePath, "", false)
	if err != nil {
		t.Fatalf("ImportRig: %v", err)
	}
	if importedCfg.Name != "alpha" {
		t.Fatalf("unexpected imported rig: %s", importedCfg.Name)
	}

	importedHistory := filepath.Join(targetStore.RigCodexHome("alpha"), "history", "session.log")
	historyRaw, err := os.ReadFile(importedHistory)
	if err != nil {
		t.Fatalf("read imported history: %v", err)
	}
	if string(historyRaw) != "history-data" {
		t.Fatalf("unexpected history content: %q", string(historyRaw))
	}

	authPath := filepath.Join(targetStore.RigCodexHome("alpha"), "auth.json")
	info, err := os.Lstat(authPath)
	if err != nil {
		t.Fatalf("lstat imported auth: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected imported auth to be symlink")
	}
	resolvedAuth, err := resolveLink(authPath)
	if err != nil {
		t.Fatalf("resolve imported auth symlink: %v", err)
	}
	expectedAuth := filepath.Join(targetStore.GlobalCodexHome, "auth.json")
	if resolvedAuth != expectedAuth {
		t.Fatalf("unexpected auth symlink target: got %s want %s", resolvedAuth, expectedAuth)
	}
}

func TestImportRigOverwrite(t *testing.T) {
	temp := t.TempDir()

	sourceStore := NewStore(Paths{
		RigRoot:         filepath.Join(temp, "source-rig-root"),
		GlobalCodexHome: filepath.Join(temp, "source-global-codex"),
	})
	sourceCfg, err := sourceStore.CreateRig("alpha")
	if err != nil {
		t.Fatalf("CreateRig(source): %v", err)
	}
	newFile := filepath.Join(sourceStore.RigCodexHome("alpha"), "history", "state.txt")
	if err := os.MkdirAll(filepath.Dir(newFile), 0o755); err != nil {
		t.Fatalf("mkdir source history: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	bundlePath := filepath.Join(temp, "alpha-bundle.tgz")
	if _, err := ExportRig(sourceStore, sourceCfg, bundlePath, "test"); err != nil {
		t.Fatalf("ExportRig: %v", err)
	}

	targetStore := NewStore(Paths{
		RigRoot:         filepath.Join(temp, "target-rig-root"),
		GlobalCodexHome: filepath.Join(temp, "target-global-codex"),
	})
	if _, err := targetStore.CreateRig("alpha"); err != nil {
		t.Fatalf("CreateRig(target): %v", err)
	}
	oldFile := filepath.Join(targetStore.RigCodexHome("alpha"), "history", "state.txt")
	if err := os.MkdirAll(filepath.Dir(oldFile), 0o755); err != nil {
		t.Fatalf("mkdir target history: %v", err)
	}
	if err := os.WriteFile(oldFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	if _, err := ImportRig(targetStore, bundlePath, "", false); err == nil {
		t.Fatal("expected ImportRig without overwrite to fail for existing rig")
	}

	if _, err := ImportRig(targetStore, bundlePath, "", true); err != nil {
		t.Fatalf("ImportRig overwrite: %v", err)
	}
	updatedRaw, err := os.ReadFile(oldFile)
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if string(updatedRaw) != "new" {
		t.Fatalf("expected overwritten data to be \"new\", got %q", string(updatedRaw))
	}
}
