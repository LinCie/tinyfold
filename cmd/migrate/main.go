package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const (
	defaultDatabaseURL   = "./data/app.db"
	defaultMigrationsDir = "migrations"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s {up|down|status|reset|version}", os.Args[0])
	}

	databaseURL := getenv("DATABASE_URL", defaultDatabaseURL)
	migrationsDir := getenv("MIGRATIONS_DIR", defaultMigrationsDir)

	if err := goose.SetDialect("sqlite"); err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	var migrationErr error
	switch os.Args[1] {
	case "up":
		migrationErr = goose.UpContext(ctx, db, migrationsDir)
	case "down":
		migrationErr = goose.DownContext(ctx, db, migrationsDir)
	case "status":
		migrationErr = goose.StatusContext(ctx, db, migrationsDir)
	case "reset":
		migrationErr = goose.ResetContext(ctx, db, migrationsDir)
	case "version":
		migrationErr = goose.VersionContext(ctx, db, migrationsDir)
	default:
		migrationErr = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if migrationErr != nil {
		log.Fatal(migrationErr)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
