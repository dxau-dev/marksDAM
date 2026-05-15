package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := SetConfigDir(dir); err != nil {
		t.Fatalf("SetConfigDir: %v", err)
	}
	if err := CreateDefaultConfig(); err != nil {
		t.Fatalf("CreateDefaultConfig: %v", err)
	}
	if err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := GetModel(); got != "claude-sonnet-4-5" {
		t.Errorf("GetModel() = %q, want %q", got, "claude-sonnet-4-5")
	}
}

func TestLoad_NoFile(t *testing.T) {
	dir := t.TempDir()
	if err := SetConfigDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := Load(); err != nil {
		t.Errorf("Load() with no config file should return nil, got %v", err)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	if err := SetConfigDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[[invalid toml"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Load(); err == nil {
		t.Error("Load() with invalid TOML should return an error, got nil")
	}
}

func TestGetDBLocation(t *testing.T) {
	dir := t.TempDir()
	if err := SetConfigDir(dir); err != nil {
		t.Fatal(err)
	}

	t.Run("custom DBPath", func(t *testing.T) {
		cfg = &Config{DBPath: "custom.db"}
		got := GetDBLocation()
		want := filepath.Join(dir, "custom.db")
		if got != want {
			t.Errorf("GetDBLocation() = %q, want %q", got, want)
		}
	})

	t.Run("nil config uses default", func(t *testing.T) {
		cfg = nil
		got := GetDBLocation()
		want := filepath.Join(dir, "marksdam.db")
		if got != want {
			t.Errorf("GetDBLocation() = %q, want %q", got, want)
		}
	})
}
