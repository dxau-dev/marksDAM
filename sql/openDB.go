package dbOpen

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"
)

func OpenDB(path string) (*sql.DB, error) {
	dsn := "file:" + path + dsnParameters

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if _, err := db.Exec(pragmaParams); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting pragmas: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}

func CloseDB(db *sql.DB) error {
	return db.Close()
}
