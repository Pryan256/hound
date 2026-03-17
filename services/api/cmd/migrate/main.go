package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down|version|force VERSION>")
		os.Exit(1)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://hound:hound@localhost:5432/hound?sslmode=disable"
	}

	// golang-migrate needs the postgres:// scheme with x-migrations-table param
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	m, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init migrations: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	cmd := os.Args[1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintf(os.Stderr, "migrate up failed: %v\n", err)
			os.Exit(1)
		}
		version, dirty, _ := m.Version()
		fmt.Printf("migrated up — version %d (dirty: %v)\n", version, dirty)

	case "down":
		if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintf(os.Stderr, "migrate down failed: %v\n", err)
			os.Exit(1)
		}
		version, dirty, _ := m.Version()
		fmt.Printf("rolled back — version %d (dirty: %v)\n", version, dirty)

	case "version":
		version, dirty, err := m.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			fmt.Fprintf(os.Stderr, "version check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("version: %d (dirty: %v)\n", version, dirty)

	case "force":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: migrate force <version>")
			os.Exit(1)
		}
		var v int
		fmt.Sscanf(os.Args[2], "%d", &v)
		if err := m.Force(v); err != nil {
			fmt.Fprintf(os.Stderr, "force failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("forced version to %d\n", v)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
