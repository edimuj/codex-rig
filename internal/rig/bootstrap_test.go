package rig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureRigBootstrapCreatesAwarenessSkillAndConfigEntry(t *testing.T) {
	temp := t.TempDir()
	store := NewStore(Paths{
		RigRoot:         filepath.Join(temp, "rig-root"),
		GlobalCodexHome: filepath.Join(temp, "global-codex"),
	})

	cfg, err := store.CreateRig("alpha")
	if err != nil {
		t.Fatalf("CreateRig: %v", err)
	}

	skillPath := filepath.Join(store.RigDir("alpha"), "bundled-skills", AwarenessSkillName, "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected bundled skill file, err=%v", err)
	}

	configPath := filepath.Join(store.RigCodexHome("alpha"), "config.toml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	pathLine := "path = " + `"` + filepath.Clean(skillPath) + `"`
	if !strings.Contains(string(content), pathLine) {
		t.Fatalf("expected config.toml to contain awareness path entry")
	}

	if err := store.SaveRigConfig(cfg); err != nil {
		t.Fatalf("SaveRigConfig: %v", err)
	}
	content2, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml (second): %v", err)
	}
	if strings.Count(string(content2), pathLine) != 1 {
		t.Fatalf("expected exactly one awareness path entry, got %d", strings.Count(string(content2), pathLine))
	}
}

func TestEnsureRigInstructionsMergesGlobalAndRigFragments(t *testing.T) {
	temp := t.TempDir()
	globalHome := filepath.Join(temp, "global-codex")
	if err := os.MkdirAll(globalHome, 0o755); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	globalFile := filepath.Join(globalHome, "AGENTS.md")
	if err := os.WriteFile(globalFile, []byte("GLOBAL-RULES"), 0o644); err != nil {
		t.Fatalf("write global AGENTS: %v", err)
	}

	store := NewStore(Paths{
		RigRoot:         filepath.Join(temp, "rig-root"),
		GlobalCodexHome: globalHome,
	})
	cfg, err := store.CreateRig("beta")
	if err != nil {
		t.Fatalf("CreateRig: %v", err)
	}

	rigFragment := filepath.Join(store.RigDir("beta"), RigInstructionFileName)
	if err := os.WriteFile(rigFragment, []byte("RIG-ONLY"), 0o644); err != nil {
		t.Fatalf("write rig fragment: %v", err)
	}
	if err := EnsureRigInstructions(store, cfg); err != nil {
		t.Fatalf("EnsureRigInstructions: %v", err)
	}

	overridePath := filepath.Join(store.RigCodexHome("beta"), GeneratedOverrideFileName)
	overrideRaw, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatalf("read generated override: %v", err)
	}
	override := string(overrideRaw)
	if !strings.Contains(override, "GLOBAL-RULES") {
		t.Fatalf("expected generated override to include global instructions")
	}
	if !strings.Contains(override, "RIG-ONLY") {
		t.Fatalf("expected generated override to include rig fragment")
	}
}
