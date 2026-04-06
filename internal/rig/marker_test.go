package rig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindMarkerUpward(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := WriteMarker(root, "alpha"); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	markerPath, marker, found, err := FindMarker(nested)
	if err != nil {
		t.Fatalf("FindMarker: %v", err)
	}
	if !found {
		t.Fatal("expected marker to be found")
	}
	expectedPath := filepath.Join(root, MarkerFileName)
	if markerPath != expectedPath {
		t.Fatalf("unexpected marker path: %s", markerPath)
	}
	if marker.Rig != "alpha" {
		t.Fatalf("unexpected rig in marker: %s", marker.Rig)
	}

	repoRoot, err := FindRepoRoot(nested)
	if err != nil {
		t.Fatalf("FindRepoRoot: %v", err)
	}
	if repoRoot != root {
		t.Fatalf("unexpected repo root: %s", repoRoot)
	}
}

func TestFindMarkerMissing(t *testing.T) {
	root := t.TempDir()
	markerPath, marker, found, err := FindMarker(root)
	if err != nil {
		t.Fatalf("FindMarker: %v", err)
	}
	if found {
		t.Fatalf("expected no marker, got path=%s marker=%+v", markerPath, marker)
	}
}
