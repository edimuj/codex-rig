package rig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var rigNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type Store struct {
	RigRoot         string
	GlobalCodexHome string
}

func NewStore(paths Paths) *Store {
	return &Store{RigRoot: paths.RigRoot, GlobalCodexHome: paths.GlobalCodexHome}
}

func ValidateRigName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("rig name is required")
	}
	if !rigNamePattern.MatchString(trimmed) {
		return fmt.Errorf("invalid rig name %q (expected %s)", name, rigNamePattern.String())
	}
	return nil
}

func (s *Store) EnsureRoot() error {
	return os.MkdirAll(s.RigsDir(), 0o755)
}

func (s *Store) RigsDir() string {
	return filepath.Join(s.RigRoot, "rigs")
}

func (s *Store) RigDir(name string) string {
	return filepath.Join(s.RigsDir(), name)
}

func (s *Store) RigConfigPath(name string) string {
	return filepath.Join(s.RigDir(name), ConfigFileName)
}

func (s *Store) RigCodexHome(name string) string {
	return filepath.Join(s.RigDir(name), "codex-home")
}

func (s *Store) CurrentRigPath() string {
	return filepath.Join(s.RigRoot, CurrentRigFileName)
}

func (s *Store) CreateRig(name string) (RigConfig, error) {
	if err := ValidateRigName(name); err != nil {
		return RigConfig{}, err
	}
	if err := s.EnsureRoot(); err != nil {
		return RigConfig{}, err
	}

	rigDir := s.RigDir(name)
	if _, err := os.Stat(rigDir); err == nil {
		return RigConfig{}, fmt.Errorf("rig %q already exists", name)
	} else if !os.IsNotExist(err) {
		return RigConfig{}, err
	}

	if err := os.MkdirAll(s.RigCodexHome(name), 0o755); err != nil {
		return RigConfig{}, err
	}

	cfg := NewRigConfig(name)
	if err := s.SaveRigConfig(cfg); err != nil {
		return RigConfig{}, err
	}
	return cfg, nil
}

func (s *Store) SaveRigConfig(cfg RigConfig) error {
	if err := s.WriteRigConfig(cfg); err != nil {
		return err
	}
	return EnsureRigBootstrap(s, cfg)
}

func (s *Store) WriteRigConfig(cfg RigConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	raw, err := MarshalRigConfig(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.RigDir(cfg.Name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.RigConfigPath(cfg.Name), raw, 0o644)
}

func (s *Store) LoadRig(name string) (RigConfig, error) {
	raw, err := os.ReadFile(s.RigConfigPath(name))
	if err != nil {
		return RigConfig{}, err
	}
	cfg, err := ParseRigConfig(raw)
	if err != nil {
		return RigConfig{}, err
	}
	if cfg.Name != name {
		return RigConfig{}, fmt.Errorf("config name mismatch: expected %q, got %q", name, cfg.Name)
	}
	return cfg, nil
}

func (s *Store) RigExists(name string) bool {
	_, err := os.Stat(s.RigConfigPath(name))
	return err == nil
}

func (s *Store) ListRigs() ([]string, error) {
	entries, err := os.ReadDir(s.RigsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	rigs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if s.RigExists(name) {
			rigs = append(rigs, name)
		}
	}
	sort.Strings(rigs)
	return rigs, nil
}

func (s *Store) SetCurrentRig(name string) error {
	if _, err := s.LoadRig(name); err != nil {
		return err
	}
	if err := os.MkdirAll(s.RigRoot, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.CurrentRigPath(), []byte(name+"\n"), 0o644)
}

func (s *Store) CurrentRig() (string, error) {
	raw, err := os.ReadFile(s.CurrentRigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func (s *Store) ResolveAuthSourcePath(cfg RigConfig) (string, error) {
	target, err := s.resolveSharedSourcePath(cfg, CategoryAuth, CategoryTargets[CategoryAuth][0])
	if err != nil {
		return "", err
	}
	return target, nil
}
