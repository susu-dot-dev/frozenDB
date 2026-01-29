# Research: CLI Flag Improvements

**Feature**: 032-cli-flag-improvements  
**Date**: Thu Jan 29 2026  
**Purpose**: Resolve technical unknowns for flexible flag positioning, NOW keyword, and finder strategy selection

---

## 1. CLI Flag Parsing Approach

### Decision
Use **manual os.Args parsing** for flexible flag positioning (flags before or after subcommands).

### Rationale
1. **Project constraints alignment**: frozenDB minimizes dependencies (only `github.com/google/uuid` + stdlib per go.mod). Adding third-party CLI libraries violates this principle.

2. **Appropriate complexity**: With only 7 commands and 2 global flags (`--path`, `--finder`), manual parsing is straightforward and maintainable (~200-300 lines).

3. **Standard library limitations**: Go's `flag` package requires flags AFTER subcommands (e.g., `frozendb begin --path db.frz`). The requirement (FR-001) explicitly needs flags in ANY position.

4. **Existing patterns**: Current implementation already manually parses `os.Args[1]` for subcommand routing. Extending this pattern is natural.

### Implementation Strategy
**Two-pass parsing approach:**
- **Pass 1**: Extract global flags (`--path`, `--finder`) and subcommand from any position in `os.Args`
- **Pass 2**: Parse command-specific positional arguments (key, value, savepointId)

**Key components:**
- Flag extraction loop: ~50 lines
- Duplicate flag detection: ~20 lines
- Case-insensitive value normalization: ~30 lines
- Error messages for missing/invalid flags: ~30 lines

**Complexity estimate**: Simple to Medium (200-300 LOC production + 30-40 test cases)

### Alternatives Considered
1. **cobra library**: Rejected - adds significant dependency (~20k+ LOC), violates "stdlib only" principle
2. **urfave/cli**: Rejected - adds external dependency (~5k LOC) for features we don't need
3. **pflag**: Rejected - still requires wrapper logic for "flags anywhere" pattern
4. **Keep flag package**: Rejected - fails to meet FR-001 requirement

---

## 2. UUIDv7 Generation with github.com/google/uuid

### Decision
Use `uuid.NewV7()` function from github.com/google/uuid v1.6.0 for NOW keyword implementation.

### Rationale
1. **Library support**: The v1.6.0 version (already in use) provides `uuid.NewV7()` function that generates UUIDv7 based on current Unix Epoch timestamp.

2. **Correctness**: UUIDv7 features time-ordered value field with millisecond precision plus random bits for uniqueness. The implementation handles sub-millisecond uniqueness through entropy (per docs: "improved entropy characteristics over versions 1 or 6").

3. **API simplicity**: Single function call with error handling:
   ```go
   key, err := uuid.NewV7()
   if err != nil {
       // Handle error
   }
   ```

4. **Version validation**: Existing `validateUUIDv7()` function in cmd/frozendb/main.go:343-355 already checks `key.Version() != 7`, so generated keys will pass validation.

### Implementation Approach
Modify `handleAdd()` in cmd/frozendb/main.go to detect "NOW" keyword (case-insensitive) and generate UUIDv7:

```go
keyStr := args[0]
var key uuid.UUID
var err error

// Check for NOW keyword (case-insensitive per A-002)
if strings.ToLower(keyStr) == "now" {
    key, err = uuid.NewV7()
    if err != nil {
        printError(pkg_frozendb.NewInvalidInputError("failed to generate UUIDv7", err))
    }
} else {
    key, err = validateUUIDv7(keyStr)
    if err != nil {
        printError(err)
    }
}
```

### Performance Characteristics
- **Uniqueness**: UUIDv7 uses 48 bits for timestamp (millisecond precision) + 74 bits of randomness
- **Time ordering**: Maintained through Unix Epoch timestamp in first 48 bits
- **Collision risk**: Negligible for rapid insertions (A-007: sub-millisecond precision via random bits)
- **Clock skew**: Handled by existing database `skew_ms` configuration (per A-006)

---

## 3. Finder Strategy Selection

### Decision
Parse `--finder` flag value (case-insensitive: "simple", "inmemory", "binary") and map to existing FinderStrategy constants.

### Rationale
1. **Existing implementation**: All three finder strategies already exist:
   - `pkg_frozendb.FinderStrategySimple` (fixed memory, O(n) GetIndex)
   - `pkg_frozendb.FinderStrategyInMemory` (~40 bytes/row, O(1) operations)
   - `pkg_frozendb.FinderStrategyBinarySearch` (optimized for time-ordered UUIDs)

2. **Current usage**: CLI currently hardcodes `FinderStrategySimple` in all commands (main.go lines 100, 134, 168, 216, 274, 322)

3. **Default choice**: Spec requires BinarySearchFinder as default (FR-005, A-004) - best balance of performance and memory for typical workloads

### Implementation Approach
1. **Parse finder flag**: Extract from `os.Args` during global flag parsing
2. **Normalize value**: Case-insensitive matching (A-003) via `strings.ToLower()`
3. **Map to constant**:
   ```go
   func parseFinderStrategy(value string) (pkg_frozendb.FinderStrategy, error) {
       switch strings.ToLower(value) {
       case "", "binary": // default to binary
           return pkg_frozendb.FinderStrategyBinarySearch, nil
       case "simple":
           return pkg_frozendb.FinderStrategySimple, nil
       case "inmemory":
           return pkg_frozendb.FinderStrategyInMemory, nil
       default:
           return "", fmt.Errorf("invalid finder strategy: %s (valid: simple, inmemory, binary)", value)
       }
   }
   ```
4. **Pass to NewFrozenDB**: Replace hardcoded `FinderStrategySimple` with parsed strategy in all 6 database-opening commands

### Scope
- **Affected commands**: begin, commit, savepoint, rollback, add, get (6 commands)
- **Unaffected commands**: create (finder is runtime concern, not stored in database per OS-001)

---

## 4. Add Command Output Format

### Decision
Output only the UUID string with hyphens on a single line to stdout (e.g., `018d5c5a-1234-7890-abcd-ef0123456789`).

### Rationale
1. **Specification requirement**: FR-004 and A-005 explicitly state "just the UUID string" for parsability in shell scripts
2. **Consistency**: Both user-provided and NOW-generated keys output identical format
3. **Unix philosophy**: Silent success with machine-readable output enables piping/scripting

### Implementation
```go
// Success: output the key to stdout (FR-004)
fmt.Println(key.String())
os.Exit(0)
```

### Current behavior comparison
- **Before**: Silent success (no output)
- **After**: UUID string output on success

---

## Summary of Technical Decisions

| Component | Decision | Complexity | Dependencies |
|-----------|----------|------------|--------------|
| Flag parsing | Manual os.Args parsing | Simple-Medium | stdlib only |
| UUIDv7 generation | uuid.NewV7() | Simple | existing (uuid v1.6.0) |
| Finder selection | Parse flag → map to constants | Simple | existing (strategies implemented) |
| Add output | fmt.Println(key.String()) | Trivial | stdlib only |

**Total implementation estimate**: 300-400 lines production code + 40-50 test cases

**Risk assessment**: Low - all changes isolated to CLI layer, no database format changes, existing functionality preserved
