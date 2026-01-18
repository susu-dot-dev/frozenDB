# Error Handling Guide

This document describes the structured error handling patterns used throughout frozenDB and provides guidelines for proper error usage.

## Structured Error Pattern

All errors in frozenDB follow a structured pattern defined in `errors.go`. The base `FrozenDBError` struct provides consistent error handling with three key components:

```go
type FrozenDBError struct {
    Code    string // Error code for programmatic handling
    Message string // Human-readable error message  
    Err     error  // Underlying error (optional)
}
```

### Error Types

Specific error types are defined in `errors.go`, each embedding the base `FrozenDBError`. Refer to `errors.go` for the complete list of available error types and their usage documentation.

### Error Creation

Always use the provided constructor functions to create errors:

```go
return NewInvalidInputError("path cannot be empty", nil)
return NewCorruptDatabaseError("invalid header format", underlyingErr)
```

## When to Create New Error Types

### Create a New Error Type When:

1. **Callers Need Different Behavior**: The calling code should handle the error differently than existing error types
2. **Distinct Error Category**: The error represents a fundamentally different category of failure
3. **Programmatic Handling Required**: Applications need to check for this specific error type using type assertions

### Use Existing Error Types When:

1. **Similar User Action**: The error results from the same type of user action as existing errors
2. **No Different Handling Needed**: Callers will treat this error the same as existing ones
3. **Descriptive Message Suffices**: The distinction can be clearly communicated through the error message

## Method-Specific Error Guidelines

### Validate() Methods

`Validate()` methods should generally return `InvalidInputError` for validation failures. These methods check structural validity and field constraints:

```go
func (cfg *CreateConfig) Validate() error {
    if cfg.path == "" {
        return NewInvalidInputError("path cannot be empty", nil)
    }
    // ... other validation checks
    return nil
}
```

**Exception**: `Validate()` methods may delegate to other validation functions that return different error types (e.g., `validatePath()` returning `PathError`).

### UnmarshalText() Methods

`UnmarshalText()` methods should generally wrap underlying errors and return `CorruptDatabaseError`. These methods parse external data and detect corruption:

```go
func (h *Header) UnmarshalText(data []byte) error {
    if len(data) != expectedSize {
        return NewCorruptDatabaseError(
            fmt.Sprintf("invalid header size: expected %d, got %d", expectedSize, len(data)),
            nil,
        )
    }
    
    if err := json.Unmarshal(jsonPart, &h.fields); err != nil {
        return NewCorruptDatabaseError("failed to parse header JSON", err)
    }
    
    return nil
}
```

## Error Code Reference

Error codes are defined alongside their corresponding error types in `errors.go`. Each error type includes documentation about when it should be used.

## Best Practices

1. **Always Use Constructors**: Never create error structs directly - use the `New*Error()` functions
2. **Provide Context**: Include specific details in the human-readable message
3. **Wrap When Appropriate**: Include underlying errors when they provide useful context
4. **Be Specific**: Use the most appropriate error type for the situation
5. **Test Error Types**: Write tests that verify the correct error types are returned
