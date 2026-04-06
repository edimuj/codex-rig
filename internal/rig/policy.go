package rig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func EnsurePolicyState(store *Store, cfg RigConfig) error {
	rigCodexHome := store.RigCodexHome(cfg.Name)
	for _, category := range ManagedCategories {
		mode := cfg.Policy[category]
		targets := CategoryTargets[category]
		for _, target := range targets {
			localPath := filepath.Join(rigCodexHome, target.RelativePath)
			sourcePath, err := store.resolveSharedSourcePath(cfg, category, target)
			if err != nil {
				return fmt.Errorf("%s: %w", category, err)
			}

			switch mode {
			case PolicyShared:
				if err := ensureShared(localPath, sourcePath, target.IsDir); err != nil {
					return fmt.Errorf("%s: %w", category, err)
				}
			case PolicyIsolated:
				if err := ensureIsolated(localPath, target.IsDir); err != nil {
					return fmt.Errorf("%s: %w", category, err)
				}
			case PolicyInherited:
				if !target.IsDir || !SupportsInherited(category) {
					return fmt.Errorf("category %q target %q does not support inherited mode", category, target.RelativePath)
				}
				if err := ensureInheritedDirectory(localPath, sourcePath); err != nil {
					return fmt.Errorf("%s: %w", category, err)
				}
			default:
				return fmt.Errorf("unsupported mode %q for category %q", mode, category)
			}
		}
	}
	return nil
}

func (s *Store) resolveSharedSourcePath(cfg RigConfig, category string, target ManagedTarget) (string, error) {
	defaultSource := filepath.Join(s.GlobalCodexHome, target.RelativePath)
	if category != CategoryAuth {
		return defaultSource, nil
	}

	kind, rigName, err := ParseAuthLinkSource(cfg.AuthLinkSource())
	if err != nil {
		return "", err
	}
	switch kind {
	case AuthSourceGlobal:
		return defaultSource, nil
	case AuthSourceRig:
		if rigName == cfg.Name {
			return "", errors.New("auth source rig cannot reference itself")
		}
		if _, loadErr := s.LoadRig(rigName); loadErr != nil {
			return "", fmt.Errorf("auth source rig %q: %w", rigName, loadErr)
		}
		return filepath.Join(s.RigCodexHome(rigName), target.RelativePath), nil
	default:
		return "", fmt.Errorf("unsupported auth source kind %q", kind)
	}
}

func ensureShared(localPath, sourcePath string, isDir bool) error {
	if err := ensureTarget(sourcePath, isDir); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}

	if info, err := os.Lstat(localPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolvedTarget, readErr := resolveLink(localPath)
			if readErr != nil {
				return readErr
			}
			if filepath.Clean(resolvedTarget) == filepath.Clean(sourcePath) {
				return nil
			}
		}
		if removeErr := os.RemoveAll(localPath); removeErr != nil {
			return removeErr
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.Symlink(sourcePath, localPath)
}

func ensureIsolated(localPath string, isDir bool) error {
	if info, err := os.Lstat(localPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if removeErr := os.Remove(localPath); removeErr != nil {
				return removeErr
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return ensureTarget(localPath, isDir)
}

func ensureInheritedDirectory(localPath, sourcePath string) error {
	if err := ensureTarget(sourcePath, true); err != nil {
		return err
	}
	if err := ensureNonSymlinkDirectory(localPath); err != nil {
		return err
	}

	sourceEntries, err := os.ReadDir(sourcePath)
	if err != nil {
		return err
	}

	for _, entry := range sourceEntries {
		sourceEntryPath := filepath.Join(sourcePath, entry.Name())
		localEntryPath := filepath.Join(localPath, entry.Name())
		if _, statErr := os.Lstat(localEntryPath); statErr != nil {
			if !os.IsNotExist(statErr) {
				return statErr
			}
			if linkErr := os.Symlink(sourceEntryPath, localEntryPath); linkErr != nil {
				return linkErr
			}
			continue
		}
	}

	localEntries, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, entry := range localEntries {
		localEntryPath := filepath.Join(localPath, entry.Name())
		info, statErr := os.Lstat(localEntryPath)
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		resolvedTarget, resolveErr := resolveLink(localEntryPath)
		if resolveErr != nil {
			return resolveErr
		}
		if !isPathWithin(resolvedTarget, sourcePath) {
			continue
		}

		expectedSourceEntry := filepath.Join(sourcePath, entry.Name())
		if filepath.Clean(resolvedTarget) != filepath.Clean(expectedSourceEntry) {
			if removeErr := os.Remove(localEntryPath); removeErr != nil {
				return removeErr
			}
			if linkErr := os.Symlink(expectedSourceEntry, localEntryPath); linkErr != nil {
				return linkErr
			}
			continue
		}
		if _, sourceErr := os.Lstat(expectedSourceEntry); sourceErr != nil {
			if os.IsNotExist(sourceErr) {
				if removeErr := os.Remove(localEntryPath); removeErr != nil {
					return removeErr
				}
				continue
			}
			return sourceErr
		}
	}

	return nil
}

func ensureTarget(path string, isDir bool) error {
	if isDir {
		if info, err := os.Lstat(path); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if removeErr := os.Remove(path); removeErr != nil {
					return removeErr
				}
				return os.MkdirAll(path, 0o755)
			}
			if info.IsDir() {
				return nil
			}
			if removeErr := os.RemoveAll(path); removeErr != nil {
				return removeErr
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		return os.MkdirAll(path, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			if removeErr := os.RemoveAll(path); removeErr != nil {
				return removeErr
			}
			file, createErr := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
			if createErr != nil {
				return createErr
			}
			return file.Close()
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func ensureNonSymlinkDirectory(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if removeErr := os.RemoveAll(path); removeErr != nil {
				return removeErr
			}
			return os.MkdirAll(path, 0o755)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

func resolveLink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target), nil
	}
	return filepath.Clean(filepath.Join(filepath.Dir(path), target)), nil
}

func isPathWithin(path, root string) bool {
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(root)
	if cleanPath == cleanRoot {
		return true
	}
	prefix := cleanRoot + string(filepath.Separator)
	return strings.HasPrefix(cleanPath, prefix)
}
