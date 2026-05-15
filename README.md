# Mark's DAM

A command-line Digital Asset Management utility that recursively scans directories for images and submits them to OpenAI's Batch API for analysis. The system extracts descriptions, text, and tags from images for searchable metadata.

## Features

- Recursive directory scanning for image files
- OpenAI Batch API integration (50% cost savings vs synchronous API)
- SQLite database for tracking images, batches, and metadata
- TOML-based configuration
- Cross-platform support (macOS, Linux)

## Installation

### Prerequisites

- Go 1.22+ (for building from source)
- OpenAI API key with Batch API access

### Building

```bash
# Build for current platform
go build -o marksdam .

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o marksdam-linux-amd64 .
```

### Environment Setup

Set your OpenAI API key:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Usage

### 1. Initialize Configuration

First, create a configuration directory with the TOML config file and SQLite database:

```bash
marksdam setup -path ./dam-config
```

This creates:
- `dam-config/config.toml` - Configuration file
- `dam-config/marksdam.db` - SQLite database

### 2. Configure Settings

Edit `config.toml` to match your environment:

```toml
webHost = "https://example.com/"
systemWebRoot = "/var/www"
imageExtensions = ["gif", "jpeg", "jpg", "png", "webp"]
dbPath = "marksdam.db"
model = "gpt-4.1"
detail = "high"
prompt = "Access the image. Return the data in the following JSON structure: {"data": {"description": $THE_DESCRIPTION, "ocr": [ $SLICE_OF_OCR_WORDS ], "tags":[ $SLICE_OF_TAGS ] }, "meta": { $ANY_META_DATA_IN_JSON_FORMAT } Where $THE_DESCRIPTION is a single sentence descripting, $SLICE_OF_OCR_WORDS are the words, if any, in the image, and $SLICE_OF_TAGS is a list of tags that will be associated with the image for searching. $ANY_META_DATA_IN_JSON_FORMAT contains any additional meta data that is relevant."
completionWindow = "24h"
```

#### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `webHost` | Base URL where images are publicly accessible | `https://example.com/` |
| `systemWebRoot` | Filesystem path to web server root | `/var/www` |
| `imageExtensions` | File extensions to process | `["gif", "jpeg", "jpg", "png", "webp"]` |
| `dbPath` | Database filename (relative to config directory) | `marksdam.db` |
| `model` | OpenAI model for image analysis | `gpt-4.1` |
| `detail` | Image detail level (`low`, `high`, `auto`) | `high` |
| `prompt` | Prompt sent to OpenAI for each image | See default above |
| `completionWindow` | Batch completion window | `24h` |

### 3. Submit Images

Navigate to the directory containing images and run:

```bash
cd /var/www/photos/album1
marksdam submit -config /path/to/dam-config
```

The command will:
1. Scan the current directory recursively for image files
2. Calculate the web URL path relative to `systemWebRoot`
3. Store image records in the database
4. Build and upload a JSONL batch file to OpenAI
5. Create the batch job

**URL Construction Example:**
- `systemWebRoot`: `/var/www`
- `webHost`: `https://example.com/`
- Current directory: `/var/www/photos/album1`
- Image: `sunset.jpg`
- **Resulting URL**: `https://example.com/photos/album1/sunset.jpg`

### 4. Retrieve Results

Poll OpenAI for batch completion and download results:

```bash
marksdam retrieve -config /path/to/dam-config
```

Run this periodically until the batch completes. Results are stored in the database as metadata tags associated with each image.

### 5. View Configuration

Display current configuration:

```bash
marksdam config -path /path/to/dam-config -print
```

## Commands Reference

| Command | Description |
|---------|-------------|
| `setup -path <dir>` | Initialize config and database in the specified directory |
| `submit -config <dir>` | Scan current directory for images and submit batch to OpenAI |
| `retrieve -config <dir>` | Poll OpenAI for results and update database |
| `config -path <dir> -print` | Display current configuration |
| `help` | Show usage information |

## Project Structure

```
marksDAM/
├── main.go                 # CLI entry point and command routing
├── cmd/
│   ├── setup.go           # Setup command implementation
│   ├── submit.go          # Submit command implementation
│   ├── retrieve.go        # Retrieve command implementation
│   └── config.go          # Config command implementation
├── config/
│   └── config.go          # TOML configuration handling
├── openai/
│   └── batch.go           # OpenAI Batch API wrapper
├── sql/
│   ├── schema.sql         # Database schema
│   ├── query.sql          # SQL queries for sqlc
│   ├── sqlc.yaml          # sqlc configuration
│   ├── createDB.go        # Database creation
│   ├── openDB.go          # Database connection
│   └── generated_files/   # sqlc generated code
└── README.md
```

## Database Schema

### Tables

- **image_file** - Discovered image files with status tracking
- **image_meta** - Unique metadata/tag strings
- **meta_map** - Many-to-many mapping between images and metadata
- **openai_batch** - Batch submission records
- **openai_request** - Individual requests within batches
- **openai_result** - Raw OpenAI response storage

### Image Status Values

| Status | Description |
|--------|-------------|
| `new` | Discovered, not yet submitted |
| `queued` | Queued for submission |
| `submitted` | Submitted to OpenAI batch |
| `completed` | Successfully processed |
| `error` | Processing failed |

## Workflow Example

```bash
# 1. Set up configuration (one-time)
marksdam setup -path ~/dam-config

# 2. Edit configuration
nano ~/dam-config/config.toml

# 3. Navigate to images directory
cd /var/www/gallery/2024

# 4. Submit batch
marksdam submit -config ~/dam-config

# 5. Wait and poll for results (run periodically)
marksdam retrieve -config ~/dam-config

# 6. Process another directory
cd /var/www/gallery/2025
marksdam submit -config ~/dam-config
```

## Dependencies

- [github.com/openai/openai-go](https://github.com/openai/openai-go) - OpenAI Go SDK
- [github.com/BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML parser
- [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) - SQLite driver
- [github.com/dxau-dev/fileUtilities](https://github.com/dxau-dev/fileUtilities) - File system utilities
- [github.com/dxau-dev/dateUtilities](https://github.com/dxau-dev/dateUtilities) - Date/time utilities

## Notes

- Images must be publicly accessible via URL for OpenAI to process them
- Batch API has a 24-hour completion window
- Failed images are logged but not automatically retried; run `submit` again to retry
- All dates are stored in UTC format in the database

## License

MIT
