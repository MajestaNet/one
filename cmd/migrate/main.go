package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MajestaNet/ide/internal/config"
	"github.com/MajestaNet/ide/internal/db"
)

// cmd/migrate applies kernel SQL migrations (migrations/) and exits.
func main() {
	config.ApplyDotEnv()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	dir := os.Getenv("MIGRATIONS_PATH")
	if err := pool.Migrate(ctx, dir); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Migrations applied")
}
