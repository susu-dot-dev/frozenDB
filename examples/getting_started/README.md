# Getting Started Example

This example demonstrates how to use the frozenDB public API to work with an existing database.

## Files

- `main.go` - The example program that demonstrates database operations
- `sample.fdb` - A pre-created frozenDB database with 3 sample records
- `create_sample.go` - One-off tool to regenerate `sample.fdb` (requires sudo)
- `example_spec_test.go` - Spec tests validating the example

## Running the Example

```bash
# From the repository root
go run ./examples/getting_started/main.go

# Or build and run
make build-examples
./dist/examples/getting_started
```

## What the Example Demonstrates

The example shows how to:
1. Open an existing frozenDB database in WRITE mode
2. Begin a new transaction
3. Insert data with UUIDv7 keys
4. Commit the transaction
5. Query data using the Get() method
6. Close the database properly

## About sample.fdb

The `sample.fdb` file is a pre-created frozenDB database that contains 3 records. This file is committed to the repository so the example can run without requiring sudo privileges for database creation.

The database was created with:
- Row size: 256 bytes
- Time skew: 5000ms

It contains 3 sample records with messages about frozenDB features.

## Regenerating sample.fdb

If you need to regenerate the sample database:

```bash
cd examples/getting_started

# Remove the old database (requires sudo to remove append-only file)
sudo chattr -a sample.fdb
rm sample.fdb

# Create new database (requires sudo for append-only attribute)
sudo go run create_sample.go

# The new sample.fdb can be committed to git
```

**Note**: Database creation requires sudo privileges because frozenDB sets the append-only file attribute at the filesystem level to ensure immutability.
