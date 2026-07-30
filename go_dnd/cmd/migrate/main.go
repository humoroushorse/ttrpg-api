package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/humoroushorse/go_dnd/pkg/database"
	_ "github.com/lib/pq"
)

func main() {
	var (
		dbURL          = flag.String("db", os.Getenv("DATABASE_URL"), "Database URL")
		migrationsPath = flag.String("path", "./migrations", "Path to migrations directory")
		action         = flag.String("action", "up", "Migration action: up, down, version")
	)
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("Database URL is required (use -db flag or DATABASE_URL env var)")
	}

	db, err := sql.Open("postgres", *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	switch *action {
	case "up":
		fmt.Println("Running migrations up...")
		if err := database.MigrateUp(db, *migrationsPath); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
		fmt.Println("Migrations completed successfully")

	case "down":
		fmt.Println("Rolling back last migration...")
		if err := database.MigrateDown(db, *migrationsPath); err != nil {
			log.Fatalf("Failed to rollback migration: %v", err)
		}
		fmt.Println("Rollback completed successfully")

	case "version":
		version, dirty, err := database.MigrateVersion(db, *migrationsPath)
		if err != nil {
			log.Fatalf("Failed to get migration version: %v", err)
		}
		fmt.Printf("Current version: %d (dirty: %v)\n", version, dirty)

	default:
		log.Fatalf("Unknown action: %s (use: up, down, version)", *action)
	}
}
