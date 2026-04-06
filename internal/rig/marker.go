package rig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Marker struct {
	Rig string
}

func ParseMarker(raw []byte) (Marker, error) {
	lines := strings.Split(string(raw), "\n")
	values := make(map[string]string)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return Marker{}, fmt.Errorf("invalid marker line %q", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		values[key] = value
	}

	rigName := strings.TrimSpace(values["rig"])
	if rigName == "" {
		return Marker{}, errors.New("marker is missing rig")
	}

	return Marker{Rig: rigName}, nil
}

func FormatMarker(marker Marker) []byte {
	return []byte(fmt.Sprintf("rig=%s\n", marker.Rig))
}

func ReadMarker(path string) (Marker, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Marker{}, err
	}
	return ParseMarker(raw)
}

func WriteMarker(repoRoot, rigName string) error {
	if strings.TrimSpace(repoRoot) == "" {
		return errors.New("repo root is required")
	}
	if strings.TrimSpace(rigName) == "" {
		return errors.New("rig name is required")
	}
	markerPath := filepath.Join(repoRoot, MarkerFileName)
	return os.WriteFile(markerPath, FormatMarker(Marker{Rig: rigName}), 0o644)
}

func FindMarker(startPath string) (markerPath string, marker Marker, found bool, err error) {
	dir, err := startDir(startPath)
	if err != nil {
		return "", Marker{}, false, err
	}

	for {
		candidate := filepath.Join(dir, MarkerFileName)
		if _, statErr := os.Stat(candidate); statErr == nil {
			parsed, parseErr := ReadMarker(candidate)
			if parseErr != nil {
				return "", Marker{}, false, parseErr
			}
			return candidate, parsed, true, nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", Marker{}, false, statErr
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", Marker{}, false, nil
}

func FindRepoRoot(startPath string) (string, error) {
	dir, err := startDir(startPath)
	if err != nil {
		return "", err
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		if _, statErr := os.Stat(gitPath); statErr == nil {
			return dir, nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.New("not inside a git repository")
}

func startDir(startPath string) (string, error) {
	if strings.TrimSpace(startPath) == "" {
		return "", errors.New("start path is required")
	}
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return filepath.Dir(absPath), nil
	}
	return absPath, nil
}
