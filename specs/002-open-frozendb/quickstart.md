# Quickstart Guide: Open frozenDB Files

**Feature**: 002-open-frozendb  
**Date**: 2026-01-09  
**Purpose**: Getting started with frozen database file operations

## Prerequisites

- Go 1.25.5 or later
- frozenDB module imported: `import "github.com/susu-dot-dev/frozenDB"`
- Existing frozenDB database file (.fdb extension)

## Basic Usage

### Opening for Read Access

```go
package main

import (
    "fmt"
    "log"
    "github.com/susu-dot-dev/frozenDB"
)

func main() {
    // Open database file for read-only access
    db, err := frozendb.NewFrozenDB("data/mydatabase.fdb", frozendb.MODE_READ)
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close() // Ensure cleanup
    
    fmt.Println("Database opened successfully in read mode")
    // Database is ready for read operations
}
```

### Opening for Write Access

```go
package main

import (
    "fmt"
    "log"
    "github.com/susu-dot-dev/frozenDB"
)

func main() {
    // Open database file for read-write access
    db, err := frozendb.NewFrozenDB("data/mydatabase.fdb", frozendb.MODE_WRITE)
    if err != nil {
        log.Fatal("Failed to open database for writing:", err)
    }
    defer db.Close() // Ensure cleanup
    
    fmt.Println("Database opened successfully in write mode")
    // Database is ready for write operations
}
```

## Error Handling Patterns

### Comprehensive Error Handling

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "github.com/susu-dot-dev/frozenDB"
)

func main() {
    db, err := frozendb.NewFrozenDB("data/mydatabase.fdb", frozendb.MODE_READ)
    if err != nil {
        handleOpenError(err)
        return
    }
    defer db.Close()
    
    fmt.Println("Database opened successfully")
}

func handleOpenError(err error) {
    var invalidInputErr *frozendb.InvalidInputError
    var pathErr *frozendb.PathError
    var corruptErr *frozendb.CorruptDatabaseError
    var writeErr *frozendb.WriteError
    
    switch {
    case errors.As(err, &invalidInputErr):
        fmt.Println("Error: Invalid input parameters")
        fmt.Println("Check file path has .fdb extension and mode is 'read' or 'write'")
        
    case errors.As(err, &pathErr):
        fmt.Println("Error: Filesystem access issue")
        fmt.Println("Check file exists and you have read permissions")
        
    case errors.As(err, &corruptErr):
        fmt.Println("Error: Database file is corrupted")
        fmt.Println("File may need to be restored from backup")
        
    case errors.As(err, &writeErr):
        fmt.Println("Error: Cannot acquire write lock")
        fmt.Println("Another process may have database open in write mode")
        
    default:
        fmt.Printf("Unexpected error: %v\n", err)
    }
}
}
```

### Safe Cleanup

```go
func safeDatabaseOperation(path string) error {
    db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ)
    if err != nil {
        return err
    }
    
    // Use defer with error handling
    // Close() is thread-safe, so multiple defer statements are safe
    defer func() {
        if closeErr := db.Close(); closeErr != nil {
            log.Printf("Warning: failed to close database: %v", closeErr)
        }
    }()
    
    // Database operations here
    return nil
}
```

### Concurrent Close Safety

```go
func concurrentCloseExample(path string) error {
    db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ)
    if err != nil {
        return err
    }
    
    // Multiple goroutines can safely call Close() on the same instance
    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // Safe to call Close() from multiple goroutines
            _ = db.Close()
        }()
    }
    wg.Wait()
    
    return nil
}
```

## Concurrent Access Patterns

### Multiple Readers

```go
func concurrentReaders(path string) error {
    // Multiple goroutines can safely read from the same file
    // No locks needed - append-only files are safe for concurrent reads
    for i := 0; i < 5; i++ {
        go func(readerID int) {
            db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ)
            if err != nil {
                log.Printf("Reader %d failed: %v", readerID, err)
                return
            }
            defer db.Close()
            
            // Perform read operations
            log.Printf("Reader %d operating", readerID)
        }(i)
    }
    return nil
}
```

### Writer Coordination

```go
func attemptWriteAccess(path string) error {
    db, err := frozendb.NewFrozenDB(path, frozendb.MODE_WRITE)
    if err != nil {
        var writeErr *frozendb.WriteError
        if errors.As(err, &writeErr) {
            fmt.Println("Database is locked by another writer")
            fmt.Println("Readers can continue - only writers need exclusive locks")
            fmt.Println("Try again later or check for stuck processes")
        }
        return err
    }
    defer db.Close()
    
    fmt.Println("Successfully acquired write lock")
    // Perform write operations
    return nil
}
```

## Integration with Create Operation

### Create and Then Open

```go
func createAndOpenDatabase(path string) error {
    // First create the database (from spec 001)
    err := frozendb.Create(frozendb.CreateConfig{
        Path:    path,
        RowSize: 1024,
        SkewMs:  5000,
    })
    if err != nil {
        return fmt.Errorf("create failed: %w", err)
    }
    
    // Then open it for operations
    db, err := frozendb.NewFrozenDB(path, frozendb.MODE_WRITE)
    if err != nil {
        return fmt.Errorf("open failed: %w", err)
    }
    defer db.Close()
    
    fmt.Println("Database created and opened successfully")
    return nil
}
```

## Common Usage Scenarios

### Read-Only Data Analysis

```go
func analyzeDatabase(path string) error {
    db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ)
    if err != nil {
        return err
    }
    defer db.Close()
    
    // Database is ready for read-only analysis
    // Future read methods (Get, Count, Enumerate) would be called here
    
    fmt.Printf("Database %s opened for analysis\n", path)
    return nil
}
```

### Data Ingestion

```go
func ingestData(path string) error {
    db, err := frozendb.NewFrozenDB(path, frozendb.MODE_WRITE)
    if err != nil {
        return fmt.Errorf("cannot open for writing: %w", err)
    }
    defer db.Close()
    
    // Database is ready for data ingestion
    // Future write methods (Add, Put) would be called here
    
    fmt.Printf("Database %s opened for data ingestion\n", path)
    return nil
}
```

## Best Practices

### Resource Management

```go
// Good: Always close database
func goodExample() {
    db, err := frozendb.NewFrozenDB("data.db", frozendb.MODE_READ)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close() // Guaranteed cleanup
    // Use database...
}

// Bad: Resource leak
func badExample() {
    db, err := frozendb.NewFrozenDB("data.db", frozendb.MODE_READ)
    if err != nil {
        log.Fatal(err)
    }
    // Missing defer db.Close() - resource leak!
    // Use database...
}
```

### Error Checking

```go
// Good: Comprehensive error handling
func openWithValidation(path, mode string) error {
    if !strings.HasSuffix(path, ".fdb") {
        return fmt.Errorf("path must have .fdb extension")
    }
    
    if mode != frozendb.MODE_READ && mode != frozendb.MODE_WRITE {
        return fmt.Errorf("mode must be frozendb.MODE_READ or frozendb.MODE_WRITE")
    }
    
    db, err := frozendb.NewFrozenDB(path, mode)
    if err != nil {
        return err
    }
    defer db.Close()
    
    fmt.Println("Database opened and validated successfully")
    return nil
}
```

### Concurrent Safety

```go
// Good: One database instance per goroutine
func concurrentSafe(path string) {
    for i := 0; i < 10; i++ {
        go func() {
            db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ)
            if err != nil {
                log.Printf("Goroutine failed: %v", err)
                return
            }
            defer db.Close()
            
            // Each goroutine has its own instance
            // No sharing of database instances across goroutines
        }()
    }
}

// Bad: Sharing instances across goroutines
func concurrentUnsafe(path string) {
    db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Don't share this db instance across goroutines!
    // Each goroutine should create its own instance
}
```

## Troubleshooting

### Common Issues

1. **"File not found"**: Check the file path and ensure it exists
2. **"Permission denied"**: Verify read/write permissions on the file
3. **"Database corruption"**: The file may be damaged or not a valid frozenDB file
4. **"Lock acquisition failed"**: Another process has the database locked
5. **"Invalid input"**: Check file path has .fdb extension and mode is valid

### Debug Mode

```go
func debugOpen(path, mode string) {
    fmt.Printf("Attempting to open: path=%s, mode=%s\n", path, mode)
    
    db, err := frozendb.NewFrozenDB(path, mode)
    if err != nil {
        fmt.Printf("Error type: %T\n", err)
        fmt.Printf("Error details: %v\n", err)
        return
    }
    defer db.Close()
    
    fmt.Println("Successfully opened database")
}
```

## Integration Examples

### Web Server Usage

```go
func setupDatabaseMiddleware(path string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ)
            if err != nil {
                http.Error(w, "Database unavailable", http.StatusInternalServerError)
                return
            }
            defer db.Close()
            
            // Store db in request context for handlers
            ctx := context.WithValue(r.Context(), "db", db)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### CLI Application

```go
func main() {
    if len(os.Args) < 3 {
        fmt.Println("Usage: myapp <database.fdb> <mode>")
        fmt.Println("Modes: read, write")
        os.Exit(1)
    }
    
    path := os.Args[1]
    mode := os.Args[2]
    
    db, err := frozendb.NewFrozenDB(path, mode)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()
    
    // Application logic here
    fmt.Printf("Database %s opened in %s mode\n", path, mode)
}
```

This quickstart provides the essential patterns for opening and working with frozenDB database files. The API is designed to be simple and safe, with comprehensive error handling and resource management built-in.