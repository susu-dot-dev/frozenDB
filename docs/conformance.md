# Transaction and Savepoint Control Code Conformance Tests

This document provides comprehensive examples of valid and invalid control code sequences for transactions and savepoints in frozenDB v1. Each example is represented as an array of tuples `(start_control, end_control)`.

## 1. Valid Sequences

### 1.1 Simple Commits

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 1.1.1 | `[(T, TC)]` | Single row transaction with commit |
| 1.1.2 | `[(T, RE), (R, TC)]` | Two row transaction with commit |
| 1.1.3 | `[(T, RE), (R, RE), (R, TC)]` | Three row transaction with commit |
| 1.1.4 | `[(T, RE), (R, RE), (R, RE), (R, TC)]` | Four row transaction with commit |

---------

### 1.2 Commits with Savepoints

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 1.2.1 | `[(T, SC)]` | Single row with savepoint and commit |
| 1.2.2 | `[(T, SE), (R, TC)]` | Savepoint on first row, commit on second |
| 1.2.3 | `[(T, SE), (R, RE), (R, TC)]` | Savepoint on first row, continue, then commit |
| 1.2.4 | `[(T, SE), (R, SE), (R, TC)]` | Two savepoints, commit on third row |
| 1.2.5 | `[(T, SE), (R, RE), (R, SE), (R, TC)]` | Savepoint on first, continue, savepoint on third, commit on fourth |
| 1.2.6 | `[(T, SE), (R, SE), (R, SE), (R, RE), (R, TC)]` | Multiple savepoints with commit |

---------

### 1.3 Full Rollbacks (R0/S0)

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 1.3.1 | `[(T, R0)]` | Single row full rollback |
| 1.3.2 | `[(T, RE), (R, R0)]` | Two row full rollback |
| 1.3.3 | `[(T, RE), (R, RE), (R, R0)]` | Three row full rollback |
| 1.3.4 | `[(T, S0)]` | Single row with savepoint and full rollback |
| 1.3.5 | `[(T, SE), (R, R0)]` | Two row with savepoint and full rollback |
| 1.3.6 | `[(T, SE), (R, RE), (R, R0)]` | Savepoint on first, continue, then full rollback |

---------

### 1.4 Partial Rollbacks (R1-R9, S1-S9)

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 1.4.1 | `[(T, SE), (R, R1)]` | Savepoint on first row, rollback to savepoint 1 |
| 1.4.2 | `[(T, SE), (R, RE), (R, R1)]` | Savepoint on first, continue, rollback to savepoint 1 |
| 1.4.3 | `[(T, SE), (R, SE), (R, R1)]` | Two savepoints, rollback to savepoint 1 |
| 1.4.4 | `[(T, SE), (R, SE), (R, R2)]` | Two savepoints, rollback to savepoint 2 |
| 1.4.5 | `[(T, SE), (R, RE), (R, SE), (R, R2)]` | Savepoint on first, continue, savepoint on third, rollback to savepoint 2 |
| 1.4.6 | `[(T, SE), (R, SE), (R, SE), (R, R2)]` | Three savepoints, rollback to savepoint 2 |
| 1.4.7 | `[(T, SE), (R, SE), (R, SE), (R, R3)]` | Three savepoints, rollback to savepoint 3 |
| 1.4.8 | `[(T, SE), (R, S1)]` | Rollback with savepoint on rollback row (S1) |
| 1.4.9 | `[(T, SE), (R, SE), (R, S1)]` | Two savepoints, rollback to savepoint 1 with savepoint on rollback row |
| 1.4.10 | `[(T, SE), (R, SE), (R, S2)]` | Two savepoints, rollback to savepoint 2 with savepoint on rollback row |
| 1.4.11 | `[(T, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, R9)]` | Maximum savepoints (9), rollback to savepoint 9 |
| 1.4.12 | `[(T, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, RE), (R, RE), (R, RE), (R, RE), (R, R5)]` | Maximum savepoints (9), rollback to savepoint 5 |

---------

### 1.5 Edge Cases

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 1.5.1 | `[(T, SC)]` | Single row transaction with savepoint and commit (savepoint 1, then commit) |
| 1.5.2 | `[(T, S0)]` | Single row transaction with savepoint and full rollback |
| 1.5.3 | `[(T, S1)]` | Single row transaction with savepoint and rollback to savepoint 1 |

---------

## 2. Invalid Sequences

### 2.1 Incomplete Transactions

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.1.1 | `[(T, RE)]` | Transaction started but never ended (missing commit/rollback) |
| 2.1.2 | `[(T, RE), (R, RE)]` | Transaction started, continued, but never ended |
| 2.1.3 | `[(T, RE), (R, RE), (R, RE), (R, RE)]` | Multiple continuations without termination |

---------

### 2.2 Nested Transactions

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.2.1 | `[(T, RE), (T, TC)]` | Transaction started, then another transaction started before first ended |
| 2.2.2 | `[(T, RE), (R, RE), (T, TC)]` | Transaction started, continued, then another transaction started |
| 2.2.3 | `[(T, TC), (R, TC)]` | Transaction with commit, then continuation row (should be T, not R) |

---------

### 2.3 Invalid Start Control Sequences

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.3.1 | `[(R, TC)]` | Row continuation without transaction start |
| 2.3.2 | `[(R, RE)]` | Row continuation without previous transaction |
| 2.3.3 | `[(R, RE), (R, TC)]` | Multiple row continuations without transaction start |

---------

### 2.4 Multiple Transaction-Ending Commands

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.4.1 | `[(T, TC), (R, TC)]` | Two commits in same transaction |
| 2.4.2 | `[(T, TC), (R, R0)]` | Commit then rollback in same transaction |
| 2.4.3 | `[(T, R0), (R, TC)]` | Rollback then commit in same transaction |
| 2.4.4 | `[(T, R0), (R, R1)]` | Two rollbacks in same transaction |
| 2.4.5 | `[(T, TC), (R, RE)]` | Commit, then continuation (invalid - transaction already ended) |

---------

### 2.5 Invalid Rollback Targets

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.5.1 | `[(T, RE), (R, R1)]` | Rollback to savepoint 1 when no savepoints exist |
| 2.5.2 | `[(T, SE), (R, R2)]` | Rollback to savepoint 2 when only one savepoint exists |
| 2.5.3 | `[(T, SE), (R, SE), (R, R3)]` | Rollback to savepoint 3 when only two savepoints exist |
| 2.5.4 | `[(T, SE), (R, RE), (R, RE), (R, RE), (R, R5)]` | Rollback to savepoint 5 when only one savepoint exists |
| 2.5.5 | `[(T, RE), (R, S1)]` | Rollback to savepoint 1 with S1 when no savepoints exist |
| 2.5.6 | `[(T, SE), (R, S2)]` | Rollback to savepoint 2 with S2 when only one savepoint exists |

---------

### 2.6 Invalid Savepoint Numbers

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.6.1 | `[(T, SE), (R, RE), (R, RE), (R, RE), (R, RE), (R, RE), (R, RE), (R, RE), (R, RE), (R, RE), (R, R10)]` | Rollback to savepoint 10 (exceeds maximum of 9) |
| 2.6.2 | `[(T, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, S10)]` | Rollback to savepoint 10 with S10 (exceeds maximum) |

---------

### 2.7 Too Many Savepoints

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.7.1 | `[(T, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, SE), (R, TC)]` | 10 savepoints (exceeds maximum of 9) |

---------

### 2.8 Invalid End Control Sequences

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.8.1 | `[(T, TE)]` | Invalid end_control: TE (T continuation doesn't make sense) |
| 2.8.2 | `[(T, RC)]` | Invalid end_control: RC (R commit - should be TC) |
| 2.8.3 | `[(T, TT)]` | Invalid end_control: TT (double T) |
| 2.8.4 | `[(T, RR)]` | Invalid end_control: RR (double R) |
| 2.8.5 | `[(T, SS)]` | Invalid end_control: SS (double S) |
| 2.8.6 | `[(T, CC)]` | Invalid end_control: CC (double C) |
| 2.8.7 | `[(T, EE)]` | Invalid end_control: EE (double E) |

---------

### 2.9 Transaction State Violations

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.9.1 | `[(T, TC), (R, RE)]` | Transaction closed (after commit), but next row is continuation instead of new transaction |
| 2.9.2 | `[(T, R0), (R, RE)]` | Transaction closed (after rollback), but next row is continuation |
| 2.9.3 | `[(T, TC), (R, RE), (R, TC)]` | Transaction closed (after commit), continuation, then another commit |

---------

### 2.10 Logical Inconsistencies

| Case ID | Sequence | Description |
|---------|----------|-------------|
| 2.10.1 | `[(T, S0)]` | Rollback to savepoint 0 with savepoint on rollback row (S0) - valid but unusual (note: actually valid per spec) |
| 2.10.2 | `[(T, RE), (R, S1)]` | Rollback to savepoint 1 when savepoint 1 is the rollback row itself (S1) - valid (savepoint created on rollback row) |

---------

## Notes

- All examples assume the transaction state machine is properly maintained
- Checksum rows (C/CS) are ignored for transaction state tracking
- Savepoint numbering is sequential within a transaction (first S = 1, second S = 2, etc.)
- Savepoint 0 always represents the transaction start (full rollback)
- Maximum transaction size: 100 data rows
- Maximum savepoints per transaction: 9 (numbered 1-9)
