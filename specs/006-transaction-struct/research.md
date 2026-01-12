# Research: Transaction Struct Implementation

## Decision: Single Slice with Direct Indexing

**Rationale**: Based on the clarification in the specification session, the complexity of virtual indexing across two slices was removed in favor of a simpler, more direct approach. A single slice with maximum 100 rows provides O(1) access while meeting all functional requirements.

**Alternatives considered**: 
- Two-slice virtual indexing: Originally considered but removed for complexity
- Linked list structure: Would add unnecessary overhead and complexity
- Map-based indexing: Would complicate ordered access

## Decision: DataRow-Based Architecture

**Rationale**: The Transaction struct will wrap existing DataRow objects, leveraging the mature v1 file format implementation. This maintains consistency with the existing codebase and avoids duplication of transaction logic.

**Alternatives considered**:
- Raw byte parsing: Would require reimplementing existing DataRow functionality
- Custom transaction format: Would break compatibility with existing frozenDB files

## Decision: Rollback Logic Implementation

**Rationale**: Implement rollback semantics exactly as specified in v1_file_format.md:
- Full rollback (R0/S0): All rows invalidated including rollback row
- Partial rollback (R1-R9/S1-S9): Rows 1 through savepoint N valid, N+1 through rollback invalid
- Commit (TC/SC): All rows through commit row valid

**Alternatives considered**:
- Lazy evaluation: Would complicate state tracking and violate correctness requirements
- Index-based invalidation: Would add unnecessary complexity for 100-row limit

## Decision: Savepoint Detection Strategy

**Rationale**: Use EndControl pattern matching with 'S' as first character to identify savepoints. Count savepoints in creation order to number them 1-9, with savepoint 0 representing transaction start.

**Alternatives considered**:
- Separate savepoint tracking: Would duplicate information already available in EndControl
- Complex indexing structures: Unnecessary given the maximum 9 savepoint limit

## Decision: Thread Safety Design

**Rationale**: Transaction struct is inherently thread-safe due to immutable underlying DataRow slice. All methods are read-only operations on immutable data, enabling concurrent access without synchronization.

**Alternatives considered**:
- Mutex-based synchronization: Unnecessary overhead for read-only operations
- Copy-on-write patterns: Would add memory overhead without benefits

## Decision: Validation Framework

**Rationale**: Implement comprehensive validation following v1_file_format.md requirements:
- Transaction must start with T and continue with R's
- Exactly one transaction-ending command required
- Maximum 100 rows and 9 savepoints enforced
- Rollback targets must exist within transaction

**Alternatives considered**:
- Partial validation: Would risk data corruption scenarios
- Deferred validation: Would complicate error handling and debugging

## Decision: Error Handling Strategy

**Rationale**: Since all rows are validated at insert time, transaction parsing errors indicate either:
- **DatabaseCorruption**: Unexpected structural violations (rows should be valid but aren't)
- **InvalidInstruction**: Logic errors in transaction construction (e.g., calling Savepoint() before inserting a row)

This simplifies error handling to two existing error types:
- `CorruptDatabaseError` for corruption scenarios (parsing valid rows that shouldn't be invalid)
- `InvalidInputError` for logic/instruction errors (improper API usage)

**Alternatives considered**:
- Three separate error types (InvalidTransactionError, SavepointError, RollbackError): Unnecessary complexity since all indicate either corruption or logic errors
- Generic errors: Would reduce debugging capability

## Technical Implementation Details

### DataRow Structure Understanding
- `StartControl`: Single byte ('T' for transaction start, 'R' for continuation)
- `EndControl`: Two bytes encoding savepoint and transaction state
- Transaction payload: Base64 UUIDv7 key + JSON value
- Fixed-width row format with parity bytes for integrity

### EndControl Pattern Analysis
- `TC`/`SC`: Transaction commit (with/without savepoint)
- `RE`/`SE`: Transaction continue (with/without savepoint)
- `R0-R9`/`S0-S9`: Rollback commands (with/without savepoint creation)

### Savepoint Numbering Logic
- Savepoints numbered 1-9 in creation order
- Savepoint 0 represents transaction start
- `S0-S9` creates savepoint first, then performs rollback

### Transaction State Machine
- **Closed**: Expecting `T` start_control
- **Open**: Expecting `R` start_control
- Transaction ends on first `*C` or `*0-9` EndControl

### Performance Considerations
- Fixed memory usage regardless of database size
- O(1) indexing within 100-row slice
- Thread-safe concurrent reads without synchronization
- Validation overhead acceptable for correctness requirements