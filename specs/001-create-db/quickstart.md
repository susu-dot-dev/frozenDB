# Quickstart Guide: frozenDB File Creation

## Overview

The `Create` function initializes a new frozenDB database file with proper immutability protection, sudo context handling, and atomic operations. This guide covers basic usage, error handling, and common patterns.

## Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Create a new database file
    config := frozendb.CreateConfig{
        Path:    "/path/to/database.fdb",
        RowSize:  1024,
        SkewMs:   5000,
    }
    err := frozendb.Create(config)
    if err != nil {
        log.Fatalf("Failed to create database: %v", err)
    }
    
    fmt.Println("Database created successfully!")
}
```

## Prerequisites

### Sudo Requirements

The `Create` function requires sudo privileges because it sets the append-only attribute (`chattr +a`) which is a privileged operation on Linux.

```bash
# Run with sudo
sudo go run main.go

# Or if the binary is already compiled
sudo ./myapp
```

### Environment Variables (Sudo Context)

When running under sudo, the following environment variables must be present:

- `SUDO_USER`: Original username
- `SUDO_UID`: Original user ID
- `SUDO_GID`: Original group ID

These are automatically set by sudo and used to ensure the created file is owned by the original user.

## Parameters

### path string
- **Description**: Filesystem path for the new database file
- **Requirements**: Must end with `.fdb`, parent directory must exist and be writable
- **Examples**: 
  - `"/var/lib/myapp/data.fdb"` (absolute)
  - `"./data.fdb"` (relative to current directory)
  - `".hidden.fdb"` (hidden file)

### rowSize int
- **Description**: Size of each data row in bytes
- **Range**: 128 to 65536 bytes
- **Impact**: Determines the maximum size of values that can be stored
- **Examples**:
  - `1024` (1KB rows)
  - `4096` (4KB rows)
  - `65536` (64KB rows)

### skewMs int
- **Description**: Time skew window in milliseconds for distributed systems
- **Range**: 0 to 86400000 (24 hours)
- **Purpose**: Allows for clock differences between nodes
- **Examples**:
  - `0` (no time skew, single-node system)
  - `5000` (5 seconds skew)
  - `60000` (1 minute skew)

## Error Handling

### Configuration-Based Creation

For better readability and extensibility, use the CreateConfig struct approach:

```go
// Simple struct-based creation
config := frozendb.CreateConfig{
    Path:    "/path/to/database.fdb",
    RowSize:  1024,
    SkewMs:   5000,
}
err := frozendb.Create(config)
```

```go
// Advanced configuration with validation
config := frozendb.CreateConfig{
    Path:    "/var/lib/app/database.fdb",
    RowSize:  4096,
    SkewMs:   10000,
}
if err := config.Validate(); err != nil {
    log.Fatal("Invalid config:", err)
}
err := frozendb.Create(config)
```

```go
// Advanced configuration with validation
config := frozendb.CreateConfig{
    Path:    "/var/lib/app/database.fdb",
    RowSize:  4096,
    SkewMs:   10000,
}
if err := config.Validate(); err != nil {
    log.Fatal("Invalid config:", err)
}
err := frozendb.Create(config)
```

### Error Types

```go
func handleCreateError(err error) {
    switch err.(type) {
    case *frozendb.InvalidInputError:
        fmt.Println("Invalid input:", err)
        // Check parameters: empty path, invalid ranges, wrong extension
        
    case *frozendb.PathError:
        fmt.Println("Path error:", err)
        // Check: parent directory exists, path is writable, file doesn't exist
        
    case *frozendb.WriteError:
        fmt.Println("Write error:", err)
        // Check: sudo permissions, disk space, filesystem support
        
    default:
        fmt.Println("Unexpected error:", err)
    }
}
```

### Common Error Scenarios

```go
// Invalid input examples
config := frozendb.CreateConfig{Path: "", RowSize: 1024, SkewMs: 0}                    // InvalidInputError: empty path
config = frozendb.CreateConfig{Path: "/path/file.txt", RowSize: 1024, SkewMs: 0}       // InvalidInputError: wrong extension
config = frozendb.CreateConfig{Path: "/path/file.fdb", RowSize: 64, SkewMs: 0}         // InvalidInputError: rowSize too small
config = frozendb.CreateConfig{Path: "/path/file.fdb", RowSize: 70000, SkewMs: 0}      // InvalidInputError: rowSize too large
config = frozendb.CreateConfig{Path: "/path/file.fdb", RowSize: 1024, SkewMs: 90000000} // InvalidInputError: skewMs too large
frozendb.Create(config)                                                                   // Pass invalid config

// Path errors
config := frozendb.CreateConfig{Path: "/nonexistent/file.fdb", RowSize: 1024, SkewMs: 0}
frozendb.Create(config)                                                                   // PathError: parent doesn't exist

config = frozendb.CreateConfig{Path: "/readonly/file.fdb", RowSize: 1024, SkewMs: 0}
frozendb.Create(config)                                                                   // PathError: parent not writable

config = frozendb.CreateConfig{Path: "/existing/file.fdb", RowSize: 1024, SkewMs: 0}
frozendb.Create(config)                                                                   // PathError: file already exists

// Write errors (sudo-related)
config = frozendb.CreateConfig{Path: "/path/file.fdb", RowSize: 1024, SkewMs: 0}
frozendb.Create(config)                                                                   // WriteError: no sudo context
```

## File Properties

After successful creation, the database file will have:

### Permissions
- **File Mode**: 0644 (`rw-r--r--`)
- **Owner**: Original user (when created under sudo)
- **Group**: Original user's primary group (when created under sudo)

### Attributes
- **Append-Only**: Set via `chattr +a` (immutable except for appends)
- **File Size**: 64 bytes (header only, no data rows yet)

### File System Location
- Can be created anywhere the user has write access
- Can be created in any directory that supports append-only attribute
- Supports both absolute and relative paths

## Troubleshooting

### Permission Denied
- **Cause**: Running without sudo when append-only attribute is required
- **Solution**: Run with `sudo`

### Operation Not Permitted  
- **Cause**: Filesystem doesn't support `chattr` (e.g., some network mounts)
- **Solution**: Use a local filesystem that supports extended attributes

### No Space Left on Device
- **Cause**: Insufficient disk space
- **Solution**: Free up disk space and retry

### Invalid Cross-Device Link
- **Cause**: Attempting to set ownership across different filesystems
- **Solution**: Create database on the same filesystem as the user's home directory
