package frozendb

import (
	"os"
	"path/filepath"
	"testing"
)

// Test_S_028_FR_001_DirectoryStructureExists verifies that the required
// directory structure exists for the refactored project.
// Spec 028: Project Structure Refactor & CLI
// FR-001: Project MUST have /pkg, /internal, /examples, and /cmd directories at repository root
func Test_S_028_FR_001_DirectoryStructureExists(t *testing.T) {
	// Get the repository root (two levels up from internal/frozendb)
	repoRoot := filepath.Join("..", "..")

	// Required directories for the refactored project structure
	requiredDirs := []string{"pkg", "internal", "examples", "cmd"}

	for _, dir := range requiredDirs {
		dirPath := filepath.Join(repoRoot, dir)
		t.Run(dir+"_directory_exists", func(t *testing.T) {
			info, err := os.Stat(dirPath)
			if err != nil {
				t.Errorf("Directory %s should exist, got error: %v", dir, err)
				return
			}
			if !info.IsDir() {
				t.Errorf("%s should be a directory, but is not", dir)
			}
		})
	}
}

// Test_S_028_FR_004_InternalImplementation verifies that all implementation
// files are located in /internal/frozendb package.
// Spec 028: Project Structure Refactor & CLI
// FR-004: All implementation code MUST be in /internal/frozendb (at least 25 .go files including tests)
func Test_S_028_FR_004_InternalImplementation(t *testing.T) {
	// Get the current directory (internal/frozendb)
	internalFrozendbPath := "."

	// Read all files in the internal/frozendb directory
	entries, err := os.ReadDir(internalFrozendbPath)
	if err != nil {
		t.Fatalf("Should be able to read internal/frozendb directory: %v", err)
	}

	// Count .go files (including test files)
	goFileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			goFileCount++
		}
	}

	// Verify we have at least 25 .go files as specified in FR-004
	if goFileCount < 25 {
		t.Errorf("internal/frozendb should contain at least 25 .go files (including tests), found %d", goFileCount)
	}

	t.Logf("Found %d .go files in internal/frozendb", goFileCount)
}
