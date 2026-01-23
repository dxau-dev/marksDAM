package dbOpen

import (
	"context"
	"database/sql"
	_ "embed"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var ddl string
var dsnParameters = "?_busy_timeout=5000&_foreign_keys=1"
var pragmaParams = `PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL;`

func CreateDB(path string) bool {

	ctx := context.Background()
	dsn := "file:" + path + dsnParameters

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(pragmaParams)
	if err != nil {
		log.Fatal("Error executing PRAGMA", err)
	}

	// create tables
	_, err = db.ExecContext(ctx, ddl)
	if err != nil {
		log.Fatal("Schema creation failed:", err)
	}

	err = db.Close()
	if err != nil {
		log.Fatal("Big db problem closing the new file:", err)
	}
	return true // If it got this far, it worked.
}
