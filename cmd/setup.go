package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	fu "github.com/dxau-dev/fileUtilities"
	"github.com/dxau-dev/marksDAM/config"
	dbOpen "github.com/dxau-dev/marksDAM/sql"
)

// Setup initializes the configuration directory with TOML config and database
func Setup(relativePath string) error {
	absPath, err := filepath.Abs(relativePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	if !fu.FileDirExists(absPath) {
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", absPath, err)
		}
		fmt.Printf("Created directory: %s\n", absPath)
	}

	if err := config.SetConfigDir(absPath); err != nil {
		return fmt.Errorf("failed to set config directory: %w", err)
	}

	tomlPath := config.GetTOMLPath()
	if fu.FileDirExists(tomlPath) {
		fmt.Printf("Config file already exists: %s\n", tomlPath)
	} else {
		if err := config.CreateDefaultConfig(); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		fmt.Printf("Created config file: %s\n", tomlPath)
	}

	dbPath := config.GetDBLocation()
	if fu.FileDirExists(dbPath) {
		fmt.Printf("Database already exists: %s\n", dbPath)
	} else {
		if err := dbOpen.CreateDB(dbPath); err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		fmt.Printf("Created database: %s\n", dbPath)
	}

	fmt.Println("\nSetup complete. Edit config.toml to configure your settings.")
	return nil
}
