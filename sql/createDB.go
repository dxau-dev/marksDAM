package dbOpen

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var ddl string

var dsnParameters = "?_busy_timeout=5000&_pragma=foreign_keys(1)"
var pragmaParams = `PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL;`

func CreateDB(path string) error {
	ctx := context.Background()
	dsn := "file:" + path + dsnParameters

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	if _, err = db.Exec(pragmaParams); err != nil {
		db.Close()
		return fmt.Errorf("setting pragmas: %w", err)
	}

	if _, err = db.ExecContext(ctx, ddl); err != nil {
		db.Close()
		return fmt.Errorf("creating schema: %w", err)
	}

	if err = db.Close(); err != nil {
		return fmt.Errorf("closing database: %w", err)
	}
	return nil
}
