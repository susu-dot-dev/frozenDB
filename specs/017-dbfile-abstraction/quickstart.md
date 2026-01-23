# Quickstart: DBFile Read/Write Modes and File Locking

This quickstart shows how to use the enhanced DBFile interface for read and write operations with proper file locking.

## Opening Files

### Read-Only Access
```go
// Open existing database for reading only
db, err := NewDBFile("mydatabase.fdb", "read")
if err != nil {
    return err
}
defer db.Close()

// Use with read-only transaction
tx, err := NewTransaction(db, header)
if err != nil {
    return err
}
```

### Write Access with Exclusive Lock
```go
// Open database for writing (acquires exclusive lock)
db, err := NewDBFile("mydatabase.fdb", "write")
if err != nil {
    return err // Fails fast if another process has write lock
}
defer db.Close()

// Use with write transaction
tx, err := NewTransaction(db, header)
if err != nil {
    return err
}
```

## Usage Patterns

### Multiple Readers Simultaneously
```go
// Multiple processes can open the same file for reading
db1, _ := NewDBFile("mydatabase.fdb", "read")  // Success
db2, _ := NewDBFile("mydatabase.fdb", "read")  // Success
db3, _ := NewDBFile("mydatabase.fdb", "read")  // Success

// All can perform read operations concurrently
data1, _ := db1.Read(0, 100)
data2, _ := db2.Read(100, 100)
data3, _ := db3.Read(200, 100)
```

### Exclusive Write Access
```go
// Only one writer can access the file at a time
writer1, err := NewDBFile("mydatabase.fdb", "write")
if err != nil {
    return err
}
defer writer1.Close()

// Another process trying to write will fail
writer2, err := NewDBFile("mydatabase.fdb", "write")
if err != nil {
    // err is WriteError - file is locked by writer1
    fmt.Printf("Cannot acquire write lock: %v\n", err)
}
```

## Mode-Specific Behavior

### Read Mode Limitations
```go
db, _ := NewDBFile("mydatabase.fdb", "read")

// Read operations work
data, _ := db.Read(0, 100)
size := db.Size()

// Write operations are blocked
err := db.SetWriter(writeChan)  // Returns InvalidActionError
fmt.Printf("Write error: %v\n", err)
```

### Write Mode Capabilities  
```go
db, _ := NewDBFile("mydatabase.fdb", "write")

// Both read and write operations work
data, _ := db.Read(0, 100)
err := db.SetWriter(writeChan)  // Success
```

## Error Handling

### Handle Mode Errors
```go
db, err := NewDBFile("mydatabase.fdb", "invalid")
if err != nil {
    // err is InvalidInputError
    fmt.Printf("Mode error: %v\n", err)
}
```

### Handle Lock Conflicts
```go
db, err := NewDBFile("mydatabase.fdb", "write")
if err != nil {
    // Check if it's a lock error
    if _, ok := err.(*WriteError); ok {
        fmt.Println("File is locked by another process")
    }
    return err
}
```

## Migration from Direct File Operations

### Before (open.go direct usage)
```go
file, err := openDatabaseFile("mydatabase.fdb", MODE_WRITE)
if err != nil {
    return err
}
err = acquireFileLock(file, MODE_WRITE, false)
if err != nil {
    return err
}
```

### After (DBFile interface)
```go
db, err := NewDBFile("mydatabase.fdb", "write")
if err != nil {
    return err // Handles mode validation and locking
}
defer db.Close()
```

The DBFile interface consolidates file operations, mode handling, and locking into a single, easy-to-use API while maintaining all existing functionality.