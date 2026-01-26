# Fuzzy Binary Search Research

## User Input Analysis: Core Algorithmic Reasoning

The user provided the fundamental insight that drives the FuzzyBinarySearch algorithm:

> "We know from the v1_file_format that when a key is inserted this must hold: new_timestamp + skew_ms > max_timestamp. Therefore, if the max_timestamp from any previous row is M, and skew_ms is 5, it follows that new_timestamp > M - 5. Thus, it follows that for any timestamp T, there cannot be any value greater than T + skew_ms preceding it. This is the primary axiom to define the algorithm. Given our target timestamp X, the lower bound to search for is X - skew_ms, and the upper bound to search for is X + skew_ms."

This axiom establishes the mathematical foundation for the algorithm:

1. **Primary Axiom**: No value greater than `T + skew_ms` can precede timestamp `T`
2. **Search Bounds**: For target `X`, valid range is `[X - skew_ms, X + skew_ms]`
3. **Three-Section Logic**: At any midpoint, we encounter three distinct sections:
   - **Clearly too low**: Values `< X - skew_ms` (must search right)
   - **Clearly too high**: Values `> X + skew_ms` (must search left)  
   - **Indeterminate**: Values within `[X - skew_ms, X + skew_ms]` (requires linear scan)

## Comparison with Standard Binary Search

### Standard Binary Search
- **Two-way decision**: Value is either `< target` or `≥ target`
- **Deterministic ordering**: Array is strictly sorted
- **Single comparison**: Each iteration definitively knows search direction
- **O(log n) complexity**: Pure logarithmic search

### Fuzzy Binary Search  
- **Three-way decision**: Value can be `< lower_bound`, `> upper_bound`, or in indeterminate range
- **Imprecise ordering**: Array may have out-of-order entries within skew window
- **Uncertain comparisons**: Some comparisons cannot determine search direction
- **O(log n) + k complexity**: Logarithmic search + linear scan of indeterminate range

### Key Algorithmic Differences

| Aspect | Standard Binary Search | Fuzzy Binary Search |
|--------|---------------------|-------------------|
| Array Properties | Strictly sorted | Mostly sorted with bounded disorder |
| Decision Points | 2 (left/right) | 3 (left/indeterminate/right) |
| Comparison Logic | `value < target ?` | `value < lower_bound ? value > upper_bound ?` |
| Termination | Exact match found or exhausted | Exact match found, exhausted, or linear scan complete |
| Space Usage | O(1) | O(1) |
| Time Complexity | O(log n) | O(log n) + k |

## Algorithm Decision: Three-Way Partitioned Binary Search

**Decision**: Implement a modified binary search with three-way partitioning that handles skew window efficiently.

**Rationale**: 
- Based on user's three-section analysis: clearly too low, clearly too high, and indeterminate middle range
- Traditional binary search has two decisions (left/right), but fuzzy binary search needs three decisions (left/indeterminate/right)
- The indeterminate range size `k` is bounded by entries within ±skew_ms (derived from primary axiom)
- Achieves required O(log n) + k complexity where k is number of entries within ±skew of target

### **Algorithm Steps:**
1. Calculate bounds: `lower_bound = target - skew_ms`, `upper_bound = target + skew_ms`
2. Perform modified binary search to find indeterminate range where values fall within bounds
3. Execute linear scan only within the indeterminate range
4. Return exact match or KeyNotFoundError

### **Three-Way Decision Logic:**
At each midpoint `mid` with value `V = get(mid)`:
- **If `V < lower_bound`**: Search right (`lo = mid + 1`) - clearly too low
- **If `V > upper_bound`**: Search left (`hi = mid - 1`) - clearly too high  
- **If `lower_bound ≤ V ≤ upper_bound`**: Enter linear scan phase - indeterminate range found

**Alternatives considered**:
- Linear scan: O(n) complexity, doesn't meet performance requirements
- Pre-processing with additional data structures: Increases space complexity beyond O(1)
- Standard binary search with tolerance: Cannot handle out-of-order entries correctly

## Interface Design: Function-Based with Callback

**Decision**: Use callback function `get(index int64) (int64, error)` for timestamp access.

**Rationale**:
- Matches existing frozenDB patterns from SimpleFinder implementation
- O(1) space complexity - no pre-loading or caching required
- Thread-safe with immutable underlying data
- Proper error propagation through callback interface
- Data-structure agnostic as required by specification

**Alternatives considered**:
- Generic interface with type parameter: Adds unnecessary complexity for this specific use case
- Direct array/slice access: Would require data structure changes and violate abstraction requirements

## Error Handling Strategy

**Decision**: Use existing frozenDB error types with proper wrapping and propagation.

**Rationale**:
- Maintains consistency with existing error handling patterns from `errors.go`
- Provides clear programmatic error handling through error codes
- Supports error unwrapping for debugging
- Meets constitutional requirements for structured error handling

**Error types to use**:
- `KeyNotFoundError`: Target not found in valid range
- `InvalidInputError`: Invalid parameters (negative skew, out of bounds indices)
- `ReadError`: Callback function failures (I/O issues, corruption)
- `CorruptDatabaseError`: Data integrity issues discovered during search

## Performance Optimization Techniques

**Decision**: Implement iterative approach with minimal memory allocation.

**Rationale**:
- Stack-efficient compared to recursive implementation
- O(1) space complexity using only lo, hi, mid index variables
- No slice creation or dynamic memory allocation during search
- Integer arithmetic with `mid = lo + (hi-lo)/2` to avoid overflow

**Key optimizations**:
1. **Early termination** on callback errors
2. **Bound checking** before search to validate range
3. **Minimal callbacks** - only call when necessary
4. **Cache-friendly** access pattern through binary search

## Integration Approach

**Decision**: Implement as standalone function in `frozendb/fuzzy_binary_search.go`.

**Rationale**:
- Follows existing code organization patterns
- Allows testing independent of existing Finder implementations
- Can be easily integrated into future Finder implementations
- Maintains single-responsibility principle

**Alternatives considered**:
- Method on existing Finder interfaces: Would require interface changes
- Separate package: Over-engineering for this focused algorithm
- Internal helper: Would limit reusability and testability

## Implementation Complexity Analysis

The FuzzyBinarySearch algorithm introduces controlled complexity that is justified and necessary:

| Complexity | Justification | Simpler Alternative Rejected |
|------------|---------------|------------------------------|
| Three-way partitioning | Required to handle indeterminate range caused by clock skew | Two-way binary search cannot handle out-of-order entries |
| Callback interface | Maintains O(1) space and data-structure agnostic design | Direct array access would violate abstraction and increase memory usage |
| Error propagation | Constitutional requirement for structured error handling | Simple error returns would break existing patterns |

The complexity is bounded and predictable:
- Binary search phase: O(log n) operations
- Linear scan phase: O(k) operations where k ≤ entries in ±skew_ms range  
- Total: O(log n) + k as specified
- Space: O(1) constant memory usage

## Validation Strategy

**Spec Testing Requirements**:
- All functional requirements (FR-001 through FR-004) must have corresponding spec tests
- Tests will validate both correctness and performance characteristics
- Follow naming convention `Test_S_022_FR_XXX_Description()`
- Co-locate with implementation in `fuzzy_binary_search_spec_test.go`

**Performance Validation**:
- Measure callback function call count to verify O(log n) + k complexity
- Test with various array sizes and skew values
- Benchmark against theoretical bounds
- Validate memory usage remains O(1)

This research provides a complete foundation for implementing the FuzzyBinarySearch algorithm that meets all frozenDB constitutional requirements while delivering the specified performance characteristics.
