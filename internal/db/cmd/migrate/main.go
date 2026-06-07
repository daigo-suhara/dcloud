package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
)

func main() {
	databaseURL := os.Getenv("DCLD_DATABASE_MIGRATION_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DCLD_DATABASE_URL")
	}
	if databaseURL == "" {
		log.Fatal("DCLD_DATABASE_MIGRATION_URL or DCLD_DATABASE_URL is required")
	}
	for attempt := 1; attempt <= 30; attempt++ {
		if err := db.Migrate(databaseURL); err == nil {
			log.Println("database migration completed")
			return
		} else if attempt == 30 {
			log.Fatal(fmt.Errorf("database migration failed: %w", err))
		} else {
			log.Printf("database migration attempt %d failed: %v", attempt, err)
			time.Sleep(2 * time.Second)
		}
	}
}
