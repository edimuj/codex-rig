package rig

import (
	"encoding/json"
	"testing"
)

func TestParseRigConfigFillsDefaultPolicy(t *testing.T) {
	raw := []byte(`{"version":1,"name":"alpha"}`)
	cfg, err := ParseRigConfig(raw)
	if err != nil {
		t.Fatalf("ParseRigConfig: %v", err)
	}
	for _, category := range ManagedCategories {
		if _, ok := cfg.Policy[category]; !ok {
			t.Fatalf("missing category policy %s", category)
		}
	}
}

func TestParseRigConfigRejectsInvalidPolicyMode(t *testing.T) {
	cfg := map[string]any{
		"version": 1,
		"name":    "alpha",
		"policy": map[string]string{
			CategoryAuth:        string(PolicyShared),
			CategorySkills:      "wat",
			CategoryPlugins:     string(PolicyShared),
			CategoryMCP:         string(PolicyShared),
			CategoryHistoryLogs: string(PolicyIsolated),
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if _, err := ParseRigConfig(raw); err == nil {
		t.Fatal("expected ParseRigConfig to fail with invalid policy mode")
	}
}

func TestParseRigConfigAllowsInheritedSkills(t *testing.T) {
	cfg := map[string]any{
		"version": 1,
		"name":    "alpha",
		"policy": map[string]string{
			CategoryAuth:        string(PolicyShared),
			CategorySkills:      string(PolicyInherited),
			CategoryPlugins:     string(PolicyShared),
			CategoryMCP:         string(PolicyShared),
			CategoryHistoryLogs: string(PolicyIsolated),
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	parsed, err := ParseRigConfig(raw)
	if err != nil {
		t.Fatalf("ParseRigConfig: %v", err)
	}
	if parsed.Policy[CategorySkills] != PolicyInherited {
		t.Fatalf("expected skills policy %q, got %q", PolicyInherited, parsed.Policy[CategorySkills])
	}
}

func TestParseRigConfigRejectsInheritedAuth(t *testing.T) {
	cfg := map[string]any{
		"version": 1,
		"name":    "alpha",
		"policy": map[string]string{
			CategoryAuth:        string(PolicyInherited),
			CategorySkills:      string(PolicyShared),
			CategoryPlugins:     string(PolicyShared),
			CategoryMCP:         string(PolicyShared),
			CategoryHistoryLogs: string(PolicyIsolated),
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if _, err := ParseRigConfig(raw); err == nil {
		t.Fatal("expected ParseRigConfig to fail for inherited auth")
	}
}

func TestParseRigConfigRejectsSelfAuthSource(t *testing.T) {
	cfg := map[string]any{
		"version": 1,
		"name":    "alpha",
		"policy": map[string]string{
			CategoryAuth:        string(PolicyShared),
			CategorySkills:      string(PolicyShared),
			CategoryPlugins:     string(PolicyShared),
			CategoryMCP:         string(PolicyShared),
			CategoryHistoryLogs: string(PolicyIsolated),
		},
		"links": map[string]string{
			CategoryAuth: "rig:alpha",
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if _, err := ParseRigConfig(raw); err == nil {
		t.Fatal("expected ParseRigConfig to reject self auth source")
	}
}
