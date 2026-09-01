package main

import (
	"fmt"

	"github.com/LRGolden/goulm-memory/pkg/memory"
)

func main() {
	// 1. Initialize the memory store in a local directory
	// Note: goulm-memory handles its own multi-process file locking.
	store, err := memory.NewStore(memory.Config{
		Dir:     "./local-memory-vault",
		Project: "example-project",
	})
	if err != nil {
		panic(err)
	}
	
	// 2. IMPORTANT: Always defer Flush() to persist the state when the program exits.
	defer store.Flush()

	// 3. Store a piece of knowledge
	fmt.Println("--> Remembering a new architectural decision...")
	_, err = store.Remember(memory.RememberOptions{
		Key:      "db-decision",
		Category: memory.CategoryDecision,
		Content:  "We decided to use PostgreSQL because of its robust JSONB support.",
		Tags:     []string{"database", "architecture", "postgres"},
	})
	if err != nil {
		panic(err)
	}

	// 4. Retrieve knowledge using semantic search
	fmt.Println("--> Recalling knowledge about 'database'...")
	results, err := store.Recall("Why did we choose our database?", &memory.Query{
		Limit: 3,
	})
	if err != nil {
		panic(err)
	}

	// 5. Print results
	for i, r := range results {
		fmt.Printf("Result %d (Score: %.2f) | Key: %s | Content: %s\n", 
			i+1, r.Score, r.Capsule.Key, r.Capsule.Content)
	}

	// 6. Print general stats
	stats, _ := store.Stats()
	fmt.Printf("\nVault Stats: %d total capsules.\n", stats.Total)
}
