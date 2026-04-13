package main

import (
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/migrate-create <migration_name>")
		os.Exit(1)
	}

	name := os.Args[1]

	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := goose.Create(nil, "migrations", name, "sql"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
