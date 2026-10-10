package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Tranduy1dol/ai-lead-automation/migration"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Print("migrations applied")
}

func run() error {
	if os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	return migration.Apply(ctx, pool)
}
