package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/susu-dot-dev/frozenDB/pkg/frozendb"
)

// ExampleData represents the JSON data we'll store in the database
type ExampleData struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// printMutex ensures that console output from different goroutines doesn't get interleaved
var printMutex sync.Mutex

// safePrintf prints to stdout with mutex protection to prevent output splicing
func safePrintf(format string, args ...interface{}) {
	printMutex.Lock()
	defer printMutex.Unlock()
	fmt.Printf(format, args...)
}

func main() {
	// Determine database path - either use provided path or look for sample.fdb
	var dbPath string
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	} else {
		// Try to find sample.fdb in current directory or same directory as binary
		dbPath = "sample.fdb"
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			// Try in the same directory as the binary
			execPath, err := os.Executable()
			if err == nil {
				dbPath = filepath.Join(filepath.Dir(execPath), "concurrent_reader_writer.sample.fdb")
			}
		}
	}

	safePrintf("=== frozenDB Concurrent Reader/Writer Example ===\n")
	safePrintf("Database: %s\n\n", dbPath)

	// Verify sample database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("Sample database not found at %s.\nUsage: %s [path-to-sample.fdb]", dbPath, os.Args[0])
	}
	safePrintf("✓ Using existing sample database\n\n")

	// Generate a UUIDv7 key that both goroutines will use
	testKey := uuid.Must(uuid.NewV7())
	safePrintf("Test key: %s\n\n", testKey)

	// WaitGroup to coordinate goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Start reader goroutine
	safePrintf("Starting reader goroutine...\n")
	go readerRoutine(dbPath, testKey, &wg)

	// Start writer goroutine
	safePrintf("Starting writer goroutine...\n")
	go writerRoutine(dbPath, testKey, &wg)

	// Wait for both goroutines to complete
	wg.Wait()

	safePrintf("\n=== Example completed successfully! ===\n")
	safePrintf("\nKey Observations:\n")
	safePrintf("  ✓ Reader opened database in MODE_READ (no lock)\n")
	safePrintf("  ✓ Writer opened database in MODE_WRITE (exclusive lock)\n")
	safePrintf("  ✓ Reader polled every 1 second for the key\n")
	safePrintf("  ✓ Key became visible to reader only AFTER commit\n")
	safePrintf("  ✓ Reader saw key did not exist before commit\n")
	safePrintf("  ✓ frozenDB's append-only design enables safe concurrent reads\n")
	safePrintf("\nNote: The sample.fdb database now contains the newly added key.\n")
}

// readerRoutine opens the database in read mode and polls for a key every 1 second
func readerRoutine(dbPath string, key uuid.UUID, wg *sync.WaitGroup) {
	defer wg.Done()

	safePrintf("[READER] Opening database in read mode...\n")
	db, err := frozendb.NewFrozenDB(dbPath, frozendb.MODE_READ, frozendb.FinderStrategySimple)
	if err != nil {
		log.Fatalf("[READER] Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			safePrintf("[READER] Warning: Failed to close database: %v\n", err)
		}
		safePrintf("[READER] Database closed\n")
	}()
	safePrintf("[READER] ✓ Database opened in MODE_READ\n")

	// Poll for the key every 1 second for up to 10 seconds
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Second)
	pollCount := 0

	for {
		select {
		case <-ticker.C:
			pollCount++
			var data ExampleData
			err := db.Get(key, &data)

			if err != nil {
				// Check if it's a KeyNotFoundError
				if _, ok := err.(*frozendb.KeyNotFoundError); ok {
					safePrintf("[READER] Poll #%d: Key does not exist (not committed yet)\n", pollCount)
				} else {
					safePrintf("[READER] Poll #%d: Error reading key: %v\n", pollCount, err)
				}
			} else {
				safePrintf("[READER] Poll #%d: ✓ Key found! Message: %s, Timestamp: %s\n",
					pollCount, data.Message, data.Timestamp.Format(time.RFC3339))
				safePrintf("[READER] Exiting after finding the key\n")
				return
			}

		case <-timeout:
			safePrintf("[READER] Timeout reached, exiting\n")
			return
		}
	}
}

// writerRoutine opens the database in write mode, waits 3 seconds, writes a key, waits 2 seconds, then commits
func writerRoutine(dbPath string, key uuid.UUID, wg *sync.WaitGroup) {
	defer wg.Done()

	// Sleep briefly to let reader start first
	time.Sleep(500 * time.Millisecond)

	safePrintf("[WRITER] Opening database in write mode...\n")
	db, err := frozendb.NewFrozenDB(dbPath, frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
	if err != nil {
		log.Fatalf("[WRITER] Failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			safePrintf("[WRITER] Warning: Failed to close database: %v\n", err)
		}
		safePrintf("[WRITER] Database closed\n")
	}()
	safePrintf("[WRITER] ✓ Database opened in MODE_WRITE\n")

	// Sleep for 3 seconds before starting transaction
	safePrintf("[WRITER] Sleeping for 3 seconds before writing...\n")
	time.Sleep(3 * time.Second)

	// Begin transaction
	safePrintf("[WRITER] Beginning transaction...\n")
	tx, err := db.BeginTx()
	if err != nil {
		log.Fatalf("[WRITER] Failed to begin transaction: %v", err)
	}
	safePrintf("[WRITER] ✓ Transaction started\n")

	// Write the key
	data := ExampleData{
		Message:   "Hello from concurrent writer!",
		Timestamp: time.Now(),
	}
	jsonData, _ := json.Marshal(data)

	if err := tx.AddRow(key, jsonData); err != nil {
		log.Fatalf("[WRITER] Failed to add row: %v", err)
	}
	safePrintf("[WRITER] ✓ Row added to transaction (key: %s)\n", key)

	// Wait 2 seconds before committing
	safePrintf("[WRITER] Sleeping for 2 seconds before commit...\n")
	time.Sleep(2 * time.Second)

	// Commit the transaction
	safePrintf("[WRITER] Committing transaction...\n")
	if err := tx.Commit(); err != nil {
		log.Fatalf("[WRITER] Failed to commit transaction: %v", err)
	}
	safePrintf("[WRITER] ✓ Transaction committed successfully\n")
	safePrintf("[WRITER] Exiting\n")
}
