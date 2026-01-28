# Data Model: Project Structure Refactor & CLI

**Feature**: 028-pkg-internal-cli-refactor  
**Date**: 2026-01-28  
**Status**: Complete

## Overview

This document defines the data entities and state transitions introduced by the project structure refactor. Since this is a pure structural refactor with zero behavior changes, this document focuses on the new organizational entities (directories, shim files) rather than database data structures.

## New Organizational Entities

### 1. Public API Shim Files (`/pkg/frozendb/*.go`)

**Purpose**: Re-export MINIMAL public API surface from internal implementation - core operations only

**Files**:
- `frozendb.go`: FrozenDB type, NewFrozenDB, MODE constants, Close
- `transaction.go`: Transaction type (excluding GetEmptyRow, GetRows)
- `errors.go`: All error types and constructor functions
- `finder.go`: FinderStrategy type and constants, Finder methods (GetIndex, GetTransactionStart, GetTransactionEnd)

**NOT INCLUDED** (kept internal-only):
- ❌ `create.go`: CreateFrozenDB, CreateConfig, NewCreateConfig, SudoContext - CLI will handle creation
- ❌ `header.go`: Header type - internal metadata structure
- ❌ Row types: NullRow, DataRow, PartialDataRow - internal file format

**Validation Rules**:
- Each shim file MUST only contain re-exports (type aliases, const, function forwards)
- Each shim file MUST have package declaration: `package frozendb`
- Each shim file MUST import: `import "github.com/susu-dot-dev/frozenDB/internal/frozendb"`
- NO implementation logic allowed in shim files
- NO internal types exposed (e.g., NullRow, DataRow, PartialDataRow, CreateConfig, SudoContext, Header)
- ONLY core operations: open, transaction, query, close

**State**: Static - files don't change at runtime

### 2. Internal Implementation (`/internal/frozendb/*.go`)

**Purpose**: Contains all actual implementation code

**State Transition**: Moved from `/frozendb/*.go` → `/internal/frozendb/*.go`

**Validation Rules**:
- Package declaration remains: `package frozendb` (path changes, not package name)
- Import paths update from `"github.com/susu-dot-dev/frozenDB/frozendb"` → `"github.com/susu-dot-dev/frozenDB/internal/frozendb"`
- NO behavior changes to any implementation
- Zero modifications to spec tests (only import path updates)
- All existing function signatures preserved

**File Count**: ~50 files including:
- Implementation: ~25 .go files
- Unit tests: ~25 *_test.go files  
- Spec tests: ~25 *_spec_test.go files

### 3. CLI Entry Point (`/cmd/frozendb/main.go`)

**Purpose**: Executable entry point for frozenDB CLI

**Attributes**:
- **Package**: `package main`
- **Output**: "Hello world\n" to stdout
- **Exit Code**: 0 on success
- **Dependencies**: fmt (stdlib only for MVP)

**State Transition**: Created new (does not exist currently)

**Validation Rules**:
- MUST compile to executable binary named `frozendb`
- MUST output exactly "Hello world" when run with no arguments
- SHOULD use minimal dependencies for MVP
- Binary MUST be excluded from git (via .gitignore)

**Future Extension Points**:
- Subcommands: create, open, query, dump, etc.
- Flags: --version, --help, --verbose
- Configuration: ENV vars, config files

### 4. Getting Started Example (`/examples/getting_started/main.go`)

**Purpose**: Validate minimal public API completeness and serve as user documentation

**Attributes**:
- **Package**: `package main`
- **Imports**: `"github.com/susu-dot-dev/frozenDB/pkg/frozendb"` for core operations
- **Creation Import**: `"github.com/susu-dot-dev/frozenDB/internal/frozendb"` for CreateFrozenDB (temporary - CLI will handle in future)
- **Workflow**: Create (internal) → Open (public) → Transaction → AddRow → Commit → Query → Close
- **File Operations**: Creates temporary .fdb file, cleans up on exit

**Validation Rules**:
- MUST compile without errors
- MUST use public API from `/pkg/frozendb` for all core operations (open, transaction, query, close)
- MAY use internal package ONLY for database creation (until CLI supports it)
- MUST demonstrate core database operations
- SHOULD include error handling examples
- SHOULD be executable (not just compilable)

**Import Pattern**:
```go
import (
    "github.com/susu-dot-dev/frozenDB/pkg/frozendb"           // Core operations
    internal "github.com/susu-dot-dev/frozenDB/internal/frozendb" // Creation only
)
```

**State**: Creates temporary database file during execution, removes on cleanup

### 5. Build Artifacts

**CLI Binary**: `frozendb` (at repository root)

**Attributes**:
- **Platform**: linux/amd64 (primary), darwin/amd64 (secondary)
- **Build Command**: `go build -o frozendb ./cmd/frozendb`
- **Size**: <10MB (Go binary, minimal dependencies)
- **Permissions**: 0755 (executable)

**Validation Rules**:
- Binary MUST be in .gitignore
- Binary MUST execute successfully: `./frozendb` → "Hello world"
- Binary MUST be rebuildable: `make clean-cli && make build-cli`

**State Transitions**:
- Non-existent → Built (via `make build-cli`)
- Built → Removed (via `make clean-cli`)
- Built → Rebuilt (via `make build-cli` after source changes)

## Directory Structure State Transitions

### Before Refactor
```
/frozendb/              [Implementation + Tests]
  ├── *.go             [50+ files]
  ├── *_test.go
  └── *_spec_test.go
```

### After Refactor
```
/pkg/frozendb/         [Public API Shims - NEW - MINIMAL SURFACE]
  ├── frozendb.go      [FrozenDB, NewFrozenDB, MODE constants, Close]
  ├── transaction.go   [Transaction (no GetEmptyRow/GetRows)]
  ├── errors.go        [Error types]
  └── finder.go        [FinderStrategy, query methods]

/internal/frozendb/    [Implementation - MOVED - ALL FUNCTIONALITY]
  ├── *.go             [Same 50+ files including create.go, header.go]
  ├── *_test.go        [Import paths updated]
  └── *_spec_test.go   [Import paths updated]

/cmd/frozendb/         [CLI Entry Point - NEW]
  └── main.go          [Hello world MVP; future: create command]

/examples/             [Examples - NEW]
  └── getting_started/
      └── main.go      [Uses internal for creation, public for operations]
```

## Import Path State Transitions

### Internal Code (within /internal/frozendb/)

**Before**:
```go
import "github.com/susu-dot-dev/frozenDB/frozendb"
```

**After**:
```go
import "github.com/susu-dot-dev/frozenDB/internal/frozendb"
```

### Public API Shims (/pkg/frozendb/)

**Import** (NEW):
```go
import "github.com/susu-dot-dev/frozenDB/internal/frozendb"
```

### External Users / Examples

**Import** (NEW):
```go
import "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
```

## Validation Rules Summary

### Structural Validation

1. **No Circular Imports**: pkg → internal (never internal → pkg)
2. **No External Access to Internal**: Compiler enforces /internal privacy
3. **Complete API Surface**: All necessary types available via /pkg
4. **Test Coverage Preserved**: All existing tests pass with import updates only

### Behavioral Validation

1. **Zero Behavior Changes**: All database operations work identically
2. **Test Pass Rate**: 100% of existing tests pass
3. **Spec Test Protection**: Zero modifications to spec test logic
4. **Performance**: Build time within 10% of current

### API Validation

1. **CLI Works**: `./frozendb` outputs "Hello world"
2. **Example Compiles**: `go build ./examples/getting_started` succeeds
3. **Example Runs**: Example executes without errors
4. **Public API Complete**: Example can perform all core operations

## Error Conditions

### Refactor Errors

1. **Import Cycle Detected**: Indicates incorrect import path updates
   - **Detection**: `go build` fails with "import cycle" error
   - **Resolution**: Review and fix import path in offending file

2. **Missing Public API**: Example fails to compile
   - **Detection**: `go build ./examples/getting_started` fails
   - **Resolution**: Add missing re-export to appropriate /pkg/frozendb file

3. **Test Failures**: Existing tests fail after refactor
   - **Detection**: `make test` shows failures
   - **Resolution**: Indicates behavior change (unacceptable) - must fix implementation

4. **Spec Test Modifications**: Git diff shows spec test logic changed
   - **Detection**: `git diff` shows changes beyond import statements in *_spec_test.go
   - **Resolution**: Violation of constitution - revert changes

### Build Errors

1. **CLI Build Fails**: `make build-cli` errors
   - **Detection**: Build command exits non-zero
   - **Resolution**: Fix compilation errors in cmd/frozendb/main.go

2. **CLI Wrong Output**: `./frozendb` doesn't output "Hello world"
   - **Detection**: Output differs from expected
   - **Resolution**: Fix main.go implementation

## Summary

This refactor introduces new organizational entities (directories, shim files, CLI, examples) without modifying any database data structures or operational logic. The data model focuses on file organization, import paths, and build artifacts rather than runtime data structures, since this is purely a structural refactor. All validation rules ensure zero behavior changes while establishing clean architectural boundaries.
