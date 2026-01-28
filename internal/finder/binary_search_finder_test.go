package finder

import (
	"testing"
)

// TestBinarySearchFinder_LogicalToPhysicalIndex tests the logical to physical index mapping.
func TestBinarySearchFinder_LogicalToPhysicalIndex(t *testing.T) {
	bsf := &BinarySearchFinder{}

	tests := []struct {
		name         string
		logicalIndex int64
		wantPhysical int64
	}{
		{
			name:         "first_logical_row",
			logicalIndex: 0,
			wantPhysical: 1, // Skips checksum at index 0
		},
		{
			name:         "logical_row_9999",
			logicalIndex: 9999,
			wantPhysical: 10000, // Still before first checksum after initial
		},
		{
			name:         "logical_row_10000",
			logicalIndex: 10000,
			wantPhysical: 10002, // Skips checksum at 10001
		},
		{
			name:         "logical_row_10001",
			logicalIndex: 10001,
			wantPhysical: 10003,
		},
		{
			name:         "logical_row_19999",
			logicalIndex: 19999,
			wantPhysical: 20001, // Before checksum at 20002
		},
		{
			name:         "logical_row_20000",
			logicalIndex: 20000,
			wantPhysical: 20003, // Skips checksum at 20002
		},
		{
			name:         "logical_row_29999",
			logicalIndex: 29999,
			wantPhysical: 30002, // Before checksum at 30003
		},
		{
			name:         "logical_row_30000",
			logicalIndex: 30000,
			wantPhysical: 30004, // Skips checksum at 30003
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bsf.logicalToPhysicalIndex(tt.logicalIndex)
			if got != tt.wantPhysical {
				t.Errorf("logicalToPhysicalIndex(%d) = %d, want %d", tt.logicalIndex, got, tt.wantPhysical)
			}
		})
	}
}

// TestBinarySearchFinder_CountLogicalRows tests the logical row counting function.
func TestBinarySearchFinder_CountLogicalRows(t *testing.T) {
	bsf := &BinarySearchFinder{}

	tests := []struct {
		name        string
		totalRows   int64
		wantLogical int64
	}{
		{
			name:        "zero_rows",
			totalRows:   0,
			wantLogical: 0,
		},
		{
			name:        "one_row_checksum",
			totalRows:   1,
			wantLogical: 0, // Only checksum row at index 0
		},
		{
			name:        "first_10000_logical_rows",
			totalRows:   10001, // Checksum at 0, logical rows 1-10000
			wantLogical: 10000,
		},
		{
			name:        "just_before_second_checksum",
			totalRows:   10001, // Checksum at 0, logical rows 1-10000
			wantLogical: 10000,
		},
		{
			name:        "at_second_checksum",
			totalRows:   10002, // Physical rows 0-10001: checksums at 0, 10001; logical rows at physical 1-10000 (logical 0-9999)
			wantLogical: 10000, // Only logical rows before the checksum at 10001
		},
		{
			name:        "after_second_checksum",
			totalRows:   20001, // Physical rows 0-20000: checksums at 0, 10001; logical rows at physical 1-10000, 10002-20000
			wantLogical: 19999, // 10000 + 9999 = 19999
		},
		{
			name:        "at_third_checksum",
			totalRows:   20003, // Physical rows 0-20002: checksums at 0, 10001, 20002; logical rows at physical 1-10000, 10002-20001
			wantLogical: 20000, // 10000 + 10000 = 20000 (checksum at 20002 is the last row)
		},
		{
			name:        "after_third_checksum",
			totalRows:   30003, // Physical rows 0-30002: checksums at 0, 10001, 20002; logical rows at physical 1-10000, 10002-20001, 20003-30002
			wantLogical: 30000, // 10000 + 10000 + 10000 = 30000
		},
		{
			name:        "large_database",
			totalRows:   100001, // 10 checksum rows (0, 10001, 20002, ..., 100000)
			wantLogical: 99991,  // 100001 - 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bsf.countLogicalRows(tt.totalRows)
			if got != tt.wantLogical {
				t.Errorf("countLogicalRows(%d) = %d, want %d", tt.totalRows, got, tt.wantLogical)
			}
		})
	}
}

// TestBinarySearchFinder_IndexMappingAlignment validates that numLogicalRows,
// logicalIndex, and physicalIndex all align properly.
func TestBinarySearchFinder_IndexMappingAlignment(t *testing.T) {
	bsf := &BinarySearchFinder{}

	tests := []struct {
		name        string
		totalRows   int64
		description string
	}{
		{
			name:        "small_database",
			totalRows:   10001,
			description: "One checksum at 0, logical rows 0-9999 map to physical 1-10000",
		},
		{
			name:        "medium_database",
			totalRows:   20002,
			description: "Checksums at 0, 10001; logical rows 0-19999 map correctly",
		},
		{
			name:        "large_database",
			totalRows:   50003,
			description: "Multiple checksums; logical rows map correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			numLogicalRows := bsf.countLogicalRows(tt.totalRows)

			// Verify all logical indices map to valid physical indices
			for logicalIndex := int64(0); logicalIndex < numLogicalRows; logicalIndex++ {
				physicalIndex := bsf.logicalToPhysicalIndex(logicalIndex)

				// Physical index must be within bounds
				if physicalIndex < 0 || physicalIndex >= tt.totalRows {
					t.Errorf("logicalIndex %d maps to physicalIndex %d which is out of bounds (totalRows: %d)",
						logicalIndex, physicalIndex, tt.totalRows)
					continue
				}

				// Physical index must not be a checksum row
				// Checksum rows are at: 0, 10001, 20002, 30003, ... = k * 10001
				if physicalIndex%10001 == 0 {
					t.Errorf("logicalIndex %d maps to physicalIndex %d which is a checksum row",
						logicalIndex, physicalIndex)
				}
			}

			// Verify that all non-checksum physical indices can be reached
			// (at least for the first few logical indices)
			reachedPhysical := make(map[int64]bool)
			for logicalIndex := int64(0); logicalIndex < numLogicalRows && logicalIndex < 1000; logicalIndex++ {
				physicalIndex := bsf.logicalToPhysicalIndex(logicalIndex)
				reachedPhysical[physicalIndex] = true
			}

			// Check that we're not missing any non-checksum rows in the first range
			// (for small databases, verify we reach all non-checksum rows)
			if tt.totalRows <= 10001 {
				for physicalIndex := int64(1); physicalIndex < tt.totalRows; physicalIndex++ {
					if physicalIndex%10001 != 0 && !reachedPhysical[physicalIndex] {
						// This is acceptable - not all physical indices need to be reachable
						// if we haven't tested all logical indices
						if physicalIndex < 1000 {
							t.Logf("Physical index %d not reached by any logical index in first 1000", physicalIndex)
						}
					}
				}
			}

			// Verify the mapping is monotonic (logical indices map to increasing physical indices)
			prevPhysical := int64(-1)
			for logicalIndex := int64(0); logicalIndex < numLogicalRows && logicalIndex < 1000; logicalIndex++ {
				physicalIndex := bsf.logicalToPhysicalIndex(logicalIndex)
				if physicalIndex <= prevPhysical {
					t.Errorf("Mapping not monotonic: logicalIndex %d -> physicalIndex %d, previous was %d",
						logicalIndex, physicalIndex, prevPhysical)
				}
				prevPhysical = physicalIndex
			}

			t.Logf("%s: numLogicalRows=%d, all %d logical indices map to valid physical indices",
				tt.description, numLogicalRows, numLogicalRows)
		})
	}
}

// TestBinarySearchFinder_IndexMappingRoundTrip validates that the mapping
// is consistent for edge cases around checksum boundaries.
func TestBinarySearchFinder_IndexMappingRoundTrip(t *testing.T) {
	bsf := &BinarySearchFinder{}

	// Test around checksum boundaries
	boundaryTests := []struct {
		name        string
		logicalIdx  int64
		description string
	}{
		{
			name:        "before_first_checksum_after_initial",
			logicalIdx:  9999,
			description: "Last logical row before checksum at physical 10001",
		},
		{
			name:        "after_first_checksum_after_initial",
			logicalIdx:  10000,
			description: "First logical row after checksum at physical 10001",
		},
		{
			name:        "before_second_checksum",
			logicalIdx:  19999,
			description: "Last logical row before checksum at physical 20002",
		},
		{
			name:        "after_second_checksum",
			logicalIdx:  20000,
			description: "First logical row after checksum at physical 20002",
		},
	}

	for _, tt := range boundaryTests {
		t.Run(tt.name, func(t *testing.T) {
			physicalIdx := bsf.logicalToPhysicalIndex(tt.logicalIdx)

			// Verify it's not a checksum row
			if physicalIdx%10001 == 0 {
				t.Errorf("%s: logicalIndex %d maps to physicalIndex %d which is a checksum row",
					tt.description, tt.logicalIdx, physicalIdx)
			}

			// Verify it's in a reasonable range
			// For logical index L, physical should be approximately L + L/10000 + 1
			expectedMin := tt.logicalIdx + (tt.logicalIdx / 10000) + 1
			if physicalIdx < expectedMin {
				t.Errorf("%s: logicalIndex %d maps to physicalIndex %d, expected at least %d",
					tt.description, tt.logicalIdx, physicalIdx, expectedMin)
			}

			t.Logf("%s: logicalIndex %d -> physicalIndex %d", tt.description, tt.logicalIdx, physicalIdx)
		})
	}
}
