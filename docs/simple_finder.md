# SimpleFinder Implementation Specification

## 1. Introduction

The SimpleFinder is a reference implementation of the Finder protocol that uses direct disk-based scanning without caching or optimization techniques. This implementation is designed for maximum correctness and serves as a baseline to validate optimized finder implementations against.

### 1.1. Design Philosophy

The SimpleFinder operates on fundamental principles:

- **Disk-Based Operations**: All data comes from direct disk reads via DBFile interface
- **Minimal In-Memory State**: Only tracks current database file size
- **One-Row-at-a-Time Processing**: Processes individual rows sequentially
- **Reference Implementation**: Intended for correctness validation, not production performance

### 1.2. Use Cases

- **Correctness Baseline**: Reference implementation for testing optimized finders
- **Development Tool**: Simple, predictable behavior for debugging
- **Small Database Scenarios**: Where simplicity is valued over performance
- **Specification Validation**: Demonstrates protocol compliance

## 2. In-Memory State Management

### 2.1. Size and MaxTimestamp Tracking

The SimpleFinder maintains two pieces of in-memory state:

**Size Tracking**: Tracks the extent of data confirmed via `OnRowAdded()` callbacks. When created, SimpleFinder initializes `size` to the current database file size from `dbFile.Size()`.

**MaxTimestamp Tracking**: Tracks the maximum timestamp among all complete non-checksum rows (DataRows and NullRows). This is a dynamic value that increases as new rows with higher timestamps are added.

**Initialization**: During creation, SimpleFinder performs a linear scan to initialize max_timestamp:
- Start from the last complete row and scan backwards to the first row
- Look for DataRow or NullRow entries (both have UUIDv7 timestamps)
- If found, set max_timestamp to the maximum timestamp found among all rows
- If no data or null rows are found, set max_timestamp to 0

**Important Distinction**: The Finder's `max_timestamp` is a dynamic tracking value. NullRow UUID timestamps are fixed values set at insertion time (equal to the database's `max_timestamp` at that moment), but the Finder's `max_timestamp` can increase as new rows are added.

### 2.2. Memory Constraints

**Memory Usage**: Exactly one row of memory for struct fields plus temporary buffer during operations:
- **Struct Size**: ~24 bytes (3 int64 fields + interface reference)
- **Read Buffer**: One row_size bytes during read operations
- **Total Memory**: O(row_size) - constant regardless of database size

## 3. GetIndex() Implementation

### 3.1. Algorithm

1. Calculate total number of complete rows in database by dividing available bytes by row_size
2. Iterate through each row index from 0 to totalRows-1
3. For each row:
   - Read row bytes from disk using DBFile.Read() at calculated offset
   - Parse row as RowUnion to determine row type
   - If row is a DataRow and its UUID matches the target key, return current index
   - Continue to next row if no match
4. If all rows examined without finding match, return key not found error

### 3.2. Performance Characteristics

- **Time Complexity**: O(n) where n is number of rows in database
- **Disk I/O**: One disk read per row examined
- **Best Case**: O(1) - key found in first row
- **Worst Case**: O(n) - key not found or in last row
- **Memory**: Constant O(row_size) regardless of database size

### 3.3. Row Matching Logic

For each row during GetIndex() search:

1. Parse the row bytes using RowUnion.UnmarshalText() to determine row type
2. If parsing fails, treat row as corrupted and skip to next row
3. If row type is DataRow, compare its UUID key with the target key
4. If UUID matches, return current index as found
5. If row type is ChecksumRow, NullRow, or PartialDataRow, skip to next row (non-searchable types)
6. Continue until all rows are examined or match is found

## 4. GetTransactionStart() Implementation

### 4.1. Algorithm

1. Validate the input index is within bounds and does not point to a checksum row
2. Read the row at the given index to check if it starts a transaction
3. If the row has start_control = 'T', return the input index (it starts the transaction)
4. If not, scan backward through preceding rows:
   - For each index from current-1 down to 0:
   - Read the row bytes from disk
   - Parse row to determine if it has start_control = 'T'
   - If found, return that index as transaction start
   - If no transaction start found in all preceding rows, return malformed transaction error

### 4.2. Performance Characteristics

- **Best Case**: O(1) when input row itself starts a transaction
- **Worst Case**: O(k) where k is distance to transaction start (maximum ~101 rows)
- **Disk I/O**: Up to 101 disk reads in worst case (100 data rows + 1 checksum row)
- **Memory Usage**: Constant O(row_size) regardless of database size

## 5. GetTransactionEnd() Implementation

### 5.1. Algorithm

1. Validate the input index is within bounds and does not point to a checksum row
2. Read the row at the given index to check if it ends a transaction
3. If the row has a transaction-ending end_control, return the input index (it ends the transaction)
4. If not, scan forward through subsequent rows:
   - For each index from current+1 to totalRows-1:
   - Read the row bytes from disk
   - Parse row to determine if it has transaction-ending end_control
   - If found, return that index as transaction end
   - If no transaction end found in remaining rows, return transaction active error

### 5.2. Performance Characteristics

- **Best Case**: O(1) when input row itself ends a transaction
- **Worst Case**: O(k) where k is distance to transaction end (maximum ~101 rows)
- **Disk I/O**: Up to 101 disk reads in worst case (100 data rows + 1 checksum row)
- **Memory Usage**: Constant O(row_size) regardless of database size

## 6. Helper Methods

### 6.1. Row Reading

```go
func (sf *SimpleFinder) readRow(index int64) ([]byte, error) {
    offset := HEADER_SIZE + index * int64(sf.rowSize)
    return sf.dbFile.Read(offset, sf.rowSize)
}
```

### 6.2. Transaction Detection

**Row Ends Transaction**: Parse row and check for transaction-ending end_control:
- For DataRows: End control[1] is 'C' (commit) or '0'-'9' (rollback)
- For NullRows: End control is 'NR'
- ChecksumRows are never in transactions

### 6.3. Index Validation

Validate that index is non-negative, within file bounds, and does not point to a checksum row. Parse the row to verify it is not a ChecksumRow type.

## 7. MaxTimestamp() Implementation

### 7.1. Algorithm

The SimpleFinder implements MaxTimestamp() with O(1) time complexity using the cached max_timestamp value.

**Method Implementation:**
```go
func (sf *SimpleFinder) MaxTimestamp() int64 {
    return sf.maxTimestamp
}
```

**Initialization Algorithm:**
1. Calculate total number of complete rows in database by dividing available bytes by row_size
2. Get skew_ms from database header (time skew window in milliseconds)
3. Initialize max_timestamp to 0 and rows_scanned to 0
4. Start from the last complete row (index: totalRows-1) and scan backwards towards index 0
5. For each row during backward scan:
   - Read row bytes from disk
   - Parse row as RowUnion to determine row type
   - Increment rows_scanned counter
   - If row is a DataRow or NullRow:
     - If row's timestamp > current max_timestamp, update max_timestamp to the row's timestamp
   - Continue scanning backwards until either:
     - rows_scanned >= skew_ms (ensuring we've scanned enough rows from the end to account for clock skew), OR
     - index reaches 0 (beginning of database)
6. After scan completes, max_timestamp contains the true maximum timestamp accounting for clock skew

### 7.2. Performance Characteristics

- **Initialization**: O(min(n, skew_ms)) where n is number of rows and skew_ms is the time skew window from the database header. The algorithm scans backwards from the end for at least skew_ms rows (or until reaching the beginning of the database), ensuring correct max_timestamp calculation while accounting for clock skew.
- **Queries**: O(1) time complexity for MaxTimestamp() calls
- **Memory**: Additional 8 bytes for max_timestamp field
- **Disk I/O**: During initialization only - subsequent queries are memory-based. The backward scan reads at most min(totalRows, skew_ms) rows from disk.

## 8. OnRowAdded() Implementation

### 8.1. Algorithm

1. Calculate expected next row index by dividing current size by row_size
2. If input index equals expected next index:
   - Update internal size by adding one row_size (confirming the new row)
   - **MaxTimestamp Update**: If the added row is a DataRow or NullRow, update max_timestamp with the row's timestamp if it's greater than current max_timestamp
   - Return success
3. If input index is less than expected (existing data position):
   - Return error indicating row index does not match expected position
4. If input index is greater than expected (skipped positions):
   - Return error indicating row index skips positions in database
5. The size tracking ensures Finder only reads as far as the last confirmed row

### 8.2. MaxTimestamp Update Logic

When OnRowAdded() is called with a new row:

**Row Type Analysis:**
- **DataRow**: Compare row timestamp with current max_timestamp, update if greater
- **NullRow**: Compare row timestamp with current max_timestamp, update if greater. Note: NullRow UUID timestamps are fixed at insertion time (equal to the database's `max_timestamp` at that moment), but the Finder's `max_timestamp` tracking value can still increase if the NullRow's timestamp is greater than the current tracked maximum.
- **ChecksumRow**: Do not update max_timestamp (checksum rows have no relevant timestamp)
- **PartialDataRow**: Do not update max_timestamp (incomplete transaction data)

**Timestamp Comparison:**
- Extract timestamp from row's UUIDv7 time component
- If timestamp > current max_timestamp, update max_timestamp
- Ensure atomic update to maintain consistency

### 7.2. Size Update Logic

The SimpleFinder only tracks the confirmed file size via `OnRowAdded()` callbacks:

- **Initial Size**: Current database file size when SimpleFinder is created
- **Update Trigger**: Only when `OnRowAdded()` is called with expected next index
- **Increment**: Size increases by exactly one `rowSize` per confirmed row
- **Validation**: Ensures no gaps in confirmed row indices

## 9. Error Handling

### 9.1. Error Types

SimpleFinder returns specific error types for different failure conditions:

- **KeyNotFoundError**: When `GetIndex()` cannot find the specified UUID
- **InvalidInputError**: When indices are out of bounds or point to checksum rows
- **CorruptDatabaseError**: When rows cannot be parsed or contain invalid data

- **TransactionActiveError**: When transaction is still open (no end found)
- **ReadError**: When disk read operations fail

## 10. Production Considerations

### 10.1. Limitations

- **Performance**: Not optimized for large databases or frequent queries
- **Concurrency**: No caching - each operation re-reads from disk
- **Memory Usage**: Minimal but at cost of repeated I/O operations
- **Use Case**: Reference implementation and correctness validation only

### 10.2. When to Use

- **Testing**: Validate optimized finder implementations against SimpleFinder results
- **Development**: Debugging transaction boundary issues and MaxTimestamp behavior
- **Small Databases**: Where simplicity outweighs performance needs
- **Specification Compliance**: Demonstrating protocol requirements including O(1) MaxTimestamp()

### 10.3. When NOT to Use

- **Production Systems**: Where performance is critical (initialization O(n) scan)
- **Large Databases**: With millions of rows where initialization time is significant
- **High Concurrency**: Scenarios with many concurrent readers
- **Performance-Sensitive Applications**: Where query latency matters

The SimpleFinder provides a clean, predictable baseline that prioritizes correctness over performance, making it ideal for specification validation and development scenarios. The MaxTimestamp() implementation demonstrates how to achieve O(1) query performance while maintaining protocol compliance.
