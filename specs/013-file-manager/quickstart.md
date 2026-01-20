# FileManager Quickstart

**Spec**: 013-file-manager\
**Date**: 2026-01-20\
**Version**: 1.0.0

## Overview

The FileManager provides thread-safe file operations for frozenDB with
concurrent read support and exclusive write control. This quickstart covers the
essential patterns for using FileManager.

## Example 1: Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Create a FileManager
    fm, err := frozendb.NewFileManager("mydatabase.fdb")
    if err != nil {
        log.Fatal(err)
    }
    defer fm.Close()

    // Read some data
    data, err := fm.Read(64, 1024) // Read 1KB starting at offset 64
    if err != nil {
        log.Printf("Read error: %v", err)
        return
    }
    fmt.Printf("Read %d bytes\n", len(data))
}
```

## Example 2: Exclusive Writes with Completion Signals

```go
package main

import (
    "fmt"
    "log"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    fm, err := frozendb.NewFileManager("mydatabase.fdb")
    if err != nil {
        log.Fatal(err)
    }
    defer fm.Close()

    // Acquire exclusive write access
    dataChan := make(chan frozendb.Data, 10)
    err = fm.SetWriter(dataChan)
    if err != nil {
        log.Fatal("Failed to get writer:", err)
    }

    // Write some data with completion notification
    responseChan := make(chan error, 1)
    dataChan <- frozendb.Data{
        Bytes:    []byte("Hello, frozenDB!"),
        Response: responseChan,
    }

    // Wait for write completion
    writeErr := <-responseChan
    if writeErr != nil {
        log.Printf("Write failed: %v", writeErr)
    } else {
        fmt.Println("Write completed successfully")
    }

    // Write more data
    responseChan2 := make(chan error, 1)
    dataChan <- frozendb.Data{
        Bytes:    []byte("Another write operation"),
        Response: responseChan2,
    }

    writeErr2 := <-responseChan2
    if writeErr2 != nil {
        log.Printf("Second write failed: %v", writeErr2)
    } else {
        fmt.Println("Second write completed successfully")
    }

    // Close writer when done
    close(dataChan)
    fmt.Println("Writer closed")
    fm.Close()
}
```
