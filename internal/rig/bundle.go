package rig

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const BundleVersion = 1

type BundleManifest struct {
	Version    int       `json:"version"`
	RigName    string    `json:"rig_name"`
	ExportedAt time.Time `json:"exported_at"`
	Tool       string    `json:"tool"`
	ToolVer    string    `json:"tool_version,omitempty"`
}

func ExportRig(store *Store, cfg RigConfig, outputPath, toolVersion string) (BundleManifest, error) {
	rigDir := store.RigDir(cfg.Name)
	if _, err := os.Stat(rigDir); err != nil {
		return BundleManifest{}, err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return BundleManifest{}, err
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return BundleManifest{}, err
	}
	defer outFile.Close()

	gz := gzip.NewWriter(outFile)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	manifest := BundleManifest{
		Version:    BundleVersion,
		RigName:    cfg.Name,
		ExportedAt: time.Now().UTC(),
		Tool:       "codex-rig",
		ToolVer:    strings.TrimSpace(toolVersion),
	}
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BundleManifest{}, err
	}
	manifestRaw = append(manifestRaw, '\n')
	if err := writeTarFile(tw, "manifest.json", manifestRaw, 0o644); err != nil {
		return BundleManifest{}, err
	}

	if err := filepath.WalkDir(rigDir, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == rigDir {
			return nil
		}

		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(rigDir, current)
		if err != nil {
			return err
		}
		archivePath := path.Join("rig", filepath.ToSlash(rel))

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = archivePath
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(current)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, f)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}); err != nil {
		return BundleManifest{}, err
	}

	return manifest, nil
}

func ImportRig(store *Store, bundlePath, targetRigName string, overwrite bool) (RigConfig, error) {
	if strings.TrimSpace(bundlePath) == "" {
		return RigConfig{}, errors.New("bundle path is required")
	}
	inFile, err := os.Open(bundlePath)
	if err != nil {
		return RigConfig{}, err
	}
	defer inFile.Close()

	gz, err := gzip.NewReader(inFile)
	if err != nil {
		return RigConfig{}, err
	}
	defer gz.Close()

	tmpDir, err := os.MkdirTemp("", "codex-rig-import-*")
	if err != nil {
		return RigConfig{}, err
	}
	defer os.RemoveAll(tmpDir)

	if err := extractBundleArchive(tar.NewReader(gz), tmpDir); err != nil {
		return RigConfig{}, err
	}

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if raw, err := os.ReadFile(manifestPath); err == nil {
		var manifest BundleManifest
		if jsonErr := json.Unmarshal(raw, &manifest); jsonErr != nil {
			return RigConfig{}, fmt.Errorf("invalid bundle manifest: %w", jsonErr)
		}
		if manifest.Version != BundleVersion {
			return RigConfig{}, fmt.Errorf("unsupported bundle version %d", manifest.Version)
		}
	} else if !os.IsNotExist(err) {
		return RigConfig{}, err
	}

	importedRigDir := filepath.Join(tmpDir, "rig")
	rawCfg, err := os.ReadFile(filepath.Join(importedRigDir, ConfigFileName))
	if err != nil {
		return RigConfig{}, err
	}
	cfg, err := ParseRigConfig(rawCfg)
	if err != nil {
		return RigConfig{}, err
	}

	desiredName := strings.TrimSpace(targetRigName)
	if desiredName == "" {
		desiredName = cfg.Name
	}
	if err := ValidateRigName(desiredName); err != nil {
		return RigConfig{}, err
	}
	cfg.Name = desiredName

	if err := store.EnsureRoot(); err != nil {
		return RigConfig{}, err
	}
	targetDir := store.RigDir(cfg.Name)
	if _, err := os.Stat(targetDir); err == nil {
		if !overwrite {
			return RigConfig{}, fmt.Errorf("rig %q already exists (use --overwrite to replace)", cfg.Name)
		}
		if removeErr := os.RemoveAll(targetDir); removeErr != nil {
			return RigConfig{}, removeErr
		}
	} else if !os.IsNotExist(err) {
		return RigConfig{}, err
	}

	if err := moveDir(importedRigDir, targetDir); err != nil {
		return RigConfig{}, err
	}

	if err := store.SaveRigConfig(cfg); err != nil {
		return RigConfig{}, err
	}
	return cfg, nil
}

func writeTarFile(tw *tar.Writer, name string, content []byte, mode fs.FileMode) error {
	header := &tar.Header{
		Name:    name,
		Mode:    int64(mode.Perm()),
		Size:    int64(len(content)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

func extractBundleArchive(tr *tar.Reader, outDir string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		relPath, err := sanitizeArchivePath(header.Name)
		if err != nil {
			return err
		}
		if relPath == "" {
			continue
		}

		if relPath != "manifest.json" && relPath != "rig" && !strings.HasPrefix(relPath, "rig"+string(filepath.Separator)) {
			continue
		}

		targetPath := filepath.Join(outDir, relPath)
		mode := fs.FileMode(header.Mode)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, mode.Perm()); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, tr)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			continue
		}
	}
}

func sanitizeArchivePath(name string) (string, error) {
	clean := path.Clean(strings.TrimSpace(name))
	if clean == "." || clean == "" {
		return "", nil
	}
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid archive path %q", name)
	}
	return filepath.FromSlash(clean), nil
}

func moveDir(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	} else {
		var linkErr *os.LinkError
		if !errors.As(err, &linkErr) || linkErr.Err != syscall.EXDEV {
			return err
		}
	}

	if err := copyDir(source, target); err != nil {
		return err
	}
	return os.RemoveAll(source)
}

func copyDir(source, target string) error {
	return filepath.WalkDir(source, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		dstPath := filepath.Join(target, rel)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		in, err := os.Open(current)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeOutErr := out.Close()
		closeInErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		if closeInErr != nil {
			return closeInErr
		}
		return nil
	})
}
