package rig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	RigRoot         string
	GlobalCodexHome string
}

func ResolvePathsForCurrentUser() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return ResolvePaths(os.Getenv, homeDir)
}

func ResolvePaths(getenv func(string) string, homeDir string) (Paths, error) {
	if strings.TrimSpace(homeDir) == "" {
		return Paths{}, errors.New("home directory is required")
	}

	rigRoot := getenv("CODEX_RIG_ROOT")
	if strings.TrimSpace(rigRoot) == "" {
		rigRoot = filepath.Join(homeDir, ".codex-rig")
	}

	globalCodexHome := getenv("CODEX_HOME")
	if strings.TrimSpace(globalCodexHome) == "" {
		globalCodexHome = filepath.Join(homeDir, ".codex")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return Paths{}, err
	}

	return Paths{
		RigRoot:         normalizePath(rigRoot, homeDir, cwd),
		GlobalCodexHome: normalizePath(globalCodexHome, homeDir, cwd),
	}, nil
}

func normalizePath(pathValue, homeDir, cwd string) string {
	trimmed := strings.TrimSpace(pathValue)
	if strings.HasPrefix(trimmed, "~/") {
		trimmed = filepath.Join(homeDir, strings.TrimPrefix(trimmed, "~/"))
	} else if trimmed == "~" {
		trimmed = homeDir
	}

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}

	return filepath.Clean(filepath.Join(cwd, trimmed))
}
