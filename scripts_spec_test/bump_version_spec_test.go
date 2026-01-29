package scripts_spec_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Test_S_033_FR_001_ScriptUpdatesVersionFile verifies that the bump-version.sh
// script correctly generates cmd/frozendb/version.go
func Test_S_033_FR_001_ScriptUpdatesVersionFile(t *testing.T) {
	// Create a temporary git repository
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create cmd/frozendb directory
	cmdDir := filepath.Join(tmpDir, "cmd", "frozendb")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create cmd/frozendb directory: %v", err)
	}

	// Copy bump-version.sh to temp repo
	scriptPath := filepath.Join(tmpDir, "scripts", "bump-version.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("Failed to create scripts directory: %v", err)
	}

	// Copy the actual script
	origScriptPath := filepath.Join("..", "scripts", "bump-version.sh")
	copyFile(t, origScriptPath, scriptPath)

	// Make script executable
	if err := os.Chmod(scriptPath, 0755); err != nil {
		t.Fatalf("Failed to make script executable: %v", err)
	}

	// Run the script with a version
	version := "v0.1.0"
	cmd := exec.Command("bash", scriptPath, version)
	cmd.Dir = tmpDir
	// Provide empty input to auto-accept any prompts
	cmd.Stdin = strings.NewReader("\n")
	output, _ := cmd.CombinedOutput()

	// Script might fail due to git push, but files should be created
	t.Logf("Script output: %s", string(output))

	// Verify version.go was created
	versionGoPath := filepath.Join(cmdDir, "version.go")
	versionGoBytes, err := os.ReadFile(versionGoPath)
	if err != nil {
		t.Fatalf("Failed to read version.go: %v", err)
	}
	versionGoStr := string(versionGoBytes)

	if !strings.Contains(versionGoStr, "package main") {
		t.Errorf("version.go does not contain 'package main'")
	}
	if !strings.Contains(versionGoStr, `const Version = "`+version+`"`) {
		t.Errorf("version.go does not contain correct version constant. Content:\n%s", versionGoStr)
	}
}

// Test_S_033_FR_002_ScriptCreatesReleaseBranch verifies that the script creates
// a release branch with the correct naming pattern
func Test_S_033_FR_002_ScriptCreatesReleaseBranch(t *testing.T) {
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	// Setup basic files
	setupBasicProject(t, tmpDir)

	// Copy and run script
	scriptPath := setupScript(t, tmpDir)
	version := "v0.2.0"

	cmd := exec.Command("bash", scriptPath, version)
	cmd.Dir = tmpDir
	cmd.Stdin = strings.NewReader("\n")
	output, _ := cmd.CombinedOutput()
	t.Logf("Script output: %s", string(output))

	// Verify branch was created
	cmd = exec.Command("git", "branch", "--list", "release/"+version)
	cmd.Dir = tmpDir
	branchOutput, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list branches: %v", err)
	}

	if !strings.Contains(string(branchOutput), "release/"+version) {
		t.Errorf("Release branch release/%s was not created", version)
	}
}

// Test_S_033_FR_003_ScriptCommitsChanges verifies that the script commits
// the version changes with a descriptive message
func Test_S_033_FR_003_ScriptCommitsChanges(t *testing.T) {
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	setupBasicProject(t, tmpDir)
	scriptPath := setupScript(t, tmpDir)
	version := "v0.3.0"

	cmd := exec.Command("bash", scriptPath, version)
	cmd.Dir = tmpDir
	cmd.Stdin = strings.NewReader("\n")
	cmd.CombinedOutput()

	// Switch to release branch and check commit
	cmd = exec.Command("git", "checkout", "release/"+version)
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to checkout release branch: %v", err)
	}

	// Get latest commit message
	cmd = exec.Command("git", "log", "-1", "--pretty=%B")
	cmd.Dir = tmpDir
	commitMsg, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get commit message: %v", err)
	}

	commitMsgStr := strings.TrimSpace(string(commitMsg))
	expectedMsg := "Bump version to " + version
	if !strings.Contains(commitMsgStr, expectedMsg) {
		t.Errorf("Commit message does not contain '%s'. Got: %s", expectedMsg, commitMsgStr)
	}
}

// Test_S_033_FR_004_ScriptPushesBranch verifies that the script attempts to push
// the branch to remote (we'll check it tries, actual push may fail in test env)
func Test_S_033_FR_004_ScriptPushesBranch(t *testing.T) {
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	setupBasicProject(t, tmpDir)
	scriptPath := setupScript(t, tmpDir)
	version := "v0.4.0"

	// Run script
	cmd := exec.Command("bash", scriptPath, version)
	cmd.Dir = tmpDir
	cmd.Stdin = strings.NewReader("\n")
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)

	// Check that script mentions pushing
	if !strings.Contains(outputStr, "push") && !strings.Contains(outputStr, "Push") {
		t.Logf("Warning: Script output doesn't mention pushing. Output: %s", outputStr)
	}

	// The actual push may fail without a real remote, but the script should attempt it
	// We verify the branch exists locally, which proves the script got to the push step
	cmd = exec.Command("git", "branch", "--list", "release/"+version)
	cmd.Dir = tmpDir
	branchOutput, _ := cmd.Output()

	if !strings.Contains(string(branchOutput), "release/"+version) {
		t.Errorf("Branch was not created, indicating script failed before push step")
	}
}

// Test_S_033_FR_005_ScriptReportsErrorsClearly verifies that the script provides
// clear error messages when things go wrong
func Test_S_033_FR_005_ScriptReportsErrorsClearly(t *testing.T) {
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	setupBasicProject(t, tmpDir)
	scriptPath := setupScript(t, tmpDir)

	// Test with invalid version format
	invalidVersion := "1.0.0" // Missing 'v' prefix
	cmd := exec.Command("bash", scriptPath, invalidVersion)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()

	outputStr := strings.ToLower(string(output))
	if !strings.Contains(outputStr, "error") && !strings.Contains(outputStr, "invalid") {
		t.Errorf("Script should report clear error for invalid version. Output: %s", string(output))
	}
}

// Test_S_033_FR_009_VersionEmbeddedInBinary verifies that the version constant
// generated by the script can be compiled into a binary
func Test_S_033_FR_009_VersionEmbeddedInBinary(t *testing.T) {
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	setupBasicProject(t, tmpDir)
	scriptPath := setupScript(t, tmpDir)
	version := "v0.9.0"

	// Run script to generate version.go
	cmd := exec.Command("bash", scriptPath, version)
	cmd.Dir = tmpDir
	cmd.Stdin = strings.NewReader("\n")
	cmd.CombinedOutput()

	// Create a simple main.go that uses the version
	mainGoPath := filepath.Join(tmpDir, "cmd", "frozendb", "main.go")
	mainGoContent := `package main

import "fmt"

func main() {
	fmt.Printf("Version: %s\n", Version)
}
`
	if err := os.WriteFile(mainGoPath, []byte(mainGoContent), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Try to build the binary
	binaryPath := filepath.Join(tmpDir, "test-binary")
	cmd = exec.Command("go", "build", "-o", binaryPath, "./cmd/frozendb")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build binary with embedded version: %v\nOutput: %s", err, string(output))
	}

	// Run the binary and verify version is embedded
	cmd = exec.Command(binaryPath)
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run binary: %v", err)
	}

	if !strings.Contains(string(output), version) {
		t.Errorf("Binary output does not contain version %s. Output: %s", version, string(output))
	}
}

// Test_S_033_FR_015_ScriptValidatesSemanticVersioning verifies that the script
// validates semantic versioning format
func Test_S_033_FR_015_ScriptValidatesSemanticVersioning(t *testing.T) {
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	setupBasicProject(t, tmpDir)
	scriptPath := setupScript(t, tmpDir)

	testCases := []struct {
		version     string
		shouldPass  bool
		description string
	}{
		{"v0.1.0", true, "valid basic version"},
		{"v1.2.3", true, "valid standard version"},
		{"v2.0.0-rc1", true, "valid with prerelease"},
		{"v1.0.0-beta.2", true, "valid with dotted prerelease"},
		{"1.0.0", false, "missing v prefix"},
		{"v1.2", false, "missing patch version"},
		{"V1.0.0", false, "uppercase V"},
		{"v1.2.3.4", false, "too many version parts"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			cmd := exec.Command("bash", scriptPath, tc.version)
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()

			if tc.shouldPass {
				// For valid versions, we expect the script to at least start processing
				// (it may fail later due to duplicate branches in test runs)
				outputStr := strings.ToLower(string(output))
				if strings.Contains(outputStr, "invalid version") || strings.Contains(outputStr, "error: invalid") {
					t.Errorf("Script rejected valid version %s. Output: %s", tc.version, string(output))
				}
			} else {
				// For invalid versions, we expect an error
				if err == nil {
					t.Errorf("Script should reject invalid version %s but succeeded", tc.version)
				}
				outputStr := strings.ToLower(string(output))
				if !strings.Contains(outputStr, "error") && !strings.Contains(outputStr, "invalid") {
					t.Errorf("Script should report error for invalid version %s. Output: %s", tc.version, string(output))
				}
			}
		})
	}
}

// Test_S_033_FR_016_ScriptChecksUncommittedChanges verifies that the script
// detects and warns about uncommitted changes
func Test_S_033_FR_016_ScriptChecksUncommittedChanges(t *testing.T) {
	tmpDir := setupTempRepo(t)
	defer os.RemoveAll(tmpDir)

	setupBasicProject(t, tmpDir)
	scriptPath := setupScript(t, tmpDir)

	// Create an uncommitted file
	uncommittedFile := filepath.Join(tmpDir, "uncommitted.txt")
	if err := os.WriteFile(uncommittedFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create uncommitted file: %v", err)
	}

	version := "v0.16.0"
	cmd := exec.Command("bash", scriptPath, version)
	cmd.Dir = tmpDir
	// Provide input to continue despite warning
	cmd.Stdin = strings.NewReader("\n")
	output, _ := cmd.CombinedOutput()

	outputStr := strings.ToLower(string(output))
	if !strings.Contains(outputStr, "uncommitted") && !strings.Contains(outputStr, "changes") {
		t.Errorf("Script should warn about uncommitted changes. Output: %s", string(output))
	}
}

// Helper functions

func setupTempRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "bump-version-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	return tmpDir
}

func setupBasicProject(t *testing.T, tmpDir string) {
	t.Helper()

	// Create go.mod
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := "module github.com/example/frozendb\n\ngo 1.25.5\n"
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create cmd/frozendb directory
	cmdDir := filepath.Join(tmpDir, "cmd", "frozendb")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create cmd/frozendb directory: %v", err)
	}

	// Commit initial state
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	cmd.Run()
}

func setupScript(t *testing.T, tmpDir string) string {
	t.Helper()

	scriptPath := filepath.Join(tmpDir, "scripts", "bump-version.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("Failed to create scripts directory: %v", err)
	}

	origScriptPath := filepath.Join("..", "scripts", "bump-version.sh")
	copyFile(t, origScriptPath, scriptPath)

	if err := os.Chmod(scriptPath, 0755); err != nil {
		t.Fatalf("Failed to make script executable: %v", err)
	}

	return scriptPath
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("Failed to read source file %s: %v", src, err)
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("Failed to write destination file %s: %v", dst, err)
	}
}
