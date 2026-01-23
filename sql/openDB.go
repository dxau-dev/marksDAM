package dbOpen

import (
	"context"
	"database/sql"
	_ "embed"
	"log"
	"time"
)

func OpenDB(path string) (*sql.DB, error) {
	// DSN: enable foreign keys and set a busy timeout (milliseconds)
	// dsnParameters declared and initialised in createDB.go
	dsn := "file:" + path + dsnParameters

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// Limit connections: serialize access unless you intentionally want concurrency.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Set PRAGMAs that are not part of DSN (journal_mode often needs exec)
	// pragmaParams declared in createDB.go
	if _, err := db.Exec(pragmaParams); err != nil {
		err := db.Close()
		if err != nil {
			return nil, err
		}
		return nil, err
	}

	// Optional: check connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		err := db.Close()
		if err != nil {
			return nil, err
		}
		return nil, err
	}
	return db, nil
}

func CloseDB(db *sql.DB) {
	err := db.Close()
	if err != nil {
		log.Fatal(err)
	}
}
