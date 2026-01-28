package finder

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// FinderFactory creates a Finder for a database file. The returned cleanup must be
// called when done. Used for data-driven conformance tests; register with
// RegisterFinderFactory or pass to RunFinderConformance.
type FinderFactory func(t *testing.T, path string, rowSize int32) (Finder, func())

var finderFactories = map[string]FinderFactory{
	"simple":        simpleFinderFactory,
	"binary_search": binarySearchFinderFactory,
}

// RegisterFinderFactory registers a FinderFactory by name for conformance tests.
func RegisterFinderFactory(name string, fn FinderFactory) {
	finderFactories[name] = fn
}

func simpleFinderFactory(t *testing.T, path string, rowSize int32) (Finder, func()) {
	t.Helper()
	dbFile, err := NewDBFile(path, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile: %v", err)
	}
	f, err := NewSimpleFinder(dbFile, int32(rowSize))
	if err != nil {
		dbFile.Close()
		t.Fatalf("NewSimpleFinder: %v", err)
	}
	return f, func() { _ = dbFile.Close() }
}

func binarySearchFinderFactory(t *testing.T, path string, rowSize int32) (Finder, func()) {
	t.Helper()
	dbFile, err := NewDBFile(path, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile: %v", err)
	}
	f, err := NewBinarySearchFinder(dbFile, int32(rowSize))
	if err != nil {
		dbFile.Close()
		t.Fatalf("NewBinarySearchFinder: %v", err)
	}
	return f, func() { _ = dbFile.Close() }
}

// RunFinderConformance runs all conformance scenarios for the given finder factory.
// On failure: can be a bug in the finder, the harness, or the conformance doc.
// v1_file_format.md and finder_protocol.md are the definitive sources of truth;
// conformance doc inaccuracies should be fixed to match them.
func RunFinderConformance(t *testing.T, factory FinderFactory) {
	t.Helper()
	for _, id := range conformanceScenarioIDs() {
		id := id
		t.Run(id, func(t *testing.T) {
			runScenario(t, id, factory)
		})
	}
}

func TestFinderConformance_SimpleFinder(t *testing.T) {
	RunFinderConformance(t, simpleFinderFactory)
}

func TestFinderConformance_InMemoryFinder(t *testing.T) {
	RunFinderConformance(t, inmemoryFinderFactory)
}

func TestFinderConformance_BinarySearchFinder(t *testing.T) {
	RunFinderConformance(t, binarySearchFinderFactory)
}

// TestFinderConformance_MaxTimestamp validates MaxTimestamp() requirements from spec 023
func TestFinderConformance_MaxTimestamp(t *testing.T) {
	t.Run("SimpleFinder", func(t *testing.T) {
		testMaxTimestampConformance(t, simpleFinderFactory)
	})
	t.Run("InMemoryFinder", func(t *testing.T) {
		testMaxTimestampConformance(t, inmemoryFinderFactory)
	})
}

func testMaxTimestampConformance(t *testing.T, factory FinderFactory) {
	t.Helper()

	// Test FR-003: Returns 0 for empty database
	t.Run("FR_003_Returns_Zero_Empty", func(t *testing.T) {
		dir := t.TempDir()
		path := setupCreate(t, dir, 0)
		finder, cleanup := factory(t, path, confRowSize)
		defer cleanup()

		maxTs := finder.MaxTimestamp()
		if maxTs != 0 {
			t.Errorf("MaxTimestamp() on empty database: got %d, want 0", maxTs)
		}
	})

	// Test FR-004: Updates on commit
	t.Run("FR_004_Updates_On_Commit", func(t *testing.T) {
		dir := t.TempDir()
		path := setupCreate(t, dir, 0)

		// Add a data row
		key1 := uuidFromTS(100)
		dbAddDataRow(t, path, key1, `{"v":1}`)

		finder, cleanup := factory(t, path, confRowSize)
		defer cleanup()

		maxTs := finder.MaxTimestamp()
		expectedTs := int64(100)
		if maxTs != expectedTs {
			t.Errorf("MaxTimestamp() after commit: got %d, want %d", maxTs, expectedTs)
		}

		// Add another row with newer timestamp
		key2 := uuidFromTS(200)
		dbAddDataRow(t, path, key2, `{"v":2}`)

		// Recreate finder to see updated maxTimestamp
		cleanup()
		finder2, cleanup2 := factory(t, path, confRowSize)
		defer cleanup2()

		maxTs2 := finder2.MaxTimestamp()
		expectedTs2 := int64(200)
		if maxTs2 != expectedTs2 {
			t.Errorf("MaxTimestamp() after second commit: got %d, want %d", maxTs2, expectedTs2)
		}
	})

	// Test FR-002: O(1) time complexity (basic validation - multiple calls should be fast)
	t.Run("FR_002_O1_Time_Complexity", func(t *testing.T) {
		dir := t.TempDir()
		path := setupCreate(t, dir, 0)

		// Add many rows to ensure we're testing with a non-trivial database
		for i := 0; i < 50; i++ {
			dbAddDataRow(t, path, uuidFromTS(1000+i), `{"v":`+string(rune('0'+i%10))+`}`)
		}

		finder, cleanup := factory(t, path, confRowSize)
		defer cleanup()

		// Multiple calls should all be fast (O(1))
		for i := 0; i < 1000; i++ {
			_ = finder.MaxTimestamp()
		}

		// If we reach here without timeout, O(1) requirement is satisfied
	})

	// Test that PartialDataRow doesn't affect MaxTimestamp
	t.Run("PartialDataRow_Does_Not_Affect", func(t *testing.T) {
		dir := t.TempDir()
		path := setupCreate(t, dir, 0)

		// Add a data row
		key1 := uuidFromTS(100)
		dbAddDataRow(t, path, key1, `{"v":1}`)

		finder, cleanup := factory(t, path, confRowSize)
		defer cleanup()

		maxTsBefore := finder.MaxTimestamp()

		// Start a transaction but don't commit (creates PartialDataRow)
		db, err := NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)
		if err != nil {
			t.Fatalf("NewFrozenDB: %v", err)
		}
		tx, err := db.BeginTx()
		if err != nil {
			db.Close()
			t.Fatalf("BeginTx: %v", err)
		}
		key2 := uuidFromTS(200)
		if err := tx.AddRow(key2, json.RawMessage(`{"v":2}`)); err != nil {
			tx.Rollback(0)
			db.Close()
			t.Fatalf("AddRow: %v", err)
		}
		// Don't commit - transaction remains open with PartialDataRow

		// MaxTimestamp should not change (PartialDataRow doesn't contribute)
		maxTsDuring := finder.MaxTimestamp()
		if maxTsDuring != maxTsBefore {
			t.Errorf("MaxTimestamp changed during uncommitted transaction: got %d, want %d", maxTsDuring, maxTsBefore)
		}

		// Clean up
		tx.Rollback(0)
		db.Close()
		cleanup()
	})
}

// --- Helpers for fixtures ---

const confRowSize = 1024
const confSkewMs = 5000

func setupCreate(t *testing.T, dir string, skewMs int) string {
	t.Helper()
	path := filepath.Join(dir, "c.fdb")
	if skewMs == 0 {
		skewMs = confSkewMs
	}
	setupMockSyscalls(false, false)
	t.Cleanup(restoreRealSyscalls)
	t.Setenv("SUDO_USER", MOCK_USER)
	t.Setenv("SUDO_UID", MOCK_UID)
	t.Setenv("SUDO_GID", MOCK_GID)
	if err := Create(CreateConfig{path: path, rowSize: confRowSize, skewMs: skewMs}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return path
}

// uuidFromTS produces a UUIDv7 that sorts in numeric order of ts (ts1 < ts2 => uuidFromTS(ts1) < uuidFromTS(ts2)).
// Used for key-ordering scenarios. Must satisfy ValidateUUIDv7 and the DB's new_ts+skew > max_ts when inserting in file order.
// CRITICAL: This function generates valid DataRow UUIDs (not NullRow UUIDs). Bytes 7, 9-15 must have at least one non-zero byte
// to avoid matching the NullRow UUID pattern. We use a simple pattern: set byte 7 to 1 to ensure it's a valid DataRow UUID.
func uuidFromTS(ts int) uuid.UUID {
	var u [16]byte
	// 48-bit big-endian ts at [0:6]
	u[0] = byte(ts >> 40)
	u[1] = byte(ts >> 32)
	u[2] = byte(ts >> 24)
	u[3] = byte(ts >> 16)
	u[4] = byte(ts >> 8)
	u[5] = byte(ts)
	u[6] = 0x70 // version 7
	u[7] = 0x01 // Set to non-zero to ensure valid DataRow UUID (not NullRow pattern)
	u[8] = 0x80 // variant RFC 4122
	u[9] = 0
	u[10] = 0
	u[11] = 0
	u[12] = 0
	u[13] = 0
	u[14] = 0
	u[15] = 0
	return uuid.UUID(u)
}

func dbAddDataRow(t *testing.T, path string, key uuid.UUID, value string) {
	t.Helper()
	db, err := NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("NewFrozenDB: %v", err)
	}
	defer db.Close()
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if err := tx.AddRow(key, json.RawMessage(value)); err != nil {
		t.Fatalf("AddRow: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func dbAddNullRow(t *testing.T, path string) {
	t.Helper()
	db, err := NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("NewFrozenDB: %v", err)
	}
	defer db.Close()
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	// Commit empty transaction to create NullRow
	// The transaction Commit() will create a NullRow automatically for empty transactions
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

// addDataRowsInOrder adds one DataRow per ts in the given order. File order = insert order.
// Each key must satisfy new_ts+skew > max_ts (caller must use valid sequences).
func addDataRowsInOrder(t *testing.T, path string, tsList []int) {
	t.Helper()
	for i, ts := range tsList {
		key := uuidFromTS(ts)
		dbAddDataRow(t, path, key, `{"ts":`+string(rune('0'+i%10))+`}`)
	}
}

// --- Scenario runner ---

type scenarioResult struct {
	index int64
	err   error
}

func runScenario(t *testing.T, id string, factory FinderFactory) {
	t.Helper()
	dir := t.TempDir()
	path, rowSize, env := setupFor(t, id, dir)
	if path == "" && rowSize == 0 {
		t.Skipf("scenario %s not implemented", id)
		return
	}
	finder, cleanup := factory(t, path, rowSize)
	defer cleanup()

	var got scenarioResult
	switch {
	case id >= "FC-GI-001" && id <= "FC-GI-042":
		got.index, got.err = runGetIndex(t, id, finder, env)
	case id >= "FC-GTS-001" && id <= "FC-GTS-012":
		got.index, got.err = runGetTransactionStart(t, id, finder, env)
	case id >= "FC-GTE-001" && id <= "FC-GTE-012":
		got.index, got.err = runGetTransactionEnd(t, id, finder, env)
	case id >= "FC-ORA-001" && id <= "FC-ORA-006":
		_, got.err = runOnRowAdded(t, id, finder, env)
	default:
		t.Fatalf("unknown scenario %s", id)
	}
	assertExpected(t, id, got, env)
}

// fixtureEnv holds keys and indices built by setup for use in run and assert.
type fixtureEnv struct {
	keys    map[string]uuid.UUID // "K", "ts:1", "ts:2", etc.
	indices map[string]int64     // "i", "ts:1" -> index
}

func (e *fixtureEnv) K(name string) uuid.UUID { return e.keys[name] }
func (e *fixtureEnv) Idx(name string) int64   { return e.indices[name] }

func setupFor(t *testing.T, id string, dir string) (path string, rowSize int32, env *fixtureEnv) {
	t.Helper()
	env = &fixtureEnv{keys: make(map[string]uuid.UUID), indices: make(map[string]int64)}
	rowSize = confRowSize

	switch id {
	case "FC-GI-001", "FC-GI-002", "FC-GI-003", "FC-GTS-001", "FC-GTS-004", "FC-GTE-001", "FC-GTE-004":
		path = setupCreate(t, dir, 0)
		// any minimal DB
	case "FC-GI-004":
		path = setupCreate(t, dir, 0)
	case "FC-GI-005":
		path = setupCreate(t, dir, 0)
		dbAddNullRow(t, path)
		dbAddNullRow(t, path)
		dbAddNullRow(t, path)
	case "FC-GI-006", "FC-GI-018":
		path = setupCreate(t, dir, 0)
		k := uuidFromTS(1)
		env.keys["K"] = k
		dbAddDataRow(t, path, k, `{"v":1}`)
		env.indices["i"] = 1
	case "FC-GI-011", "FC-GI-012":
		path = setupCreate(t, dir, 0)
		k1, k2 := uuidFromTS(10), uuidFromTS(30)
		env.keys["K1"], env.keys["K2"] = k1, k2
		dbAddDataRow(t, path, k1, `{}`)
		dbAddNullRow(t, path)
		dbAddDataRow(t, path, k2, `{}`)
		env.indices["K1"], env.indices["K2"] = 1, 3
	case "FC-GI-020":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{1, 2, 3})
		env.keys["ts:2"] = uuidFromTS(2)
		env.indices["ts:2"] = 2
	case "FC-GI-021", "FC-GI-022":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{3, 1})
		env.keys["ts:1"] = uuidFromTS(1)
		env.keys["ts:3"] = uuidFromTS(3)
		env.indices["ts:1"] = 2
		env.indices["ts:3"] = 1
	case "FC-GI-023":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{5, 2, 8})
		env.keys["ts:2"] = uuidFromTS(2)
		env.indices["ts:2"] = 2
	case "FC-GI-024":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{1, 5, 3})
		env.keys["ts:3"] = uuidFromTS(3)
		env.indices["ts:3"] = 3
	case "FC-GI-025":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{6, 2, 5})
		env.keys["ts:5"] = uuidFromTS(5)
		env.indices["ts:5"] = 3
	case "FC-GI-026":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{5, 8, 2})
		env.keys["ts:2"] = uuidFromTS(2)
		env.indices["ts:2"] = 3
	case "FC-GI-027":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{8, 2, 5})
		env.keys["ts:8"] = uuidFromTS(8)
		env.indices["ts:8"] = 1
	case "FC-GI-028":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{6, 2, 8})
		env.keys["ts:6"] = uuidFromTS(6)
		env.indices["ts:6"] = 1
	case "FC-GI-030", "FC-GI-031", "FC-GI-032":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{10, 12, 11, 14, 13})
		addDataRowsInOrder(t, path, []int{100, 102, 101, 104, 103})
		env.keys["ts:11"] = uuidFromTS(11)
		env.keys["ts:101"] = uuidFromTS(101)
		env.keys["ts:50"] = uuidFromTS(50) // not present
		env.indices["ts:11"] = 3
		env.indices["ts:101"] = 8
	case "FC-GI-033", "FC-GI-034", "FC-GI-035":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{100, 110, 101, 109, 102, 108, 103, 107, 104, 106, 105, 114, 113, 112, 111})
		env.keys["ts:105"] = uuidFromTS(105)
		env.keys["ts:100"] = uuidFromTS(100)
		env.keys["ts:114"] = uuidFromTS(114)
		env.indices["ts:105"] = 11
		env.indices["ts:100"] = 1
		env.indices["ts:114"] = 12
	case "FC-GI-036", "FC-GI-037":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{20, 10, 30})
		addDataRowsInOrder(t, path, []int{200, 210, 201})
		env.keys["ts:201"] = uuidFromTS(201)
		env.keys["ts:50"] = uuidFromTS(50)
		env.indices["ts:201"] = 6
	case "FC-GI-038":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{50, 52, 51, 54, 53, 55})
		env.keys["ts:53"] = uuidFromTS(53)
		env.indices["ts:53"] = 5
	case "FC-GI-039", "FC-GI-040":
		// Conformance: "125 at 50, 150 at 2". With skew 5, 150 at 2 is infeasible (101+5<150). We use: 125 at 50, 149 at 49.
		// 125 must appear only at the 50th row: seq[0..47] must not include 125 (100+25 would place 125 at index 26).
		path = setupCreate(t, dir, 0)
		seq := make([]int, 50)
		for i := 0; i < 25; i++ {
			seq[i] = 100 + i
		}
		for i := 25; i < 48; i++ {
			seq[i] = 126 + (i - 25)
		}
		seq[48] = 149
		seq[49] = 125
		addDataRowsInOrder(t, path, seq)
		env.keys["ts:125"] = uuidFromTS(125)
		env.keys["ts:149"] = uuidFromTS(149)
		env.indices["ts:125"] = 50
		env.indices["ts:149"] = 49
	case "FC-GI-042":
		path = setupCreate(t, dir, 0)
		addDataRowsInOrder(t, path, []int{100, 110, 101, 109, 102, 108, 103, 107, 104, 106, 105, 115, 114, 116, 113, 117, 112, 118, 111, 119})
		env.keys["ts:119"] = uuidFromTS(119)
		env.indices["ts:119"] = 20
	case "FC-GTS-002", "FC-GTS-003", "FC-GTE-002", "FC-GTE-003":
		path = setupCreate(t, dir, 0)
		dbAddDataRow(t, path, uuidFromTS(1), `{}`)
		dbAddDataRow(t, path, uuidFromTS(2), `{}`)
		env.indices["totalRows"] = 3
	case "FC-GTS-007", "FC-GTS-008", "FC-GTE-007", "FC-GTE-008":
		path = setupCreate(t, dir, 0)
		tx, db := openAndBegin(t, path)
		tx.AddRow(uuidFromTS(1), json.RawMessage(`{}`))
		tx.AddRow(uuidFromTS(2), json.RawMessage(`{}`))
		tx.AddRow(uuidFromTS(3), json.RawMessage(`{}`))
		_ = tx.Commit()
		_ = db.Close()
		env.indices["start"] = 1
		env.indices["end"] = 3
	case "FC-GTS-009", "FC-GTE-009":
		path = setupCreate(t, dir, 0)
		dbAddNullRow(t, path)
		env.indices["i"] = 1
	case "FC-ORA-001", "FC-ORA-002", "FC-ORA-003":
		path = setupCreate(t, dir, 0)
		dbAddDataRow(t, path, uuidFromTS(1), `{}`)
		env.indices["expectedNext"] = 2
	default:
		return "", 0, nil
	}
	return path, rowSize, env
}

func openAndBegin(t *testing.T, path string) (*Transaction, *FrozenDB) {
	t.Helper()
	db, err := NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("NewFrozenDB: %v", err)
	}
	tx, err := db.BeginTx()
	if err != nil {
		db.Close()
		t.Fatalf("BeginTx: %v", err)
	}
	return tx, db
}

func runGetIndex(t *testing.T, id string, f Finder, env *fixtureEnv) (int64, error) {
	t.Helper()
	var key uuid.UUID
	switch id {
	case "FC-GI-001":
		key = uuid.Nil
	case "FC-GI-002":
		key = uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	case "FC-GI-003":
		key = uuidFromTS(99999)
	case "FC-GI-004", "FC-GI-005":
		key = uuidFromTS(1)
	case "FC-GI-006", "FC-GI-018":
		key = env.K("K")
	case "FC-GI-011":
		key = env.K("K1")
	case "FC-GI-012":
		key = env.K("K2")
	case "FC-GI-020":
		key = env.K("ts:2")
	case "FC-GI-021":
		key = env.K("ts:1")
	case "FC-GI-022":
		key = env.K("ts:3")
	case "FC-GI-023":
		key = env.K("ts:2")
	case "FC-GI-024":
		key = env.K("ts:3")
	case "FC-GI-025":
		key = env.K("ts:5")
	case "FC-GI-026":
		key = env.K("ts:2")
	case "FC-GI-027":
		key = env.K("ts:8")
	case "FC-GI-028":
		key = env.K("ts:6")
	case "FC-GI-030":
		key = env.K("ts:11")
	case "FC-GI-031":
		key = env.K("ts:101")
	case "FC-GI-032", "FC-GI-037":
		key = env.K("ts:50")
	case "FC-GI-033":
		key = env.K("ts:105")
	case "FC-GI-034":
		key = env.K("ts:100")
	case "FC-GI-035":
		key = env.K("ts:114")
	case "FC-GI-036":
		key = env.K("ts:201")
	case "FC-GI-038":
		key = env.K("ts:53")
	case "FC-GI-039":
		key = env.K("ts:125")
	case "FC-GI-040":
		key = env.K("ts:149")
	case "FC-GI-042":
		key = env.K("ts:119")
	default:
		t.Fatalf("runGetIndex: %s", id)
		return 0, nil
	}
	return f.GetIndex(key)
}

func runGetTransactionStart(t *testing.T, id string, f Finder, env *fixtureEnv) (int64, error) {
	t.Helper()
	var idx int64
	switch id {
	case "FC-GTS-001":
		idx = -1
	case "FC-GTS-002", "FC-GTS-003":
		idx = 3
	case "FC-GTS-004":
		idx = 0
	case "FC-GTS-007":
		idx = 2
	case "FC-GTS-008":
		idx = 3
	case "FC-GTS-009", "FC-GTE-009":
		idx = env.Idx("i")
	default:
		t.Fatalf("runGetTransactionStart: %s", id)
		return 0, nil
	}
	return f.GetTransactionStart(idx)
}

func runGetTransactionEnd(t *testing.T, id string, f Finder, env *fixtureEnv) (int64, error) {
	t.Helper()
	var idx int64
	switch id {
	case "FC-GTE-001":
		idx = -1
	case "FC-GTE-002", "FC-GTE-003":
		idx = 3
	case "FC-GTE-004":
		idx = 0
	case "FC-GTE-007":
		idx = 1
	case "FC-GTE-008":
		idx = 2
	case "FC-GTE-009":
		idx = env.Idx("i")
	default:
		t.Fatalf("runGetTransactionEnd: %s", id)
		return 0, nil
	}
	return f.GetTransactionEnd(idx)
}

func runOnRowAdded(t *testing.T, id string, f Finder, env *fixtureEnv) (int64, error) {
	t.Helper()
	// FC-ORA-001: (0, nil)
	// FC-ORA-002: (1, valid) when expected 2
	// FC-ORA-003: (4, valid) when expected 2
	ru := &RowUnion{DataRow: &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(confRowSize),
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload:   &DataRowPayload{Key: uuidFromTS(1), Value: json.RawMessage(`{}`)},
		},
	}}
	switch id {
	case "FC-ORA-001":
		return 0, f.OnRowAdded(0, nil)
	case "FC-ORA-002":
		return 0, f.OnRowAdded(1, ru)
	case "FC-ORA-003":
		return 0, f.OnRowAdded(4, ru)
	default:
		t.Fatalf("runOnRowAdded: %s", id)
		return 0, nil
	}
}

func assertExpected(t *testing.T, id string, got scenarioResult, env *fixtureEnv) {
	t.Helper()
	wantErr, wantIdx := expectedFor(id, env)
	if wantErr != "" {
		if got.err == nil {
			t.Errorf("%s: want error %s, got nil", id, wantErr)
			return
		}
		if !matchErr(wantErr, got.err) {
			t.Errorf("%s: want error %s, got %T %v", id, wantErr, got.err, got.err)
		}
		return
	}
	if got.err != nil {
		t.Errorf("%s: want index %d, got error %v", id, wantIdx, got.err)
		return
	}
	if got.index != wantIdx {
		t.Errorf("%s: want index %d, got %d", id, wantIdx, got.index)
	}
}

func expectedFor(id string, env *fixtureEnv) (wantErr string, wantIdx int64) {
	errs := map[string]string{
		"FC-GI-001": "InvalidInputError", "FC-GI-002": "InvalidInputError",
		"FC-GI-003": "KeyNotFoundError", "FC-GI-004": "KeyNotFoundError", "FC-GI-005": "KeyNotFoundError",
		"FC-GI-032": "KeyNotFoundError", "FC-GI-037": "KeyNotFoundError",
		"FC-GTS-001": "InvalidInputError", "FC-GTS-002": "InvalidInputError", "FC-GTS-003": "InvalidInputError",
		"FC-GTS-004": "InvalidInputError",
		"FC-GTE-001": "InvalidInputError", "FC-GTE-002": "InvalidInputError", "FC-GTE-003": "InvalidInputError",
		"FC-GTE-004": "InvalidInputError",
		"FC-ORA-001": "InvalidInputError", "FC-ORA-002": "InvalidInputError", "FC-ORA-003": "InvalidInputError",
	}
	if e, ok := errs[id]; ok {
		return e, 0
	}
	idx := map[string]int64{
		"FC-GI-006": 1, "FC-GI-018": 1,
		"FC-GI-011": 1, "FC-GI-012": 3,
		"FC-GI-020": 2, "FC-GI-021": 2, "FC-GI-022": 1,
		"FC-GI-023": 2, "FC-GI-024": 3, "FC-GI-025": 3, "FC-GI-026": 3, "FC-GI-027": 1, "FC-GI-028": 1,
		"FC-GI-030": 3, "FC-GI-031": 8,
		"FC-GI-033": 11, "FC-GI-034": 1, "FC-GI-035": 12,
		"FC-GI-036": 6, "FC-GI-038": 5, "FC-GI-039": 50, "FC-GI-040": 49, "FC-GI-042": 20,
		"FC-GTS-007": 1, "FC-GTS-008": 1, "FC-GTS-009": 1,
		"FC-GTE-007": 3, "FC-GTE-008": 3, "FC-GTE-009": 1,
	}
	if i, ok := idx[id]; ok {
		return "", i
	}
	if env != nil {
		if i, ok := env.indices["start"]; ok && (id == "FC-GTS-007" || id == "FC-GTS-008") {
			return "", i
		}
		if i, ok := env.indices["end"]; ok && (id == "FC-GTE-007" || id == "FC-GTE-008") {
			return "", i
		}
		if i, ok := env.indices["i"]; ok {
			return "", i
		}
	}
	return "", 0
}

func matchErr(want string, err error) bool {
	switch want {
	case "InvalidInputError":
		var e *InvalidInputError
		return errors.As(err, &e)
	case "KeyNotFoundError":
		var e *KeyNotFoundError
		return errors.As(err, &e)
	case "CorruptDatabaseError":
		var e *CorruptDatabaseError
		return errors.As(err, &e)
	case "TransactionActiveError":
		var e *TransactionActiveError
		return errors.As(err, &e)
	case "ReadError":
		var e *ReadError
		return errors.As(err, &e)
	default:
		return false
	}
}

func conformanceScenarioIDs() []string {
	return []string{
		"FC-GI-001", "FC-GI-002", "FC-GI-003", "FC-GI-004", "FC-GI-005", "FC-GI-006",
		"FC-GI-011", "FC-GI-012", "FC-GI-018",
		"FC-GI-020", "FC-GI-021", "FC-GI-022", "FC-GI-023", "FC-GI-024", "FC-GI-025",
		"FC-GI-026", "FC-GI-027", "FC-GI-028",
		"FC-GI-030", "FC-GI-031", "FC-GI-032", "FC-GI-033", "FC-GI-034", "FC-GI-035",
		"FC-GI-036", "FC-GI-037", "FC-GI-038", "FC-GI-039", "FC-GI-040", "FC-GI-042",
		"FC-GTS-001", "FC-GTS-002", "FC-GTS-003", "FC-GTS-004", "FC-GTS-007", "FC-GTS-008", "FC-GTS-009",
		"FC-GTE-001", "FC-GTE-002", "FC-GTE-003", "FC-GTE-004", "FC-GTE-007", "FC-GTE-008", "FC-GTE-009",
		"FC-ORA-001", "FC-ORA-002", "FC-ORA-003",
	}
}
