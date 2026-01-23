package cmd

import (
	"fmt"

	"github.com/dxau-dev/marksDAM/config"
)

// ShowConfig displays the current configuration
func ShowConfig(configPath string, print bool) error {
	if err := config.SetConfigDir(configPath); err != nil {
		return fmt.Errorf("failed to set config directory: %w", err)
	}

	if err := config.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg := config.GetConfig()

	if print {
		fmt.Println("Current Configuration:")
		fmt.Println("======================")
		fmt.Printf("Config Directory: %s\n", config.GetConfigDir())
		fmt.Printf("TOML File:        %s\n", config.GetTOMLPath())
		fmt.Printf("Database:         %s\n", config.GetDBLocation())
		fmt.Println()
		fmt.Printf("Web Host:         %s\n", cfg.WebHost)
		fmt.Printf("Image Extensions: %v\n", cfg.ImageExtensions)
		fmt.Printf("Model:            %s\n", cfg.Model)
		fmt.Printf("Detail:           %s\n", cfg.Detail)
		fmt.Printf("Completion Window: %s\n", cfg.CompletionWindow)
		fmt.Println()
		fmt.Println("Prompt:")
		fmt.Printf("  %s\n", cfg.Prompt)
	} else {
		fmt.Printf("Configuration loaded from: %s\n", config.GetTOMLPath())
		fmt.Println("Use -print to display configuration values")
	}

	return nil
}
