package frozendb_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/susu-dot-dev/frozenDB/pkg/frozendb"
)

// Test_S_028_FR_005_MinimalPublicAPI verifies that the public API includes ONLY
// the core types and functions needed for database operations.
// Spec 028: Project Structure Refactor & CLI
// FR-005: Public API MUST include ONLY: FrozenDB, Transaction (without GetEmptyRow/GetRows),
// MODE constants, FinderStrategy, error types
func Test_S_028_FR_005_MinimalPublicAPI(t *testing.T) {
	t.Run("FrozenDB_type_exists", func(t *testing.T) {
		// Verify we can reference the FrozenDB type
		var _ *frozendb.FrozenDB
	})

	t.Run("Transaction_type_exists", func(t *testing.T) {
		// Verify we can reference the Transaction type
		var _ *frozendb.Transaction
	})

	t.Run("MODE_constants_exist", func(t *testing.T) {
		// Verify MODE constants are exported
		if frozendb.MODE_READ == "" {
			t.Error("MODE_READ should be defined and non-empty")
		}
		if frozendb.MODE_WRITE == "" {
			t.Error("MODE_WRITE should be defined and non-empty")
		}
	})

	t.Run("FinderStrategy_type_exists", func(t *testing.T) {
		// Verify FinderStrategy type and constants exist
		var _ = frozendb.FinderStrategySimple
		var _ = frozendb.FinderStrategyInMemory
		var _ = frozendb.FinderStrategyBinarySearch
	})

	t.Run("NewFrozenDB_function_exists", func(t *testing.T) {
		// Verify NewFrozenDB function is exported (will fail at runtime if called with invalid path)
		// We're just checking the function exists and has the right signature
		_ = frozendb.NewFrozenDB
	})

	t.Run("error_types_exist", func(t *testing.T) {
		// Verify all error types are exported
		var _ *frozendb.FrozenDBError
		var _ *frozendb.InvalidInputError
		var _ *frozendb.InvalidActionError
		var _ *frozendb.PathError
		var _ *frozendb.WriteError
		var _ *frozendb.CorruptDatabaseError
		var _ *frozendb.KeyOrderingError
		var _ *frozendb.TombstonedError
		var _ *frozendb.ReadError
		var _ *frozendb.KeyNotFoundError
		var _ *frozendb.TransactionActiveError
		var _ *frozendb.InvalidDataError
	})

	t.Run("error_constructors_exist", func(t *testing.T) {
		// Verify error constructor functions are exported
		_ = frozendb.NewInvalidInputError
		_ = frozendb.NewInvalidActionError
		_ = frozendb.NewPathError
		_ = frozendb.NewWriteError
		_ = frozendb.NewCorruptDatabaseError
		_ = frozendb.NewKeyOrderingError
		_ = frozendb.NewTombstonedError
		_ = frozendb.NewReadError
		_ = frozendb.NewKeyNotFoundError
		_ = frozendb.NewTransactionActiveError
		_ = frozendb.NewInvalidDataError
	})
}

// Test_S_028_FR_005_ExcludeCreationAPI verifies that database creation functions
// are NOT exposed in the public API.
// Spec 028: Project Structure Refactor & CLI
// FR-005: CreateFrozenDB, CreateConfig, SudoContext MUST NOT be in /pkg
func Test_S_028_FR_005_ExcludeCreationAPI(t *testing.T) {
	// Parse the pkg/frozendb package to check what's exported
	pkgPath := "."
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, pkgPath, nil, 0)
	if err != nil {
		t.Fatalf("Failed to parse package: %v", err)
	}

	// Collect all exported identifiers
	exportedNames := make(map[string]bool)
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if s.Name.IsExported() {
								exportedNames[s.Name.Name] = true
							}
						case *ast.ValueSpec:
							for _, name := range s.Names {
								if name.IsExported() {
									exportedNames[name.Name] = true
								}
							}
						}
					}
				case *ast.FuncDecl:
					if d.Name.IsExported() {
						exportedNames[d.Name.Name] = true
					}
				}
			}
		}
	}

	// Verify creation API is NOT exported
	forbiddenNames := []string{
		"CreateFrozenDB",
		"CreateConfig",
		"NewCreateConfig",
		"SudoContext",
	}

	for _, name := range forbiddenNames {
		if exportedNames[name] {
			t.Errorf("Creation API %s should NOT be exported from pkg/frozendb", name)
		}
	}
}

// Test_S_028_FR_005_ExcludeInternalTypes verifies that internal row types
// are NOT exposed in the public API.
// Spec 028: Project Structure Refactor & CLI
// FR-005: NullRow, DataRow, PartialDataRow, Header MUST NOT be in /pkg
func Test_S_028_FR_005_ExcludeInternalTypes(t *testing.T) {
	// Parse the pkg/frozendb package to check what's exported
	pkgPath := "."
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, pkgPath, nil, 0)
	if err != nil {
		t.Fatalf("Failed to parse package: %v", err)
	}

	// Collect all exported type names
	exportedTypes := make(map[string]bool)
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							if typeSpec.Name.IsExported() {
								exportedTypes[typeSpec.Name.Name] = true
							}
						}
					}
				}
			}
		}
	}

	// Verify internal types are NOT exported
	forbiddenTypes := []string{
		"NullRow",
		"DataRow",
		"PartialDataRow",
		"Header",
		"Row",
		"RowUnion",
	}

	for _, typeName := range forbiddenTypes {
		if exportedTypes[typeName] {
			t.Errorf("Internal type %s should NOT be exported from pkg/frozendb", typeName)
		}
	}
}

// Test_S_028_FR_005_TransactionMethodsExcluded verifies that Transaction
// does not expose GetEmptyRow or GetRows methods.
// This test ensures the public API doesn't expose internal types through Transaction methods.
func Test_S_028_FR_005_TransactionMethodsExcluded(t *testing.T) {
	// Parse all .go files in pkg/frozendb
	pkgPath := "."
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, pkgPath, nil, 0)
	if err != nil {
		t.Fatalf("Failed to parse package: %v", err)
	}

	// Look for any exported methods on Transaction type
	forbiddenMethods := []string{"GetEmptyRow", "GetRows"}

	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			for _, decl := range file.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok {
					// Check if this is a method
					if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
						// Get the receiver type name
						recvType := ""
						switch t := funcDecl.Recv.List[0].Type.(type) {
						case *ast.StarExpr:
							if ident, ok := t.X.(*ast.Ident); ok {
								recvType = ident.Name
							}
						case *ast.Ident:
							recvType = t.Name
						}

						// If it's a Transaction method, check if it's forbidden
						if recvType == "Transaction" && funcDecl.Name.IsExported() {
							for _, forbidden := range forbiddenMethods {
								if funcDecl.Name.Name == forbidden {
									t.Errorf("Transaction method %s should NOT be in public API (found in %s)",
										forbidden, filepath.Base(fileName))
								}
							}
						}
					}
				}
			}
		}
	}

	// Additional check: verify the file content doesn't contain method forwards for these
	// This catches cases where methods might be forwarded without being redeclared
	t.Run("check_source_files_for_forbidden_forwards", func(t *testing.T) {
		matches, err := filepath.Glob(filepath.Join(pkgPath, "*.go"))
		if err != nil {
			t.Fatalf("Failed to glob source files: %v", err)
		}

		for _, match := range matches {
			// Skip test files
			if strings.HasSuffix(match, "_test.go") {
				continue
			}

			content, err := parser.ParseFile(fset, match, nil, parser.ParseComments)
			if err != nil {
				continue
			}

			// Check for method declarations
			for _, decl := range content.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok {
					if funcDecl.Recv != nil && funcDecl.Name != nil {
						methodName := funcDecl.Name.Name
						if methodName == "GetEmptyRow" || methodName == "GetRows" {
							t.Errorf("Found forbidden method %s in %s", methodName, filepath.Base(match))
						}
					}
				}
			}
		}
	})
}
