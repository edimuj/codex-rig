package rig

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ConfigFileName     = "rig.json"
	CurrentRigFileName = "current"
	MarkerFileName     = ".codex-rig"
)

type PolicyMode string

const (
	PolicyShared    PolicyMode = "shared"
	PolicyIsolated  PolicyMode = "isolated"
	PolicyInherited PolicyMode = "inherited"
)

const (
	CategoryAuth        = "auth"
	CategorySkills      = "skills"
	CategoryPlugins     = "plugins"
	CategoryMCP         = "mcp"
	CategoryHistoryLogs = "history/logs"
)

const (
	AuthSourceGlobal = "global"
	AuthSourceRig    = "rig:"
)

var ManagedCategories = []string{
	CategoryAuth,
	CategorySkills,
	CategoryPlugins,
	CategoryMCP,
	CategoryHistoryLogs,
}

type ManagedTarget struct {
	RelativePath string
	IsDir        bool
}

var CategoryTargets = map[string][]ManagedTarget{
	CategoryAuth: {
		{RelativePath: "auth.json", IsDir: false},
	},
	CategorySkills: {
		{RelativePath: "skills", IsDir: true},
	},
	CategoryPlugins: {
		{RelativePath: "plugins", IsDir: true},
	},
	CategoryMCP: {
		{RelativePath: "mcp", IsDir: true},
	},
	CategoryHistoryLogs: {
		{RelativePath: "history", IsDir: true},
		{RelativePath: "logs", IsDir: true},
	},
}

type RigConfig struct {
	Version   int                   `json:"version"`
	Name      string                `json:"name"`
	CreatedAt time.Time             `json:"created_at"`
	Policy    map[string]PolicyMode `json:"policy"`
	Links     map[string]string     `json:"links,omitempty"`
}

func DefaultPolicy() map[string]PolicyMode {
	return map[string]PolicyMode{
		CategoryAuth:        PolicyShared,
		CategorySkills:      PolicyShared,
		CategoryPlugins:     PolicyShared,
		CategoryMCP:         PolicyShared,
		CategoryHistoryLogs: PolicyIsolated,
	}
}

func NewRigConfig(name string) RigConfig {
	return RigConfig{
		Version:   1,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		Policy:    DefaultPolicy(),
		Links: map[string]string{
			CategoryAuth: AuthSourceGlobal,
		},
	}
}

func ParseRigConfig(raw []byte) (RigConfig, error) {
	var cfg RigConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return RigConfig{}, err
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return RigConfig{}, err
	}
	return cfg, nil
}

func MarshalRigConfig(cfg RigConfig) ([]byte, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func (c *RigConfig) Normalize() {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}

	defaults := DefaultPolicy()
	if c.Policy == nil {
		c.Policy = defaults
	} else {
		for category, mode := range defaults {
			if _, ok := c.Policy[category]; !ok {
				c.Policy[category] = mode
			}
		}
	}

	if c.Links == nil {
		c.Links = map[string]string{}
	}
	if c.Policy[CategoryAuth] == PolicyShared {
		c.Links[CategoryAuth] = NormalizeAuthLinkSource(c.Links[CategoryAuth])
	}
}

func (c RigConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("rig name is required")
	}
	if c.Version <= 0 {
		return fmt.Errorf("invalid rig config version: %d", c.Version)
	}
	for category, mode := range c.Policy {
		if _, ok := CategoryTargets[category]; !ok {
			return fmt.Errorf("unknown category %q", category)
		}
		switch mode {
		case PolicyShared, PolicyIsolated:
		case PolicyInherited:
			if !SupportsInherited(category) {
				return fmt.Errorf("category %q does not support inherited mode", category)
			}
		default:
			return fmt.Errorf("invalid policy mode %q for category %q", mode, category)
		}
	}
	for _, category := range ManagedCategories {
		if _, ok := c.Policy[category]; !ok {
			return fmt.Errorf("missing policy for category %q", category)
		}
	}

	for category, source := range c.Links {
		if category != CategoryAuth {
			return fmt.Errorf("links for category %q are not supported", category)
		}
		kind, targetRig, err := ParseAuthLinkSource(source)
		if err != nil {
			return err
		}
		if kind == AuthSourceRig {
			if err := ValidateRigName(targetRig); err != nil {
				return fmt.Errorf("invalid auth link source rig %q: %w", targetRig, err)
			}
			if targetRig == c.Name {
				return errors.New("auth link source cannot reference itself")
			}
		}
	}

	if c.Policy[CategoryAuth] == PolicyShared {
		if _, _, err := ParseAuthLinkSource(c.AuthLinkSource()); err != nil {
			return err
		}
	}

	return nil
}

func SupportsInherited(category string) bool {
	switch category {
	case CategorySkills, CategoryPlugins:
		return true
	default:
		return false
	}
}

func IsManagedCategory(category string) bool {
	_, ok := CategoryTargets[category]
	return ok
}

func NormalizeCategory(category string) string {
	normalized := strings.ToLower(strings.TrimSpace(category))
	switch normalized {
	case "history", "logs", "history_logs":
		return CategoryHistoryLogs
	default:
		return normalized
	}
}

func NormalizeAuthLinkSource(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return AuthSourceGlobal
	}
	if strings.EqualFold(trimmed, AuthSourceGlobal) {
		return AuthSourceGlobal
	}
	if strings.HasPrefix(trimmed, AuthSourceRig) {
		return AuthSourceRig + strings.TrimSpace(strings.TrimPrefix(trimmed, AuthSourceRig))
	}
	return trimmed
}

func ParseAuthLinkSource(source string) (kind string, rigName string, err error) {
	normalized := NormalizeAuthLinkSource(source)
	if normalized == AuthSourceGlobal {
		return AuthSourceGlobal, "", nil
	}
	if strings.HasPrefix(normalized, AuthSourceRig) {
		rigName = strings.TrimSpace(strings.TrimPrefix(normalized, AuthSourceRig))
		if rigName == "" {
			return "", "", errors.New("auth link source requires rig name")
		}
		return AuthSourceRig, rigName, nil
	}
	return "", "", fmt.Errorf("invalid auth link source %q (expected %q or %q<name>)", source, AuthSourceGlobal, AuthSourceRig)
}

func (c RigConfig) AuthLinkSource() string {
	if c.Links == nil {
		return AuthSourceGlobal
	}
	return NormalizeAuthLinkSource(c.Links[CategoryAuth])
}
