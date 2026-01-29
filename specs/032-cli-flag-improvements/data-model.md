# Data Model: CLI Flag Improvements

**Feature**: 032-cli-flag-improvements  
**Date**: Thu Jan 29 2026  
**Purpose**: Define new data entities, validation rules, and state changes for CLI enhancements

---

## Overview

This feature introduces CLI-level data structures and validation rules without modifying the database file format or core data model. All changes are confined to command-line argument parsing and processing.

---

## 1. Global CLI Flags

### Entity: GlobalFlags

**Purpose**: Represents parsed global flags that apply across all subcommands.

**Attributes**:
- `path` (string, required): Database file path for all database operations
- `finder` (FinderStrategyValue, optional): Finder strategy selection (default: "binary")
- `subcommand` (string, required): The CLI subcommand to execute

**Source**: Parsed from `os.Args` using flexible positioning algorithm (can appear before or after subcommand)

**Validation Rules**:
1. **VR-001**: `path` flag MUST be present for all commands except `create`
2. **VR-002**: `path` flag MUST NOT be specified more than once (duplicate detection)
3. **VR-003**: `finder` flag MUST NOT be specified more than once
4. **VR-004**: `finder` value MUST be empty string (default) or one of: "simple", "inmemory", "binary" (case-insensitive)
5. **VR-005**: At least one subcommand MUST be present in arguments
6. **VR-006**: Exactly one subcommand MUST be specified (no multiple subcommands)

**Error Mapping**:
- Missing `path`: `InvalidInputError("missing required flag: --path", nil)`
- Duplicate `path`: `InvalidInputError("duplicate flag: --path", nil)`
- Duplicate `finder`: `InvalidInputError("duplicate flag: --finder", nil)`
- Invalid `finder` value: `InvalidInputError("invalid finder strategy: <value> (valid: simple, inmemory, binary)", nil)`
- Missing subcommand: `InvalidInputError("missing subcommand", nil)` (existing error from main.go:26)
- Unknown subcommand: `InvalidInputError("unknown command: <name>", nil)` (existing error from main.go:57)

---

## 2. Finder Strategy Value

### Entity: FinderStrategyValue

**Purpose**: Represents the user-provided finder strategy choice from CLI input.

**Type**: String (parsed from `--finder` flag value)

**Valid Values** (case-insensitive):
- `""` (empty/omitted): Maps to BinarySearchFinder (default per FR-005)
- `"simple"`, `"Simple"`, `"SIMPLE"`: Maps to FinderStrategySimple
- `"inmemory"`, `"InMemory"`, `"INMEMORY"`: Maps to FinderStrategyInMemory
- `"binary"`, `"Binary"`, `"BINARY"`: Maps to FinderStrategyBinarySearch

**Normalization**: Convert to lowercase before mapping (per A-003)

**Mapping to Internal Constants**:
```
CLI Value (normalized) → Internal Constant → Implementation
--------------------------------------------------------------
""           → pkg_frozendb.FinderStrategyBinarySearch → binary_search_finder.go
"simple"     → pkg_frozendb.FinderStrategySimple       → simple_finder.go
"inmemory"   → pkg_frozendb.FinderStrategyInMemory     → inmemory_finder.go
"binary"     → pkg_frozendb.FinderStrategyBinarySearch → binary_search_finder.go
```

**State Transition**: CLI flag value → Normalized string → FinderStrategy constant → NewFrozenDB parameter → Finder interface implementation

**Scope**: Applies only to runtime finder selection for current command; NOT persisted in database (per OS-001)

---

## 3. NOW Keyword

### Entity: NOWKeyword

**Purpose**: Special keyword recognized by `add` command to trigger automatic UUIDv7 key generation.

**Type**: String constant (case-insensitive)

**Recognition Rules**:
1. **VR-007**: The string "NOW" (any case: "now", "NOW", "Now", "nOw", etc.) in the key argument position triggers UUIDv7 generation
2. **VR-008**: Normalized via `strings.ToLower(keyStr) == "now"` for case-insensitive matching

**State Transition Flow**:
```
User input: "NOW" (any case)
    ↓
Case-insensitive detection: strings.ToLower(keyStr) == "now"
    ↓
Generate UUIDv7: key, err := uuid.NewV7()
    ↓
Validate version: key.Version() == 7 (should always pass)
    ↓
Use in transaction: tx.AddRow(key, value)
```

**Generated Key Characteristics**:
- Format: UUIDv7 per RFC 4122 draft (48-bit timestamp + 74-bit random)
- Time component: Unix Epoch milliseconds (current system time)
- Version byte: 7 (validated by existing `validateUUIDv7()`)
- Ordering: Chronologically ordered (Constitution principle IV)
- Uniqueness: Ensured by random bits for sub-millisecond insertions (A-007)

**Error Mapping**:
- UUID generation failure: `InvalidInputError("failed to generate UUIDv7", err)` where err is from uuid.NewV7()
- Duplicate key (collision): `DatabaseError` (existing behavior from tx.AddRow)

---

## 4. Command Output Entity

### Entity: AddCommandOutput

**Purpose**: Structured output from `add` command upon successful completion.

**Attributes**:
- `key` (UUID string): The key that was inserted (user-provided or NOW-generated)

**Format**: Single line containing UUID with hyphens (per FR-004, A-005)
```
018d5c5a-1234-7890-abcd-ef0123456789
```

**Output Rules**:
1. **VR-009**: Output MUST be written to stdout (not stderr)
2. **VR-010**: Output MUST be a single line (terminated by newline)
3. **VR-011**: Format MUST be standard UUID string with hyphens (via `key.String()`)
4. **VR-012**: Output format MUST be identical for both user-provided and NOW-generated keys

**State Change**: Modifies existing behavior
- **Before**: Silent success (no stdout output, exit code 0)
- **After**: Key printed to stdout, exit code 0

**Usage Pattern** (enables shell scripting):
```bash
# Capture generated key
key=$(frozendb add --path db.frz NOW '{"data": "value"}')
echo "Inserted with key: $key"

# Use in subsequent commands
frozendb get --path db.frz "$key"
```

---

## 5. Flag Parsing State Machine

### Entity: FlagParserState

**Purpose**: Internal state during two-pass parsing of `os.Args`.

**State Attributes**:
- `seenPath` (bool): Tracks if `--path` flag encountered (for duplicate detection)
- `seenFinder` (bool): Tracks if `--finder` flag encountered (for duplicate detection)
- `pathValue` (string): Extracted path value
- `finderValue` (string): Extracted finder value (empty if not specified)
- `subcommand` (string): Identified subcommand
- `remainingArgs` ([]string): Positional arguments for subcommand

**State Transitions**:
```
START
  ↓
Parse os.Args sequentially
  ↓
For each arg:
  - If "--path" and next arg exists → extract path, set seenPath=true
  - If "--finder" and next arg exists → extract finder, set seenFinder=true
  - If no "--" prefix and subcommand empty → set subcommand
  - Otherwise → append to remainingArgs
  ↓
Validation checks:
  - seenPath == true (for non-create commands) → PASS else ERROR
  - seenPath was set at most once → PASS else ERROR (duplicate)
  - seenFinder was set at most once → PASS else ERROR (duplicate)
  ↓
Normalize finderValue (case-insensitive, default to "binary")
  ↓
Map finderValue to FinderStrategy constant
  ↓
Route to subcommand handler with parsed state
  ↓
END
```

**Error States**:
- Missing required flags → InvalidInputError
- Duplicate flags → InvalidInputError
- Invalid flag values → InvalidInputError
- Unknown flags → Can be ignored (for extensibility) or error (for strictness)

---

## 6. Data Flow Relationships

### Component Interactions

```
os.Args (user input)
    ↓
FlagParserState (two-pass parsing)
    ↓
GlobalFlags (validated structure)
    ├─→ pathValue → NewFrozenDB(path, mode, strategy)
    ├─→ finderValue → FinderStrategyValue → FinderStrategy constant
    └─→ subcommand → Command handler routing
            ↓
Command-specific argument parsing
    ├─→ "add" command
    │     ├─→ keyStr == "now" (case-insensitive) → NOWKeyword detection
    │     │     ↓
    │     │   uuid.NewV7() → Generated UUIDv7 key
    │     │     ↓
    │     ├─→ tx.AddRow(key, value)
    │     └─→ AddCommandOutput: fmt.Println(key.String())
    │
    └─→ Other commands (begin, commit, etc.)
          └─→ Existing behavior (with finder strategy support)
```

---

## 7. Validation Rule Summary

| Rule ID | Validation | Error Type | Error Message |
|---------|------------|------------|---------------|
| VR-001 | `--path` required (except create) | InvalidInputError | "missing required flag: --path" |
| VR-002 | `--path` not duplicated | InvalidInputError | "duplicate flag: --path" |
| VR-003 | `--finder` not duplicated | InvalidInputError | "duplicate flag: --finder" |
| VR-004 | `--finder` value valid | InvalidInputError | "invalid finder strategy: <value> (valid: simple, inmemory, binary)" |
| VR-005 | Subcommand present | InvalidInputError | existing: "Usage: frozendb <command> [arguments]" |
| VR-006 | One subcommand only | InvalidInputError | existing: routing handles this |
| VR-007 | NOW keyword recognized (case-insensitive) | N/A (detection) | N/A |
| VR-008 | NOW normalization correct | N/A (internal) | N/A |
| VR-009 | Add output to stdout | N/A (behavioral) | N/A |
| VR-010 | Add output single line | N/A (behavioral) | N/A |
| VR-011 | Add output UUID format | N/A (behavioral) | N/A |
| VR-012 | Add output consistency | N/A (behavioral) | N/A |

---

## 8. No Changes to Existing Data Model

**Important**: This feature does NOT modify:
- Database file format (v1 file format unchanged)
- Row structures (DataRow, NullRow, ChecksumRow remain identical)
- Transaction semantics (start_control, end_control unchanged)
- Key storage format (UUIDv7 already supported in database)
- Value storage format (JSON RawMessage unchanged)
- Finder implementations (Simple, InMemory, BinarySearch unchanged)

All changes are **CLI-layer only** - no impact on core database operations, data integrity, or file format.
