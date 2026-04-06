package rig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInheritedPolicyPreservesLocalOverridesAndCleansStaleLinks(t *testing.T) {
	temp := t.TempDir()
	paths := Paths{
		RigRoot:         filepath.Join(temp, "rig-root"),
		GlobalCodexHome: filepath.Join(temp, "global-codex"),
	}
	store := NewStore(paths)

	if err := os.MkdirAll(filepath.Join(paths.GlobalCodexHome, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir global skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.GlobalCodexHome, "skills", "shared.md"), []byte("shared"), 0o644); err != nil {
		t.Fatalf("write shared.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.GlobalCodexHome, "skills", "stale.md"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale.md: %v", err)
	}

	cfg, err := store.CreateRig("alpha")
	if err != nil {
		t.Fatalf("CreateRig: %v", err)
	}
	cfg.Policy[CategorySkills] = PolicyInherited
	if err := store.SaveRigConfig(cfg); err != nil {
		t.Fatalf("SaveRigConfig: %v", err)
	}

	skillsDir := filepath.Join(store.RigCodexHome("alpha"), "skills")
	sharedPath := filepath.Join(skillsDir, "shared.md")
	stalePath := filepath.Join(skillsDir, "stale.md")

	if info, err := os.Lstat(sharedPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected shared.md to be symlink, err=%v", err)
	}
	if info, err := os.Lstat(stalePath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected stale.md to be symlink, err=%v", err)
	}

	if err := os.Remove(sharedPath); err != nil {
		t.Fatalf("remove shared symlink: %v", err)
	}
	if err := os.WriteFile(sharedPath, []byte("local override"), 0o644); err != nil {
		t.Fatalf("write local override: %v", err)
	}

	if err := os.Remove(filepath.Join(paths.GlobalCodexHome, "skills", "stale.md")); err != nil {
		t.Fatalf("remove global stale.md: %v", err)
	}
	if err := EnsurePolicyState(store, cfg); err != nil {
		t.Fatalf("EnsurePolicyState: %v", err)
	}

	overrideData, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("read override file: %v", err)
	}
	if string(overrideData) != "local override" {
		t.Fatalf("override content was not preserved, got %q", string(overrideData))
	}
	if _, err := os.Lstat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale symlink to be removed, err=%v", err)
	}
}

func TestAuthRigToRigLinkUsesSourceRigPath(t *testing.T) {
	temp := t.TempDir()
	paths := Paths{
		RigRoot:         filepath.Join(temp, "rig-root"),
		GlobalCodexHome: filepath.Join(temp, "global-codex"),
	}
	store := NewStore(paths)

	if _, err := store.CreateRig("source"); err != nil {
		t.Fatalf("CreateRig(source): %v", err)
	}
	targetCfg, err := store.CreateRig("target")
	if err != nil {
		t.Fatalf("CreateRig(target): %v", err)
	}

	targetCfg.Policy[CategoryAuth] = PolicyShared
	targetCfg.Links[CategoryAuth] = "rig:source"
	if err := store.SaveRigConfig(targetCfg); err != nil {
		t.Fatalf("SaveRigConfig(target): %v", err)
	}

	authPath := filepath.Join(store.RigCodexHome("target"), "auth.json")
	info, err := os.Lstat(authPath)
	if err != nil {
		t.Fatalf("lstat target auth: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected target auth.json to be symlink")
	}

	resolved, err := resolveLink(authPath)
	if err != nil {
		t.Fatalf("resolveLink: %v", err)
	}
	expected := filepath.Join(store.RigCodexHome("source"), "auth.json")
	if resolved != expected {
		t.Fatalf("unexpected auth link target: got %s want %s", resolved, expected)
	}
}
