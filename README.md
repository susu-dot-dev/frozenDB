# frozenDB

A single-file key-value database that exploits two guarantees: OS-level append-only files (data never corrupted) and UUIDv7's time-ordered keys (enabling binary search without indexes). The result: sub-linear query performance with constant memory overhead, regardless of database size

## Why frozenDB?

frozenDB's origin story starts with the desire to have data that cannot change once inserted. Postgres can do this - with e.g. [GRANT UPDATE role](https://www.postgresql.org/docs/current/ddl-priv.html), but I didn't want to have an entire separate database deployment for my simple use case. Sqlite doesn't have this kind of functionality, and as I was thinking about this that generally makes sense. The file is literally on-disk - there's _some_, but not a lot of security value in blocking writes from within sqlite, when an attacker could just modify the disk itself.

Ah, but linux does provide OS-level guarantees about files with the [append file attribute](https://www.man7.org/linux/man-pages/man1/chattr.1.html). So, down the rabbit hole I went. First you start with "I'll just append a series of events (basically a WAL), to "but then how would transactions work" and also "linear scans make me sad". And that, my friends, is how you end up with your own database library for funsies.

So, if you're interested in an exploration of a database with interesting constraints - single file, append-only, sub-linear search, then keep on exploring. If you're here for the spec-driven-development of the repository, then this is definitely the right place for you. If you want to deploy this to production, maybe don't?

Anyways, here's what you get with frozenDB:

**Concurrent reads without coordination**: Because all writes append to the end of the file, readers can safely access any committed data without locks, waiting, or coordination with writers.

**OS-level data immutability**: Once data successfully reaches disk, it cannot be modified or deleted without root permissions. Traditional databases enforce immutability through application logic—frozenDB leverages the operating system's append-only file semantics to make tampering impossible at the filesystem level.

**Corruption resistance**: Partial writes or crashes only affect uncommitted transactions at the end of the file. All previously committed data remains intact and immediately readable without recovery procedures. Sentinel bytes validate transaction boundaries, making corruption detection explicit. Any data-recovery can preserve the initial, working parts of the database and just clip the end of the database

## Performance

An append-only, single-file structure presents an obvious challenge: there's no straightforward way to maintain an index or any auxiliary data structure that would normally be updated as the database grows. Traditional databases sidestep this by allowing in-place modifications to index files—frozenDB cannot.

frozenDB leverages **UUIDv7's time-ordered structure** to solve this. UUIDv7 embeds a timestamp in its most significant bits, so keys written via append-only operations naturally arrive in roughly chronological order. This ordering enables **binary search** directly on the data file: O(log n) lookups without requiring a separate index.

To handle clock skew and out-of-order inserts, frozenDB uses a configurable time skew window. Binary search locates the approximate position, then a bounded linear probe finds the exact key within the skew window.

**Fixed memory overhead**: With the `BinarySearchFinder` strategy, memory usage remains constant regardless of database size. This makes frozenDB viable in memory-constrained environments where the database may grow to arbitrary sizes but available RAM is fixed.

## Quick Start

The CLI provides a basic playground to explore frozenDB's behavior before integrating it into your application.

```bash
# Download the latest release from https://github.com/susu-dot-dev/frozenDB/releases

# Create a new database (requires sudo for append-only file attribute)
❯ sudo frozendb create data.fdb

# Add data with a UUIDv7 key (NOW generates a new UUIDv7)
❯ frozendb --path data.fdb add NOW '{"name": "FrozenDB allows any JSON type"}'
019c0c83-25e6-79b7-8d08-cd94eacd66ea

# Add another row
❯ frozendb --path data.fdb add NOW '"All additions are inserted as part of a transaction, so you can commit or roll them back"'
019c0c83-b524-721d-9a6f-5e9b2468317f

# Uncommitted data cannot be read
❯ frozendb --path data.fdb get 019c0c83-25e6-79b7-8d08-cd94eacd66ea
Error: key_not_found: key exists only in uncommitted transaction (caused by: transaction_active: transaction has no ending row)

# Commit the transaction
❯ frozendb --path data.fdb commit

# Now the data is readable
❯ frozendb --path data.fdb get 019c0c83-25e6-79b7-8d08-cd94eacd66ea
{
  "name": "FrozenDB allows any JSON type"
}
```

## Application integration

For Go applications, import frozenDB as a dependency:

```bash
go get github.com/susu-dot-dev/frozenDB/pkg/frozendb
```

```go
package main

import (
    "encoding/json"
    "log"

    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
)

type MyData struct {
    Name string `json:"name"`
}

func main() {
    // Open database (create via CLI first: sudo frozendb create data.fdb)
    db, err := frozendb.NewFrozenDB(
        "data.fdb",
        frozendb.MODE_WRITE,
        frozendb.FinderStrategyBinarySearch,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Begin transaction
    tx, err := db.BeginTx()
    if err != nil {
        log.Fatal(err)
    }

    // Add data with UUIDv7 keys
    key1 := uuid.Must(uuid.NewV7())
    data1 := MyData{Name: "FrozenDB allows any JSON type"}
    jsonData1, _ := json.Marshal(data1)
    
    if err := tx.AddRow(key1, jsonData1); err != nil {
        log.Fatal(err)
    }
    log.Printf("Added row with key: %s", key1)

    key2 := uuid.Must(uuid.NewV7())
    jsonData2, _ := json.Marshal("All additions are inserted as part of a transaction, so you can commit or roll them back")
    
    if err := tx.AddRow(key2, jsonData2); err != nil {
        log.Fatal(err)
    }
    log.Printf("Added row with key: %s", key2)

    // Uncommitted data cannot be read yet
    var retrieved MyData
    if err := db.Get(key1, &retrieved); err != nil {
        log.Printf("Expected error: %v", err)
        // Error: key_not_found: key exists only in uncommitted transaction
    }

    // Commit the transaction
    if err := tx.Commit(); err != nil {
        log.Fatal(err)
    }
    log.Println("Transaction committed")

    // Now the data is readable
    if err := db.Get(key1, &retrieved); err != nil {
        log.Fatal(err)
    }
    log.Printf("Retrieved: %s", retrieved.Name)
    // Output: Retrieved: FrozenDB allows any JSON type
}
```

For a complete working example, see [examples/getting_started/main.go](examples/getting_started/main.go).

## Database Design

The core challenge: support transaction management with immediate disk writes, maintain append-only semantics (no modifications ever), and enable efficient binary search. These requirements push against each other—how do you manage transactions when you can't modify or delete data? How do you binary search when you can't maintain a separate index?

**Fixed-width rows.** Each row is exactly `row_size` bytes (specified in the header). To read row `i`, seek to `header + i * row_size`. The fixed width eliminates the need for index files or offset tables.

**Sentinels for alignment.** Each row begins with `0x1F` and ends with `0x0A`. If corruption causes misalignment, the sentinels catch it immediately. These bytes—along with null padding—cannot appear in JSON-encoded strings, preventing data from being mistaken for structural markers.

**Embedded transaction state.** We can't use separate control rows because they'd break the addressing math (row `i` must be at a predictable offset). Instead, transaction state lives inside each data row. `BeginTx()` writes a row with `start_control = T`. `AddRow()` writes the UUID key and JSON value. But the transaction's fate—commit or rollback—is determined by `end_control`:

```
[0x1F][start_control][UUID key][JSON value][padding][end_control][parity][0x0A]
  ↑         ↑             ↑          ↑          ↑          ↑          ↑      ↑
 start    T or R      24 bytes   variable   null bytes  TC/RE/R0  2 bytes  end
```

- `RE` = continue transaction, more rows coming
- `TC` = commit transaction, all rows valid
- `R0-R9` = rollback to savepoint N

**Sequential writes.** Rows are written progressively as the user provides data, not atomically. Call `AddRow(key, value)` and the row start, UUID, and JSON are written immediately. The `end_control` is written later based on the next operation (commit, rollback, or add another row). By ordering writes carefully within the fixed row size, all transaction logic embeds directly in the row—no follow-up metadata or external control records needed.

Parity bytes provide a per-row checksum for corruption detection.

This design satisfies all constraints: append-only writes are safe (sentinels and parity detect incomplete writes), transactions work without deletion (control codes mark validity), and binary search works (fixed-width enables direct addressing). Every `AddRow()` writes to disk immediately; durability doesn't depend on buffering or waiting for commits.

The full specification is in [docs/v1_file_format.md](docs/v1_file_format.md).

## Local Development

```bash
# Clone the repository
git clone https://github.com/susu-dot-dev/frozenDB.git
cd frozenDB

# Build the CLI
make build-cli
# Binary output: dist/frozendb

# Run the complete CI pipeline (deps, tidy, fmt, lint, test, build)
make ci
```

**Codebase structure:**
- `specs/` - Spec-driven development: feature specifications drive implementation
- `docs/` - Durable documentation: file format specs and design docs
- `internal/frozendb/` - Core implementation (not importable externally)
- `pkg/frozendb/` - Public library API (imports from internal)
- `cmd/frozendb/` - CLI tool (imports from internal)

Before submitting changes, run `make ci` to ensure all checks pass (formatting, linting, tests, build).

## Status

frozenDB is currently an **academic curiosity**. The code works, has high-level performance guarantees (O(log n) lookups, constant memory overhead), and includes a comprehensive test suite. However:

- **Not battle-tested**: Has not been deployed in production environments
- **File format not optimized**: Prioritizes simplicity and correctness over space efficiency
- **Performance not tuned**: Disk reads and other operations have not been optimized for real-world workloads

Use this project to explore append-only database design and UUIDv7-based indexing, but not for production systems requiring proven reliability or optimized performance.

## License

MIT
