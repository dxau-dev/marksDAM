package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration options loaded from TOML
type Config struct {
	WebHost          string   `toml:"webHost"`
	SystemWebRoot    string   `toml:"systemWebRoot"`
	ImageExtensions  []string `toml:"imageExtensions"`
	DBPath           string   `toml:"dbPath"`
	Model            string   `toml:"model"`
	Detail           string   `toml:"detail"`
	Prompt           string   `toml:"prompt"`
	CompletionWindow string   `toml:"completionWindow"`
}

// configDir is the directory containing the TOML and database files
var configDir string

// cfg holds the loaded configuration
var cfg *Config

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		WebHost:          "https://example.com/",
		SystemWebRoot:    "/var/www",
		ImageExtensions:  []string{"gif", "jpeg", "jpg", "png", "webp"},
		DBPath:           "marksdam.db",
		Model:            "gpt-4.1",
		Detail:           "high",
		Prompt:           "Access the image. Return the data in the following JSON structure: {\"data\": {\"description\": $THE_DESCRIPTION, \"ocr\": [ $SLICE_OF_OCR_WORDS ], \"tags\":[ $SLICE_OF_TAGS ] }, \"meta\": { $ANY_META_DATA_IN_JSON_FORMAT } Where $THE_DESCRIPTION is a single sentence descripting, $SLICE_OF_OCR_WORDS are the words, if any, in the image, and $SLICE_OF_TAGS is a list of tags that will be associated with the image for searching. $ANY_META_DATA_IN_JSON_FORMAT contains any additional meta data that is relevant.",
		CompletionWindow: "24h",
	}
}

// SetConfigDir sets the directory where config.toml and the database are located
func SetConfigDir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	configDir = absDir
	return nil
}

// GetConfigDir returns the configured directory
func GetConfigDir() string {
	return configDir
}

// GetTOMLPath returns the full path to the TOML configuration file
func GetTOMLPath() string {
	return filepath.Join(configDir, "config.toml")
}

// GetDBLocation returns the full path to the database file
func GetDBLocation() string {
	if cfg != nil && cfg.DBPath != "" {
		return filepath.Join(configDir, cfg.DBPath)
	}
	return filepath.Join(configDir, "marksdam.db")
}

// Load reads the TOML configuration file from the configured directory
func Load() error {
	cfg = DefaultConfig()

	tomlPath := GetTOMLPath()
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		return nil
	}

	_, err := toml.DecodeFile(tomlPath, cfg)
	return err
}

// Save writes the current configuration to the TOML file
func Save(c *Config) error {
	tomlPath := GetTOMLPath()

	f, err := os.Create(tomlPath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(c)
}

// CreateDefaultConfig creates a new config.toml with default values
func CreateDefaultConfig() error {
	return Save(DefaultConfig())
}

// GetWebHost returns the web host URL
func GetWebHost() string {
	if cfg != nil {
		return cfg.WebHost
	}
	return DefaultConfig().WebHost
}

// GetSystemWebRoot returns the filesystem path to the web server root
func GetSystemWebRoot() string {
	if cfg != nil {
		return cfg.SystemWebRoot
	}
	return DefaultConfig().SystemWebRoot
}

// GetImageExtensions returns the list of image extensions to process
func GetImageExtensions() []string {
	if cfg != nil {
		return cfg.ImageExtensions
	}
	return DefaultConfig().ImageExtensions
}

// GetModel returns the OpenAI model to use
func GetModel() string {
	if cfg != nil {
		return cfg.Model
	}
	return DefaultConfig().Model
}

// GetDetail returns the detail level for image processing
func GetDetail() string {
	if cfg != nil {
		return cfg.Detail
	}
	return DefaultConfig().Detail
}

// GetPrompt returns the prompt to send to OpenAI
func GetPrompt() string {
	if cfg != nil {
		return cfg.Prompt
	}
	return DefaultConfig().Prompt
}

// GetCompletionWindow returns the completion window for batch processing
func GetCompletionWindow() string {
	if cfg != nil {
		return cfg.CompletionWindow
	}
	return DefaultConfig().CompletionWindow
}

// GetConfig returns the loaded configuration
func GetConfig() *Config {
	if cfg != nil {
		return cfg
	}
	return DefaultConfig()
}
