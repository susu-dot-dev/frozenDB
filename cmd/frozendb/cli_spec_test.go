package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Test_S_028_FR_002_CLIEntryPoint verifies that /cmd/frozendb/main.go exists with a main() function
//
// Functional Requirement FR-002: CLI entry point exists at /cmd/frozendb/main.go
// Success Criteria SC-001: Developers can build CLI in <1 minute with `make build-cli`
// Success Criteria SC-002: CLI outputs "Hello world" exactly when executed
func Test_S_028_FR_002_CLIEntryPoint(t *testing.T) {
	// Get repository root (two levels up from cmd/frozendb)
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get repository root: %v", err)
	}

	mainGoPath := filepath.Join(repoRoot, "cmd", "frozendb", "main.go")

	// Verify main.go exists
	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		t.Fatalf("main.go does not exist at expected path: %s", mainGoPath)
	}

	// Parse main.go to verify it contains a main() function
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, mainGoPath, nil, 0)
	if err != nil {
		t.Fatalf("Failed to parse main.go: %v", err)
	}

	// Verify package declaration is "main"
	if node.Name.Name != "main" {
		t.Errorf("Package name is %q, expected %q", node.Name.Name, "main")
	}

	// Find main() function
	hasMainFunc := false
	for _, decl := range node.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Name.Name == "main" {
				hasMainFunc = true

				// Verify main() has no parameters
				if funcDecl.Type.Params != nil && funcDecl.Type.Params.NumFields() > 0 {
					t.Errorf("main() function has parameters, expected no parameters")
				}

				// Verify main() has no return values
				if funcDecl.Type.Results != nil && funcDecl.Type.Results.NumFields() > 0 {
					t.Errorf("main() function has return values, expected no return values")
				}

				break
			}
		}
	}

	if !hasMainFunc {
		t.Errorf("main.go does not contain a main() function")
	}

	t.Logf("✓ CLI entry point exists at %s with valid main() function", mainGoPath)
}

// Test_S_028_FR_002_HelloWorldOutput verifies that the CLI binary outputs "Hello world" exactly
//
// Functional Requirement FR-002: CLI outputs "Hello world\n" when executed
// Success Criteria SC-002: CLI outputs "Hello world" exactly when executed
func Test_S_028_FR_002_HelloWorldOutput(t *testing.T) {
	// Get repository root
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get repository root: %v", err)
	}

	// Build the CLI binary in a temporary location
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "frozendb-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/frozendb")
	buildCmd.Dir = repoRoot
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, buildOutput)
	}

	// Execute the CLI binary
	execCmd := exec.Command(binaryPath)
	output, err := execCmd.Output()
	if err != nil {
		t.Fatalf("Failed to execute CLI: %v", err)
	}

	// Verify output is exactly "Hello world\n"
	expectedOutput := "Hello world\n"
	actualOutput := string(output)

	if actualOutput != expectedOutput {
		t.Errorf("CLI output mismatch:\nExpected: %q\nActual:   %q", expectedOutput, actualOutput)
	}

	t.Logf("✓ CLI outputs exactly %q", expectedOutput)
}

// Test_S_028_FR_003_CLIBuildable verifies that the CLI can be built and creates a valid binary
//
// Functional Requirement FR-003: CLI is buildable with `go build -o frozendb ./cmd/frozendb`
// Success Criteria SC-001: Developers can build CLI in <1 minute with `make build-cli`
func Test_S_028_FR_003_CLIBuildable(t *testing.T) {
	// Get repository root
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get repository root: %v", err)
	}

	// Build the CLI binary in a temporary location
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "frozendb")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/frozendb")
	buildCmd.Dir = repoRoot
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build CLI with command 'go build -o frozendb ./cmd/frozendb': %v\nOutput: %s", err, buildOutput)
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary was not created at expected path: %s", binaryPath)
	}

	// Verify binary is executable
	fileInfo, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("Failed to stat binary: %v", err)
	}

	if fileInfo.Mode()&0111 == 0 {
		t.Errorf("Binary is not executable (mode: %s)", fileInfo.Mode())
	}

	// Verify binary can be executed
	execCmd := exec.Command(binaryPath)
	output, err := execCmd.Output()
	if err != nil {
		t.Fatalf("Binary failed to execute: %v", err)
	}

	// Basic sanity check - output should contain "Hello world"
	if !strings.Contains(string(output), "Hello world") {
		t.Errorf("Binary output does not contain 'Hello world': %q", string(output))
	}

	t.Logf("✓ CLI is buildable and creates executable binary at %s", binaryPath)
}
