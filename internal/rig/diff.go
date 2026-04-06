package rig

import (
	"fmt"
	"os"
	"path/filepath"
)

type EntryDiff struct {
	Category   string
	Mode       PolicyMode
	LocalPath  string
	SourcePath string
	Desired    string
	Actual     string
	Match      bool
}

func DiffPolicyState(store *Store, cfg RigConfig) ([]EntryDiff, error) {
	rigCodexHome := store.RigCodexHome(cfg.Name)
	diffs := make([]EntryDiff, 0, len(ManagedCategories))
	for _, category := range ManagedCategories {
		mode := cfg.Policy[category]
		for _, target := range CategoryTargets[category] {
			localPath := filepath.Join(rigCodexHome, target.RelativePath)
			sourcePath, sourceErr := store.resolveSharedSourcePath(cfg, category, target)
			if sourceErr != nil {
				return nil, sourceErr
			}

			entry := EntryDiff{
				Category:   category,
				Mode:       mode,
				LocalPath:  localPath,
				SourcePath: sourcePath,
			}

			switch mode {
			case PolicyShared:
				entry.Desired = "symlink:" + filepath.Clean(sourcePath)
				actual, err := actualState(localPath)
				if err != nil {
					return nil, err
				}
				entry.Actual = actual
				entry.Match = entry.Desired == entry.Actual
			case PolicyIsolated:
				if target.IsDir {
					entry.Desired = "dir"
				} else {
					entry.Desired = "file"
				}
				actual, err := actualState(localPath)
				if err != nil {
					return nil, err
				}
				entry.Actual = actual
				entry.Match = entry.Desired == entry.Actual
			case PolicyInherited:
				entry.Desired = "inherited:" + filepath.Clean(sourcePath)
				actual, match, err := actualInheritedState(localPath, sourcePath)
				if err != nil {
					return nil, err
				}
				entry.Actual = actual
				entry.Match = match
			default:
				return nil, fmt.Errorf("unsupported mode %q", mode)
			}

			diffs = append(diffs, entry)
		}
	}
	return diffs, nil
}

func actualState(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing", nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := resolveLink(path)
		if readErr != nil {
			return "", readErr
		}
		return "symlink:" + filepath.Clean(target), nil
	}
	if info.IsDir() {
		return "dir", nil
	}
	if info.Mode().IsRegular() {
		return "file", nil
	}
	return "other", nil
}

func actualInheritedState(localPath, sourcePath string) (state string, match bool, err error) {
	sourceInfo, sourceErr := os.Stat(sourcePath)
	if sourceErr != nil {
		if os.IsNotExist(sourceErr) {
			return "source:missing", false, nil
		}
		return "", false, sourceErr
	}
	if !sourceInfo.IsDir() {
		return "source:not-dir", false, nil
	}

	localInfo, localErr := os.Lstat(localPath)
	if localErr != nil {
		if os.IsNotExist(localErr) {
			return "local:missing", false, nil
		}
		return "", false, localErr
	}
	if localInfo.Mode()&os.ModeSymlink != 0 {
		return "local:symlink", false, nil
	}
	if !localInfo.IsDir() {
		return "local:not-dir", false, nil
	}

	sourceEntries, err := os.ReadDir(sourcePath)
	if err != nil {
		return "", false, err
	}
	for _, sourceEntry := range sourceEntries {
		localEntryPath := filepath.Join(localPath, sourceEntry.Name())
		localEntryInfo, localEntryErr := os.Lstat(localEntryPath)
		if localEntryErr != nil {
			if os.IsNotExist(localEntryErr) {
				return "missing:" + sourceEntry.Name(), false, nil
			}
			return "", false, localEntryErr
		}
		if localEntryInfo.Mode()&os.ModeSymlink == 0 {
			continue
		}

		resolvedTarget, resolveErr := resolveLink(localEntryPath)
		if resolveErr != nil {
			return "", false, resolveErr
		}
		expectedTarget := filepath.Join(sourcePath, sourceEntry.Name())
		if filepath.Clean(resolvedTarget) != filepath.Clean(expectedTarget) && isPathWithin(resolvedTarget, sourcePath) {
			return "mislinked:" + sourceEntry.Name(), false, nil
		}
	}

	localEntries, err := os.ReadDir(localPath)
	if err != nil {
		return "", false, err
	}
	for _, localEntry := range localEntries {
		localEntryPath := filepath.Join(localPath, localEntry.Name())
		info, statErr := os.Lstat(localEntryPath)
		if statErr != nil {
			return "", false, statErr
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		resolvedTarget, resolveErr := resolveLink(localEntryPath)
		if resolveErr != nil {
			return "", false, resolveErr
		}
		if !isPathWithin(resolvedTarget, sourcePath) {
			continue
		}

		expectedTarget := filepath.Join(sourcePath, localEntry.Name())
		if filepath.Clean(resolvedTarget) != filepath.Clean(expectedTarget) {
			return "mislinked:" + localEntry.Name(), false, nil
		}
		if _, sourceErr := os.Lstat(expectedTarget); sourceErr != nil {
			if os.IsNotExist(sourceErr) {
				return "stale:" + localEntry.Name(), false, nil
			}
			return "", false, sourceErr
		}
	}

	return "inherited:ok", true, nil
}
