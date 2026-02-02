# Testing Guide for frozenDB

This document describes patterns and best practices for writing tests in frozenDB. It covers how to create test databases, populate them with rows, and structure tests effectively.

## Core Principle: Avoid Direct Create() Calls

**IMPORTANT:** Tests should generally NEVER call `Create()` directly as it requires elevated privileges (sudo). The only exception is when directly testing the Create functionality itself.

**Why Avoid Create()?**
- Requires elevated privileges for file attribute operations
- Slower (file I/O vs in-memory)
- Complex setup (requires mocking syscalls and environment variables)
- Poor test isolation

**When to Use Create():**
- Only when testing Create functionality itself (e.g., `create_test.go`, `create_spec_test.go`)
- Always use the `setupCreate()` helper which handles all required mocking

## Test Database Creation Patterns

### Pattern 1: In-Memory Mock Databases (Preferred for Unit Tests)

Build test databases entirely in memory using `buildTestDatabase()`. This is fast, requires no file I/O, and provides complete control.

```go
rows := []testRow{
    {rowType: "data", value: `{"name":"test"}`, 
     startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
}
data, keys, header := buildTestDatabase(rowSize, rows)

dbFile := newMockGetDBFile(data, MODE_READ)
finder, _ := NewSimpleFinder(dbFile, rowSize)
db := &FrozenDB{file: dbFile, header: header, finder: finder}
```

**Use for:** Unit tests, row parsing, corrupted/partial row simulation, benchmarks

### Pattern 2: File-Based Tests with setupCreate()

For integration tests requiring real files, use `setupCreate()` which handles all mocking:

```go
dir := t.TempDir()
path := setupCreate(t, dir, 0)  // 0 uses default skewMs
dbAddDataRow(t, path, uuid.Must(uuid.NewV7()), `{"value":"test"}`)
```

**Use for:** Integration tests, file locking, conformance tests, concurrent access

### Pattern 3: Transaction Tests with Mock Writers

Test transaction logic without file I/O using `createTransactionWithMockWriter()`:

```go
header := createTestHeader()
tx := createTransactionWithMockWriter(header)
// Test transaction operations
```

**Use for:** Transaction logic, commit/rollback, savepoints, error handling

## Populating Test Databases

### File-Based Database Helpers

```go
// Add single data row
dbAddDataRow(t, path, key, `{"name":"alice"}`)

// Add null row (empty transaction)
dbAddNullRow(t, path)

// Add multiple rows with specific timestamps (in ms)
addDataRowsInOrder(t, path, []int{1000, 2000, 3000})

// Create UUIDv7 with specific timestamp for testing
key := uuidFromTS(1000)  // Timestamp: 1000ms
```

**Note:** `uuidFromTS()` generates valid DataRow UUIDs (not NullRow UUIDs)

## Mock Implementations

### buildTestDatabase() - In-Memory Database Builder

Creates complete database in memory with precise control over contents:

```go
type testRow struct {
    rowType      string       // "data", "null", "checksum", "partial", "corrupt"
    value        string       // JSON value for data rows
    startControl StartControl
    endControl   EndControl
    bytesWritten int          // For partial rows
}

rows := []testRow{
    {rowType: "data", value: `{"a":1}`, 
     startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
    {rowType: "null", 
     startControl: START_TRANSACTION, endControl: NULL_ROW_CONTROL},
    {rowType: "checksum"},
    {rowType: "partial", bytesWritten: 25},
    {rowType: "corrupt"},
}

data, keys, header := buildTestDatabase(rowSize, rows)
// Returns: data []byte, keys []uuid.UUID, header *Header
```

### Row Builder Functions

```go
// Individual row builders (used internally by buildTestDatabase)
buildDataRow(rowSize, key, `{"value":"data"}`, START_TRANSACTION, TRANSACTION_COMMIT)
buildNullRow(rowSize, START_TRANSACTION, NULL_ROW_CONTROL)
buildChecksumRow(rowSize, uint32(12345))
buildPartialDataRow(rowSize, 50)  // 50 bytes written
```

### Mock DBFile Implementations

```go
// mockGetDBFile: Full-featured mock for Get() testing
mockFile := newMockGetDBFile(data, MODE_READ)
mockFile.injectReadError(1, NewReadError("forced error", nil))
mockFile.simulateClose()

// mockDBFile: Minimal mock for transaction testing
// (See transaction_spec_test.go for implementation)
```

## Test Setup and Teardown

```go
// Use t.TempDir() for automatic cleanup
dir := t.TempDir()

// Use defer for resources
db, err := NewFrozenDB(path, MODE_READ, FinderStrategySimple)
if err != nil { t.Fatalf("error: %v", err) }
defer db.Close()

// Use t.Cleanup() for complex cleanup
setupMockSyscalls(false, false)
t.Cleanup(restoreRealSyscalls)

// Factory functions return cleanup
finder, cleanup := simpleFinderFactory(t, path, confRowSize)
defer cleanup()
```

## Common Test Helpers

Located in `finder_conformance_test.go`:

```go
setupCreate(t, dir, 0)                    // Create database
dbAddDataRow(t, path, key, `{"v":"d"}`)   // Add data row
dbAddNullRow(t, path)                     // Add null row
addDataRowsInOrder(t, path, []int{...})   // Add rows by timestamp
uuidFromTS(1000)                          // UUID with timestamp
createTestHeader()                        // Test header
```

Benchmark variants use `b` parameter: `setupCreateB()`, `dbAddDataRowB()`

## Best Practices

1. **Use table-driven tests** for multiple scenarios
2. **Test both success and error paths**
3. **Prefer in-memory tests** for unit tests (faster, better isolation)
4. **Use descriptive test names**: `TestGet_ReturnsErrorWhenKeyNotFound`
5. **Use subtests** for related scenarios: `t.Run("scenario", func(t *testing.T) {...})`
6. **Always clean up resources** with defer or t.Cleanup()
7. **Use constants** instead of magic numbers: `confRowSize`, `confSkewMs`
8. **Mark helpers with t.Helper()** for better error reporting
9. **Avoid t.Parallel()** - incompatible with `t.Setenv()` used by `createTestDatabase()` and adds complexity

## Writing Spec Tests

Spec tests validate functional requirements from specification documents and are typically the **first task** when implementing a feature. Unlike unit tests which focus on implementation details and edge cases, spec tests verify that functional requirements (FR-XXX) are correctly implemented from the user/system perspective. See [spec_testing.md](spec_testing.md) for complete guidelines.

### Critical Rules for Spec Test Generation

Spec tests must be written as complete, functional tests from the start. This is a strict requirement with no exceptions.

**NEVER stub out tests.** Do not write placeholder tests with `t.Skip("TODO")` or empty test bodies with the intention of implementing them later. Every spec test must be fully implemented when written. If you find yourself wanting to stub a test, this indicates the implementation task should be broken down differently.

**NEVER write tests that only exercise code without verification.** A test that calls a function but doesn't check its return value or side effects is not a valid test. Every spec test must include assertions that verify the actual behavior matches what the specification requires. The test must be written such that it would fail if the behavior is incorrect. There must be some part of the test that will fail if the functional requirement is not satisfied

**IF behavior verification is impossible, STOP immediately.** Do not proceed with writing the spec test. Instead, STOP and explicitly ask the user how to proceed. Some functional requirements are hidden implementation details, or otherwise difficult to verify the behavior. If the spec has not provided clear guidance about how to proceed, you MUST stop and ask for guidance, and not break the spec testing conventions as described.

### Test Structure

Every spec test should follow a clear three-phase structure:

**Setup Phase:** Create the test environment including any necessary databases, objects, or state. Use the appropriate helper functions (`setupCreate()`, `buildTestDatabase()`, `createTransactionWithMockWriter()`) based on what you're testing. Ensure all setup is complete before moving to execution.

**Execute Phase:** Call the function or operation being tested. This should be a clear, single operation that exercises the functional requirement. Avoid mixing multiple operations unless the requirement specifically concerns their interaction.

**Verify Phase:** Assert that the actual behavior matches what the spec requires. This is the most critical phase. Use explicit comparisons that check actual values against expected values. If testing error conditions, verify both that an error occurred AND that it's the correct type of error. If testing state changes, verify the state is actually in the expected condition.

## Quick Reference

**Never call Create() directly** - use `setupCreate()` or in-memory mocks

**For unit tests:**
```go
data, keys, header := buildTestDatabase(rowSize, rows)
```

**For integration tests:**
```go
path := setupCreate(t, dir, 0)
dbAddDataRow(t, path, key, value)
```

**For transaction tests:**
```go
tx := createTransactionWithMockWriter(header)
```

## See Also

- [spec_testing.md](spec_testing.md) - Guidelines for spec-driven test development
- [v1_file_format.md](v1_file_format.md) - File format specification
- [finder_conformance.md](finder_conformance.md) - Finder implementation testing
- [error_handling.md](error_handling.md) - Error handling patterns
