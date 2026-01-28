package main

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/susu-dot-dev/frozenDB/pkg/frozendb"
)

// ExampleData represents the JSON data we'll store in the database
type ExampleData struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

func main() {
	// Use the sample database in the same directory as the binary
	dbPath := filepath.Join(".", "sample.fdb")

	fmt.Println("=== frozenDB Getting Started Example ===")
	fmt.Println()

	// Step 1: Open the existing sample database
	fmt.Println("Step 1: Opening sample database...")
	fmt.Printf("  Database: %s\n", dbPath)
	db, err := frozendb.NewFrozenDB(dbPath, frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Warning: Failed to close database: %v", err)
		}
	}()
	fmt.Println("✓ Database opened in WRITE mode")
	fmt.Println("  Note: sample.fdb contains 3 pre-existing records")
	fmt.Println()

	// Step 2: Begin a transaction to add new data
	fmt.Println("Step 2: Beginning transaction to add new data...")
	tx, err := db.BeginTx()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	fmt.Println("✓ Transaction started")
	fmt.Println()

	// Step 3: Insert new data using UUIDv7 keys
	fmt.Println("Step 3: Inserting new data...")

	// Generate UUIDv7 keys (time-ordered)
	key1 := uuid.Must(uuid.NewV7())
	key2 := uuid.Must(uuid.NewV7())
	key3 := uuid.Must(uuid.NewV7())

	// Create data to store
	data1 := ExampleData{Message: "Hello from the example!", Count: 100}
	data2 := ExampleData{Message: "frozenDB is immutable", Count: 200}
	data3 := ExampleData{Message: "Time-ordered UUIDv7 keys", Count: 300}

	// Marshal to JSON
	jsonData1, _ := json.Marshal(data1)
	jsonData2, _ := json.Marshal(data2)
	jsonData3, _ := json.Marshal(data3)

	// Add rows to transaction
	if err := tx.AddRow(key1, jsonData1); err != nil {
		log.Fatalf("Failed to add row 1: %v", err)
	}
	fmt.Printf("✓ Added row with key: %s\n", key1)

	if err := tx.AddRow(key2, jsonData2); err != nil {
		log.Fatalf("Failed to add row 2: %v", err)
	}
	fmt.Printf("✓ Added row with key: %s\n", key2)

	if err := tx.AddRow(key3, jsonData3); err != nil {
		log.Fatalf("Failed to add row 3: %v", err)
	}
	fmt.Printf("✓ Added row with key: %s\n", key3)
	fmt.Println()

	// Step 4: Commit the transaction
	fmt.Println("Step 4: Committing transaction...")
	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}
	fmt.Println("✓ Transaction committed successfully")
	fmt.Println()

	// Step 5: Query the data back
	fmt.Println("Step 5: Querying newly added data...")

	var retrievedData ExampleData
	if err := db.Get(key2, &retrievedData); err != nil {
		log.Fatalf("Failed to retrieve data: %v", err)
	}

	fmt.Printf("✓ Retrieved data for key %s:\n", key2)
	fmt.Printf("  Message: %s\n", retrievedData.Message)
	fmt.Printf("  Count: %d\n", retrievedData.Count)
	fmt.Println()

	// Step 6: Verify all newly added keys are accessible
	fmt.Println("Step 6: Verifying all newly added keys...")
	keys := []uuid.UUID{key1, key2, key3}
	for _, key := range keys {
		var data ExampleData
		if err := db.Get(key, &data); err != nil {
			log.Fatalf("Failed to verify key %s: %v", key, err)
		}
		fmt.Printf("✓ Key %s: %s\n", key, data.Message)
	}
	fmt.Println()

	// Step 7: Close the database
	fmt.Println("Step 7: Closing database...")
	if err := db.Close(); err != nil {
		log.Fatalf("Failed to close database: %v", err)
	}
	fmt.Println("✓ Database closed")
	fmt.Println()

	fmt.Println("=== Example completed successfully! ===")
	fmt.Println()
	fmt.Println("Key Takeaways:")
	fmt.Println("  ✓ Opened an existing frozenDB database (sample.fdb)")
	fmt.Println("  ✓ Used transactions to add multiple records atomically")
	fmt.Println("  ✓ Queried data using UUIDv7 keys")
	fmt.Println("  ✓ All data is append-only and immutable")
	fmt.Println()
	fmt.Println("Next Steps:")
	fmt.Println("  • Check out the docs/ directory for file format details")
	fmt.Println("  • Import 'github.com/susu-dot-dev/frozenDB/pkg/frozendb' in your project")
	fmt.Println("  • Use UUIDv7 keys for time-ordered data")
	fmt.Println("  • Remember: frozenDB is append-only and immutable!")
}
