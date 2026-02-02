# Concurrent Reader/Writer Example

This example demonstrates frozenDB's support for concurrent read and write operations using two goroutines:

1. **Reader Goroutine**: Opens the database in `MODE_READ` and polls for a specific key every 1 second
2. **Writer Goroutine**: Opens the database in `MODE_WRITE`, waits 3 seconds, writes the key, waits 2 more seconds, then commits

## Key Concepts Demonstrated

- **Concurrent Access**: Multiple processes can safely access the same database file simultaneously
- **Read Mode**: `MODE_READ` allows multiple readers with no file locking
- **Write Mode**: `MODE_WRITE` requires exclusive file lock (only one writer at a time)
- **Transaction Visibility**: Keys are only visible to readers AFTER the transaction is committed
- **Mutex-Protected Output**: Uses a shared mutex to prevent console output from different goroutines from getting interleaved

## Running the Example

### From Source

```bash
cd examples/concurrent_reader_writer
go build -o concurrent_example main.go
./concurrent_example
```

### From Built Binaries

```bash
# Build all examples
make build-examples

# Run from dist directory
cd dist/examples
./concurrent_reader_writer
```

The example will automatically find `concurrent_reader_writer.sample.fdb` in the same directory as the binary.

## Expected Output

The output shows the timeline of concurrent operations:

1. Both goroutines start and open the database
2. Reader polls and finds the key does NOT exist (polls 1-5)
3. Writer sleeps 3 seconds, then begins transaction and writes the key
4. Reader continues polling - key still doesn't exist during uncommitted transaction
5. Writer commits the transaction after 2 more seconds
6. Reader immediately finds the key on the next poll (poll 6)

## Timeline

```
Time    Reader                          Writer
----    ------                          ------
0.0s    Open in MODE_READ              
0.5s                                    Open in MODE_WRITE
1.0s    Poll #1: Not found             
2.0s    Poll #2: Not found              
3.0s    Poll #3: Not found              Begin transaction
3.5s                                    Write key (uncommitted)
4.0s    Poll #4: Not found             
5.0s    Poll #5: Not found              
5.5s                                    Commit transaction
6.0s    Poll #6: ✓ Found!              Exit
```

## Important Notes

- The reader sees "Key does not exist" while the transaction is uncommitted
- This demonstrates frozenDB's ACID transaction semantics
- The append-only file format enables safe concurrent reads without blocking
- The mutex (`printMutex`) ensures console output doesn't get spliced between goroutines
- This example modifies `sample.fdb` - the database will contain the new key after running

## Clean Up

To reset the database for re-running from source:

```bash
cp ../getting_started/sample.fdb sample.fdb
```

Or to reset the database in dist:

```bash
cp concurrent_reader_writer.sample.fdb.backup concurrent_reader_writer.sample.fdb
```

Note: The example modifies the database file, so you may want to make a backup before running.
