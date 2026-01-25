# Finder Conformance

Testable scenarios and expected outcomes for Finder implementations. A conformant finder must pass every scenario. Terms (index, row types, totalRows, etc.) are defined in [finder_protocol.md](finder_protocol.md) and [v1_file_format.md](v1_file_format.md). However, this conformance tester is not necessarily comprehensive, and the exact definition of a conformant Finder is defined by the Finder protocol specification.

**ID pattern:** `FC-{method}-{seq}` — FC = Finder Conformance; method = GI (GetIndex), GTS (GetTransactionStart), GTE (GetTransactionEnd), ORA (OnRowAdded); seq = 1-based number.

---

## 1. GetIndex(key)

| ID | Setup | Operation | Expected |
|----|-------|-----------|----------|
| FC-GI-001 | Any | `GetIndex(uuid.Nil)` | `InvalidInputError` |
| FC-GI-002 | Any | `GetIndex(uuid)` with version ≠ 7 (e.g. v4) | `InvalidInputError` |
| FC-GI-003 | Any DB; key K not present in any DataRow | `GetIndex(K)` | `KeyNotFoundError` |
| FC-GI-004 | 0=checksum only (no DataRows) | `GetIndex(valid UUIDv7)` | `KeyNotFoundError` |
| FC-GI-005 | 0=checksum, 1..K=NullRows, no DataRows | `GetIndex(valid UUIDv7)` | `KeyNotFoundError` |
| FC-GI-006 | 0=checksum, 1=DataRow with key K | `GetIndex(K)` | `1` |
| FC-GI-007 | 0=checksum, 1..10000=DataRows, 10001=checksum; key at 10000 | `GetIndex(key at 10000)` | `10000` |
| FC-GI-008 | 0=checksum, 1..10000=DataRows, 10001=checksum, 10002=DataRow with key K | `GetIndex(K)` | `10002` |
| FC-GI-009 | 0=checksum, 1..10000=DataRows; key at 5000 | `GetIndex(key at 5000)` | `5000` |
| FC-GI-010 | 0, 10001=checksums; 10002..20001=DataRows; key at 15000 | `GetIndex(key at 15000)` | `15000` |
| FC-GI-011 | 0=checksum, 1=DataRow K1, 2=NullRow, 3=DataRow K2 | `GetIndex(K1)` | `1` |
| FC-GI-012 | 0=checksum, 1=DataRow K1, 2=NullRow, 3=DataRow K2 | `GetIndex(K2)` | `3` |
| FC-GI-013 | 0=checksum, 1..N=DataRows (last has K), then PartialDataRow | `GetIndex(K)` | `N` |
| FC-GI-014 | 0=checksum, 1..N=DataRows, then PartialDataRow with key K in payload (state 2/3) | `GetIndex(K)` | `KeyNotFoundError` |
| FC-GI-015 | DataRow with key K and end_control TC or SC | `GetIndex(K)` | that DataRow’s index |
| FC-GI-016 | DataRow with key K in a tx that ended with R0–R9 or S0–S9 | `GetIndex(K)` | that DataRow’s index |
| FC-GI-017 | Finder in write mode; row with K written; `OnRowAdded` for that row not yet called | `GetIndex(K)` | `KeyNotFoundError` |
| FC-GI-018 | `OnRowAdded(i, RowUnion{DataRow with key K})` has completed | `GetIndex(K)` | `i` |
| FC-GI-019 | DB with a corrupt or unreadable row in the scan path for key K | `GetIndex(K)` | `CorruptDatabaseError` or `ReadError` |

### 1.2 Key ordering (binary-search stress)

Notation: `ts [a,b,c]` = in file order, DataRows have keys with timestamps a,b,c. Skew 5. UUID comparison order matches numeric order (a&lt;b ⇒ K_a &lt; K_b). These target finders that binary-search by key: the file is not sorted by key. Additional patterns: **clusters** (keys in bands e.g. 10–15 vs 100–105); **gaps** (KeyNotFound for ts between clusters); **one band** (many rows in [100..114]); **more than one tx in one band** (e.g. 50 rows in [100..150]); **cluster after checksum** (second cluster at 10002+). Value order can disagree with file order. Typical bugs: (1) assuming min/max at file start/end, (2) “go left/right in value” treated as “go left/right in file”, (3) excluding the correct file index when the probe lies in a different value-order position, (4) wrong handling at cluster boundaries or in gaps.

| ID | Setup | Operation | Expected |
|----|-------|-----------|----------|
| FC-GI-020 | 0=checksum, 1..3=DataRows with keys ts [1,2,3] | `GetIndex(ts 2)` | `2` |
| FC-GI-021 | 0=checksum, 1..2=DataRows with keys ts [3,1] | `GetIndex(ts 1)` | `2` |
| FC-GI-022 | 0=checksum, 1..2=DataRows with keys ts [3,1] | `GetIndex(ts 3)` | `1` |
| FC-GI-023 | 0=checksum, 1..3=DataRows with keys ts [5,2,8] | `GetIndex(ts 2)` | `2` |
| FC-GI-024 | 0=checksum, 1..3=DataRows with keys ts [1,5,3] | `GetIndex(ts 3)` | `3` |
| FC-GI-025 | 0=checksum, 1..3=DataRows with keys ts [6,2,5] | `GetIndex(ts 5)` | `3` |
| FC-GI-026 | 0=checksum, 1..3=DataRows with keys ts [5,8,2] | `GetIndex(ts 2)` | `3` |
| FC-GI-027 | 0=checksum, 1..3=DataRows with keys ts [8,2,5] | `GetIndex(ts 8)` | `1` |
| FC-GI-028 | 0=checksum, 1..3=DataRows with keys ts [6,2,8] | `GetIndex(ts 6)` | `1` |
| FC-GI-029 | 0=checksum, 1..9999=DataRows, 10000/10002/10003 with keys ts [5,2,8], 10001=checksum | `GetIndex(ts 2)` | `10002` |
| FC-GI-030 | 0=checksum, 1..5=ts [10,12,11,14,13], 6..10=ts [100,102,101,104,103] (two clusters) | `GetIndex(ts 11)` | `3` |
| FC-GI-031 | Same as FC-GI-030 | `GetIndex(ts 101)` | `8` |
| FC-GI-032 | Same as FC-GI-030 | `GetIndex(ts 50)` | `KeyNotFoundError` |
| FC-GI-033 | 0=checksum, 1..15=ts [100,110,101,109,102,108,103,107,104,106,105,114,113,112,111] (one band) | `GetIndex(ts 105)` | `11` |
| FC-GI-034 | Same as FC-GI-033 | `GetIndex(ts 100)` | `1` |
| FC-GI-035 | Same as FC-GI-033 | `GetIndex(ts 114)` | `12` |
| FC-GI-036 | 0=checksum, 1..3=ts [20,10,30], 4..6=ts [200,210,201] (two clusters, OOO in each) | `GetIndex(ts 201)` | `6` |
| FC-GI-037 | Same as FC-GI-036 | `GetIndex(ts 50)` | `KeyNotFoundError` |
| FC-GI-038 | 0=checksum, 1..6=ts [50,52,51,54,53,55] (one tx in one skew band [50..55]) | `GetIndex(ts 53)` | `5` |
| FC-GI-039 | 0=checksum, 1..50=ts [100..150] OOO: 125 at 50, 150 at 2 (more than one tx in one band) | `GetIndex(ts 125)` | `50` |
| FC-GI-040 | Same as FC-GI-039 | `GetIndex(ts 150)` | `2` |
| FC-GI-041 | 0=checksum, 1..9999=ts in [10..99], 10001=checksum, 10002..10005=ts [1000,1002,1001,1003] (cluster after checksum) | `GetIndex(ts 1001)` | `10004` |
| FC-GI-042 | 0=checksum, 1..20=ts [100,110,101,109,102,108,103,107,104,106,105,115,114,116,113,117,112,118,111,119] | `GetIndex(ts 119)` | `20` |

---

## 2. GetTransactionStart(index)

| ID | Setup | Operation | Expected |
|----|-------|-----------|----------|
| FC-GTS-001 | Any | `GetTransactionStart(-1)` | `InvalidInputError` |
| FC-GTS-002 | 0=checksum, 1,2=DataRows (totalRows=3) | `GetTransactionStart(3)` | `InvalidInputError` |
| FC-GTS-003 | 0=checksum, 1,2=DataRows, then PartialDataRow (totalRows=3) | `GetTransactionStart(3)` | `InvalidInputError` |
| FC-GTS-004 | Any | `GetTransactionStart(0)` | `InvalidInputError` |
| FC-GTS-005 | 0=checksum, 1..10000=DataRows, 10001=checksum | `GetTransactionStart(10001)` | `InvalidInputError` |
| FC-GTS-006 | DataRow with start_control T at index i | `GetTransactionStart(i)` | `i` |
| FC-GTS-007 | 0=checksum, 1=T…RE, 2=R…RE, 3=R…TC | `GetTransactionStart(2)` | `1` |
| FC-GTS-008 | 0=checksum, 1=T…RE, 2=R…RE, 3=R…TC | `GetTransactionStart(3)` | `1` |
| FC-GTS-009 | NullRow at i | `GetTransactionStart(i)` | `i` |
| FC-GTS-010 | 0=checksum, 1..9999=DataRows, 10000=T…RE, 10001=checksum, 10002=R… (same tx) | `GetTransactionStart(10002)` | `10000` |
| FC-GTS-011 | 0=checksum, 1..10000=DataRows, 10001=checksum, 10002=T… | `GetTransactionStart(10002)` | `10002` |
| FC-GTS-012 | Corrupt/malformed rows so no start_control T exists in backward scan from given index | `GetTransactionStart(index in that region)` | `CorruptDatabaseError` |

---

## 3. GetTransactionEnd(index)

| ID | Setup | Operation | Expected |
|----|-------|-----------|----------|
| FC-GTE-001 | Any | `GetTransactionEnd(-1)` | `InvalidInputError` |
| FC-GTE-002 | 0=checksum, 1,2=DataRows (totalRows=3) | `GetTransactionEnd(3)` | `InvalidInputError` |
| FC-GTE-003 | 0=checksum, 1,2=DataRows, then PartialDataRow (totalRows=3) | `GetTransactionEnd(3)` | `InvalidInputError` |
| FC-GTE-004 | Any | `GetTransactionEnd(0)` | `InvalidInputError` |
| FC-GTE-005 | 0=checksum, 1..10000=DataRows, 10001=checksum | `GetTransactionEnd(10001)` | `InvalidInputError` |
| FC-GTE-006 | DataRow or NullRow with transaction-ending end_control at i (TC, SC, R0–R9, S0–S9, NR) | `GetTransactionEnd(i)` | `i` |
| FC-GTE-007 | 0=checksum, 1=T…RE, 2=R…RE, 3=R…TC | `GetTransactionEnd(1)` | `3` |
| FC-GTE-008 | 0=checksum, 1=T…RE, 2=R…RE, 3=R…TC | `GetTransactionEnd(2)` | `3` |
| FC-GTE-009 | NullRow at i | `GetTransactionEnd(i)` | `i` |
| FC-GTE-010 | 0=checksum, 1..9999=DataRows, 10000=T…RE, 10001=checksum, 10002=R…RE, 10003=R…TC | `GetTransactionEnd(10000)` | `10003` |
| FC-GTE-011 | 0=checksum, 1..9999=DataRows, 10000=T…RE, 10001=checksum, 10002=R…RE, 10003=R…TC | `GetTransactionEnd(10002)` | `10003` |
| FC-GTE-012 | 0=checksum, 1..N=DataRows, N has RE or SE; PartialDataRow after (no further complete row). totalRows = N+1. | `GetTransactionEnd(i)` for any i in that tx (e.g. 1 or N) | `TransactionActiveError` |

---

## 4. OnRowAdded(index, row)

OnRowAdded is only called for complete rows (DataRow, NullRow, ChecksumRow).

| ID | Setup | Operation | Expected |
|----|-------|-----------|----------|
| FC-ORA-001 | Any | `OnRowAdded(i, nil)` | `InvalidInputError` |
| FC-ORA-002 | Finder with 2 complete rows (expected next index 2) | `OnRowAdded(1, valid RowUnion)` | `InvalidInputError` |
| FC-ORA-003 | Finder with 2 complete rows (expected next index 2) | `OnRowAdded(4, valid RowUnion)` | `InvalidInputError` |
| FC-ORA-004 | Finder with expected next index i; RowUnion with DataRow K | `OnRowAdded(i, row)` | nil; then `GetIndex(K)` → `i` |
| FC-ORA-005 | Finder with expected next index = checksum index | `OnRowAdded(checksumIndex, RowUnion{ChecksumRow})` | nil; then `GetTransactionStart(checksumIndex)` → `InvalidInputError`, `GetTransactionEnd(checksumIndex)` → `InvalidInputError` |
| FC-ORA-006 | Finder with expected next index i | `OnRowAdded(i, RowUnion{NullRow})` | nil; then `GetTransactionStart(i)` → `i`, `GetTransactionEnd(i)` → `i` |

---

## 5. Error Types (for Expected column)

| Error | Used in |
|-------|---------|
| `InvalidInputError` | FC-GI-001, FC-GI-002; FC-GTS-001–005; FC-GTE-001–005; FC-ORA-001–003 |
| `KeyNotFoundError` | FC-GI-003–005, FC-GI-014, FC-GI-017, FC-GI-032, FC-GI-037 |
| `CorruptDatabaseError` | FC-GI-019; FC-GTS-012 |
| `TransactionActiveError` | FC-GTE-012 |
| `ReadError` | FC-GI-019 |

---

## 6. Conformance Checklist

- [ ] GetIndex: FC-GI-001–042
- [ ] GetTransactionStart: FC-GTS-001–012
- [ ] GetTransactionEnd: FC-GTE-001–012
- [ ] OnRowAdded: FC-ORA-001–006
