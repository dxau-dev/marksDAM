package dbOpen

import (
	"path/filepath"
	"testing"
)

func TestCreateDB(t *testing.T) {
	tests := []struct {
		name    string
		path    func() string
		wantErr bool
	}{
		{
			name:    "valid temp path",
			path:    func() string { return filepath.Join(t.TempDir(), "test.db") },
			wantErr: false,
		},
		{
			name:    "nonexistent parent directory",
			path:    func() string { return "/nonexistent/subdir/test.db" },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateDB(tt.path())
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDB() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenDB(t *testing.T) {
	t.Run("invalid path returns error", func(t *testing.T) {
		// Before the shadowing fix, this returned (nil, nil) — a false success.
		_, err := OpenDB("/nonexistent/path/to/db.db")
		if err == nil {
			t.Error("expected error for nonexistent parent directory, got nil")
		}
	})

	t.Run("valid path succeeds", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "open_test.db")
		if err := CreateDB(dbPath); err != nil {
			t.Fatalf("CreateDB: %v", err)
		}
		db, err := OpenDB(dbPath)
		if err != nil {
			t.Fatalf("OpenDB: %v", err)
		}
		if err := CloseDB(db); err != nil {
			t.Errorf("CloseDB: %v", err)
		}
	})
}

func TestCloseDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "close_test.db")
	if err := CreateDB(dbPath); err != nil {
		t.Fatalf("CreateDB: %v", err)
	}
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	if err := CloseDB(db); err != nil {
		t.Errorf("CloseDB returned unexpected error: %v", err)
	}
}
