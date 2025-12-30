package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	err := Init([]string{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify .tin directory was created
	if _, err := os.Stat(filepath.Join(tmpDir, ".tin")); os.IsNotExist(err) {
		t.Error("expected .tin directory to be created")
	}
}

func TestInit_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp directory
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	// First init should succeed
	if err := Init([]string{}); err != nil {
		t.Fatalf("First Init failed: %v", err)
	}

	// Second init should fail
	err := Init([]string{})
	if err == nil {
		t.Error("expected error on second init")
	}
}

func TestInit_HelpFlag(t *testing.T) {
	// Test that --help doesn't error
	err := Init([]string{"--help"})
	if err != nil {
		t.Errorf("Init --help should not error: %v", err)
	}

	err = Init([]string{"-h"})
	if err != nil {
		t.Errorf("Init -h should not error: %v", err)
	}
}
