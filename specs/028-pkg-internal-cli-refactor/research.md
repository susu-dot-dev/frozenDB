# Research: Project Structure Refactor & CLI

**Feature**: 028-pkg-internal-cli-refactor  
**Date**: 2026-01-28  
**Status**: Complete

## Overview

This document captures research findings for refactoring frozenDB into a clean architectural structure with `/pkg`, `/internal`, `/cmd`, and `/examples` directories while maintaining 100% backward compatibility.

## Decision: Go Project Layout Pattern

**Chosen**: Standard Go project layout with `/pkg`, `/internal`, `/cmd`, `/examples`

**Rationale**:
1. **Industry Standard**: Used by major Go projects (Kubernetes, Docker, Prometheus, Terraform)
2. **Go Compiler Enforced**: `/internal` package privacy is enforced at compile-time by Go toolchain
3. **Clear Intent**: Directory names communicate purpose without documentation
4. **Zero Learning Curve**: Go developers immediately understand the structure
5. **Module-Friendly**: Works seamlessly with Go modules and import paths

**Alternatives Considered**:
- **Flat structure with naming conventions**: Rejected because lacks compiler enforcement of boundaries
- **src/ directory**: Rejected as unnecessary - Go projects typically don't use src/ at root level
- **Single public file approach**: Rejected because too limiting for future API expansion

**References**:
- golang-standards/project-layout: Community-documented standard patterns
- Go Wiki "Go Project Layout": Official guidance on /internal and /cmd
- Major open-source projects: Kubernetes, Istio, CockroachDB all use this pattern

## Decision: Re-export Shim Layer Pattern

**Chosen**: Type aliases + function forwards in `/pkg/frozendb`

**Rationale**:
1. **Zero Overhead**: Type aliases compile to zero runtime cost in Go
2. **Complete IDE Support**: Godoc, autocomplete, and type checking work identically
3. **Explicit Control**: Can selectively exclude internal types (NullRow, GetEmptyRow, GetRows)
4. **Maintainable**: Clear mapping between public API and internal implementation
5. **Testable**: Examples can validate the public API surface

**Implementation Pattern**:
```go
// pkg/frozendb/frozendb.go
package frozendb

import "github.com/susu-dot-dev/frozenDB/internal/frozendb"

// Re-export types
type FrozenDB = frozendb.FrozenDB
type CreateConfig = frozendb.CreateConfig

// Re-export constants
const (
    MODE_READ  = frozendb.MODE_READ
    MODE_WRITE = frozendb.MODE_WRITE
)

// Re-export functions
func NewFrozenDB(path string, mode string, strategy FinderStrategy) (*FrozenDB, error) {
    return frozendb.NewFrozenDB(path, mode, strategy)
}
```

**Alternatives Considered**:
- **Wrapper structs with delegation**: Rejected due to runtime overhead and complex API surface duplication
- **Generated code**: Rejected as over-engineered for this use case
- **Interface-based abstraction**: Rejected because adds complexity without benefit

## Decision: Transaction API Cleanup

**Chosen**: Remove `GetEmptyRow()` and `GetRows()` from public Transaction API

**Rationale**:
1. **Implementation Detail Leakage**: NullRow is an internal file format structure
2. **No User Value**: External users don't need direct access to raw row data
3. **Future Flexibility**: Removing now prevents breaking changes later
4. **Constitution Alignment**: Reduces API surface area to essential operations

**Current Usage Analysis**:
- `GetEmptyRow()`: Returns internal `*NullRow` type - implementation detail
- `GetRows()`: Returns `[]DataRow` slice - exposes internal row structures
- Both methods expose internal types that should not be in public API

**Migration Path**: None needed - these methods have no documented external use

**Alternatives Considered**:
- **Deprecate first, remove later**: Rejected because this is the right time during refactor
- **Keep with documentation warnings**: Rejected because still commits to maintaining internal types publicly

## Decision: Minimal Public API Surface

**Chosen**: Expose ONLY core operations in `/pkg/frozendb` - exclude database creation from public API

**Rationale**:
1. **Future CLI Integration**: Database creation will be handled by CLI commands (e.g., `frozendb create`) in future releases
2. **Reduced Maintenance Burden**: Smaller public API means fewer compatibility commitments
3. **Hide Implementation Details**: CreateConfig, SudoContext, and file creation logic are implementation details
4. **Cleaner User Experience**: Library users typically work with existing databases; CLI handles creation
5. **Easier API Evolution**: Can modify creation logic without breaking public API contracts

**Public API Includes** (in /pkg/frozendb):
- `FrozenDB` type and `NewFrozenDB()` - Open existing databases
- `Transaction` type with `Begin()`, `AddRow()`, `Commit()`, `Rollback()` - Transaction operations
- `MODE_READ`, `MODE_WRITE` constants - Access modes
- `FinderStrategy` type and constants - Query optimization strategies
- Error types and constructors - Error handling
- Finder methods: `GetIndex()`, `GetTransactionStart()`, `GetTransactionEnd()` - Query operations
- `Close()` - Resource cleanup

**Excluded from Public API** (internal-only):
- `CreateFrozenDB()`, `CreateConfig`, `NewCreateConfig()` - CLI will handle creation
- `SudoContext` - Internal file creation detail
- `NullRow`, `DataRow`, `PartialDataRow` - Internal row structures
- `GetEmptyRow()`, `GetRows()` - Internal transaction inspection
- File format constants and implementation details
- `Header` type - Internal metadata structure

**Example Impact**:
- `examples/getting_started/` will temporarily use internal package for database creation
- Example demonstrates: `import "github.com/susu-dot-dev/frozenDB/internal/frozendb"` for CreateFrozenDB
- Once CLI supports creation, examples will use: `exec.Command("frozendb", "create", ...)`

**Future CLI Commands** (not in this spec):
```bash
frozendb create --path /data/db.fdb --row-size 256 --skew-ms 5000
frozendb open --path /data/db.fdb --mode read
```

**Alternatives Considered**:
- **Expose all creation APIs publicly**: Rejected because CLI should be primary interface for creation
- **Separate creation package**: Rejected as over-engineered; internal package is sufficient
- **Keep current full API**: Rejected because exposes too many implementation details

## Decision: CLI Implementation Approach

**Chosen**: Minimal "Hello world" CLI with cobra foundation for future commands

**Rationale**:
1. **Spec Requirement**: Spec explicitly states "Hello world" as MVP with "future commands" expectation
2. **Industry Standard**: cobra is the de facto CLI framework in Go ecosystem (used by kubectl, hugo, docker)
3. **Low Risk**: Single file implementation with minimal dependencies
4. **Extensible**: Root command structure makes adding database commands straightforward in future

**Implementation**:
```go
// cmd/frozendb/main.go
package main

import "fmt"

func main() {
    fmt.Println("Hello world")
}
```

**Build Integration**:
- Add `make build-cli` target: `go build -o frozendb ./cmd/frozendb`
- Add `frozendb` to `make ci` dependency chain
- Update `.gitignore` to exclude `frozendb` binary

**Alternatives Considered**:
- **flag package**: Rejected because spec indicates future command expansion needs subcommand support
- **Full cobra implementation now**: Rejected as beyond spec scope (only "Hello world" required)
- **Custom command framework**: Rejected due to unnecessary complexity

## Decision: Examples Strategy

**Chosen**: Single `examples/getting_started/main.go` demonstrating core workflows

**Rationale**:
1. **Validation**: Proves public API is complete and works correctly
2. **Documentation**: Serves as living example for new users
3. **Regression Test**: Build failures indicate API problems during refactors
4. **Focused Scope**: One comprehensive example better than multiple incomplete ones

**Example Content**:
- Create database using internal package: `internal/frozendb.CreateFrozenDB()` (temporary until CLI supports creation)
- Open database with public API: `pkg/frozendb.NewFrozenDB()`
- Start transaction with `BeginTransaction()`
- Add rows with `AddRow()`
- Commit transaction with `Commit()`
- Query with `GetIndex()`
- Close database

**Import Strategy**:
```go
// Temporary for database creation
import "github.com/susu-dot-dev/frozenDB/internal/frozendb"

// Future: Use CLI for creation
// $ frozendb create --path /tmp/db.fdb --row-size 256 --skew-ms 5000
```

**Build Verification**:
- Example must compile: `go build ./examples/getting_started`
- Example should be runnable (creates temp database)
- Include example in CI pipeline to catch API breaks

**Alternatives Considered**:
- **Multiple focused examples**: Rejected as over-scope for this feature
- **Test files instead of examples**: Rejected because examples serve documentation purpose
- **No examples**: Rejected because misses opportunity to validate public API
- **Only use public API**: Rejected because creation won't be in public API (CLI-only future)

## Decision: Test Migration Strategy

**Chosen**: Move tests with source, update imports, verify all pass

**Rationale**:
1. **Co-location**: Tests belong with implementation in `/internal/frozendb`
2. **Zero Behavior Change**: Tests validate that refactor preserves functionality
3. **Import Updates Only**: Test code doesn't change, only import paths
4. **Spec Test Protection**: Constitutional requirement - spec tests cannot be modified without permission

**Migration Steps**:
1. Move `*_test.go` and `*_spec_test.go` files to `/internal/frozendb`
2. Update imports from `package frozendb` to `package frozendb` (path changes, not package name)
3. Run `make test` and verify 100% pass rate
4. Run `make test-spec` to specifically verify spec tests

**Constitution Compliance**:
- Spec tests may NOT be modified (only import paths updated)
- All existing tests must pass
- If any test fails, indicates refactor broke behavior (must fix)

**Alternatives Considered**:
- **Keep tests at root**: Rejected because Go convention is to co-locate tests
- **Separate test package**: Rejected because tests need access to internal types
- **Rewrite tests**: Rejected - violates spec test constitutional protection

## Decision: Import Path Migration

**Chosen**: External users import `github.com/susu-dot-dev/frozenDB/pkg/frozendb`

**Rationale**:
1. **Semantic Clarity**: `/pkg` in import path signals "this is public API"
2. **Go Module Compatibility**: Standard practice in Go ecosystem
3. **Version Independence**: Path doesn't change across versions
4. **External vs Internal**: Compiler enforces `/internal` cannot be imported externally

**Migration for Existing Code**:
```go
// Old import (still works if external code exists)
import "github.com/susu-dot-dev/frozenDB/frozendb"

// New import (recommended after refactor)
import "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
```

**Backward Compatibility Note**: Since frozenDB is currently in active development (spec 028), we assume no external consumers yet. If external consumers exist, we could temporarily maintain both import paths.

**Alternatives Considered**:
- **Import path redirect**: Rejected as unnecessary complexity for pre-1.0 project
- **Versioned imports (v2)**: Rejected because this isn't a breaking change with shim layer
- **Keep old path**: Rejected because defeats purpose of refactor

## Decision: Makefile Updates

**Chosen**: Add CLI build targets, integrate into CI

**Rationale**:
1. **User Convenience**: `make build-cli` provides clear command
2. **CI Integration**: `make ci` builds and validates CLI
3. **Consistent Interface**: Follows existing Makefile patterns

**New Targets**:
```makefile
.PHONY: build-cli
build-cli: ## Build the frozendb CLI binary
	go build -o frozendb ./cmd/frozendb

.PHONY: clean-cli
clean-cli: ## Remove built CLI binary
	rm -f frozendb

# Update ci target to include CLI build
ci: deps tidy fmt lint test build build-cli
```

**Alternatives Considered**:
- **No Makefile changes**: Rejected because reduces discoverability
- **Separate CI workflow**: Rejected as overkill for single binary

## Decision: Documentation Updates

**Chosen**: Update README.md with new import paths and structure

**Rationale**:
1. **First Impression**: README is first thing users see
2. **Quick Start**: Must show correct import path for new users
3. **Migration Guide**: Helps any existing users transition

**Sections to Update**:
- Installation: Show `go get github.com/susu-dot-dev/frozenDB/pkg/frozendb`
- Quick Start: Update import statements
- Project Structure: Document new layout
- Building CLI: Add instructions for `make build-cli`

**Alternatives Considered**:
- **Separate MIGRATION.md**: Rejected as unnecessary for pre-1.0 project
- **No documentation updates**: Rejected because leaves users confused

## Implementation Complexity Analysis

**Estimated Complexity**: Low-Medium

**Breakdown**:
- **File Movement**: Simple but tedious (50+ files)
- **Import Updates**: Mechanical string replacement
- **Shim Layer**: Straightforward type aliases and forwards (~150 lines total)
- **CLI**: Trivial (10 lines)
- **Examples**: Simple (50-100 lines)
- **Tests**: Verification only, no changes beyond imports

**Risk Assessment**:
- **Low Risk**: No behavior changes, validated by existing tests
- **Medium Effort**: High file count but each change is simple
- **High Confidence**: Constitution check passes, comprehensive test coverage exists

**Validation Plan**:
1. All existing tests pass with zero modifications (except imports)
2. Example compiles and runs successfully
3. CLI builds and outputs "Hello world"
4. `make ci` completes successfully
5. No circular import errors

## Summary

The refactor follows established Go community patterns, maintains 100% backward compatibility through re-export shims, cleans up the public API by removing internal type exposure, and validates the new structure through a working example. The approach is low-risk, well-understood, and positions frozenDB for future growth with clear architectural boundaries.
