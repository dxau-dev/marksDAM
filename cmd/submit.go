package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	du "github.com/dxau-dev/dateUtilities"
	fu "github.com/dxau-dev/fileUtilities"
	"github.com/dxau-dev/marksDAM/config"
	"github.com/dxau-dev/marksDAM/openai"
	dbOpen "github.com/dxau-dev/marksDAM/sql"
	"github.com/dxau-dev/marksDAM/sql/generated_files"
)

// Submit scans the current directory for images and submits them to OpenAI batch API.
func Submit(configPath string) error {
	if err := config.SetConfigDir(configPath); err != nil {
		return fmt.Errorf("failed to set config directory: %w", err)
	}

	if err := config.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dbPath := config.GetDBLocation()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found at %s. Run 'setup' first", dbPath)
	}

	db, err := dbOpen.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if err := dbOpen.CloseDB(db); err != nil {
			log.Printf("warning: failed to close database: %v", err)
		}
	}()

	queries := dbAccess.New(db)
	ctx := context.Background()

	fmt.Println("Scanning current directory for images...")
	imageFiles, err := scanForImages()
	if err != nil {
		return fmt.Errorf("failed to scan for images: %w", err)
	}

	if len(imageFiles) == 0 {
		fmt.Println("No image files found")
		return nil
	}

	fmt.Printf("Found %d image files\n", len(imageFiles))

	nowUnix := du.ToUTC(time.Now()).Unix()
	webHost := config.GetWebHost()

	webURLPath, err := calculateWebURLPath()
	if err != nil {
		return fmt.Errorf("failed to calculate web URL path: %w", err)
	}
	if webURLPath != "" {
		fmt.Printf("Web URL path prefix: %s\n", webURLPath)
	}

	for _, img := range imageFiles {
		url := buildImageURL(webHost, webURLPath, img.Path)
		id, err := queries.InsertImageFile(ctx, dbAccess.InsertImageFileParams{
			File:          img.Path,
			Name:          img.Name,
			Ext:           img.Extension,
			Url:           sql.NullString{String: url, Valid: true},
			SizeBytes:     sql.NullInt64{Valid: false},
			MtimeUnix:     sql.NullInt64{Int64: img.ModTime.Unix(), Valid: true},
			CreatedAtUnix: nowUnix,
			UpdatedAtUnix: nowUnix,
		})
		if err != nil {
			fmt.Printf("Warning: failed to insert image %s: %v\n", img.Path, err)
			continue
		}
		if id > 0 {
			fmt.Printf("  Added: %s (id=%d)\n", img.Path, id)
		}
	}

	pendingImages, err := queries.GetPendingImages(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending images: %w", err)
	}

	if len(pendingImages) == 0 {
		fmt.Println("No pending images to submit")
		return nil
	}

	model := config.GetModel()
	prompt := config.GetPrompt()
	detail := config.GetDetail()

	// B5: build requests in memory only — no DB writes until after CreateBatch succeeds.
	fmt.Printf("Creating batch requests for %d images...\n", len(pendingImages))
	batchRequests := make([]openai.BatchRequest, 0, len(pendingImages))
	for _, img := range pendingImages {
		customID := fmt.Sprintf("img-%d", img.ID)
		imageURL := img.Url.String
		if imageURL == "" {
			imageURL = buildImageURL(webHost, webURLPath, img.File)
		}
		batchRequests = append(batchRequests, openai.BuildBatchRequest(customID, imageURL, model, prompt, detail))
	}

	jsonlData, err := openai.BuildJSONL(batchRequests)
	if err != nil {
		return fmt.Errorf("failed to build JSONL: %w", err)
	}

	fmt.Println("Uploading batch file to OpenAI...")
	client := openai.NewClient()

	inputFileID, err := client.UploadBatchFile(ctx, jsonlData)
	if err != nil {
		return fmt.Errorf("failed to upload batch file: %w", err)
	}
	fmt.Printf("Uploaded batch file: %s\n", inputFileID)

	fmt.Println("Creating batch...")
	batch, err := client.CreateBatch(ctx, inputFileID)
	if err != nil {
		return fmt.Errorf("failed to create batch: %w", err)
	}

	// B5: CreateBatch succeeded — safe to write to DB now.
	// B3: handle json.Marshal error.
	batchJSON, marshalErr := json.Marshal(batch)
	if marshalErr != nil {
		fmt.Printf("Warning: failed to marshal batch JSON: %v\n", marshalErr)
	}
	rawJson := sql.NullString{Valid: false}
	if marshalErr == nil {
		rawJson = sql.NullString{String: string(batchJSON), Valid: true}
	}

	batchID, err := queries.InsertBatch(ctx, dbAccess.InsertBatchParams{
		OpenaiBatchID:   batch.ID,
		InputFileID:     inputFileID,
		Endpoint:        string(batch.Endpoint),
		Status:          string(batch.Status),
		RequestCount:    sql.NullInt64{Int64: int64(len(batchRequests)), Valid: true},
		SubmittedAtUnix: nowUnix,
		RawJson:         rawJson,
	})
	if err != nil {
		return fmt.Errorf("failed to save batch to database: %w", err)
	}

	for _, img := range pendingImages {
		customID := fmt.Sprintf("img-%d", img.ID)
		if _, err := queries.InsertRequest(ctx, dbAccess.InsertRequestParams{
			ImageFileID:   img.ID,
			CustomID:      customID,
			Model:         model,
			CreatedAtUnix: nowUnix,
		}); err != nil {
			fmt.Printf("Warning: failed to create request for image %d: %v\n", img.ID, err)
		}
	}

	if err := queries.AttachRequestsToBatch(ctx, dbAccess.AttachRequestsToBatchParams{
		BatchID:       sql.NullInt64{Int64: batchID, Valid: true},
		UpdatedAtUnix: sql.NullInt64{Int64: nowUnix, Valid: true},
	}); err != nil {
		fmt.Printf("Warning: failed to attach requests to batch: %v\n", err)
	}

	for _, img := range pendingImages {
		if err := queries.UpdateImageFileStatus(ctx, dbAccess.UpdateImageFileStatusParams{
			Status:        "submitted",
			UpdatedAtUnix: nowUnix,
			ID:            img.ID,
		}); err != nil {
			fmt.Printf("Warning: failed to update image status: %v\n", err)
		}
	}

	fmt.Printf("\nBatch submitted successfully!\n")
	fmt.Printf("  Batch ID: %s\n", batch.ID)
	fmt.Printf("  Status: %s\n", batch.Status)
	fmt.Printf("  Requests: %d\n", len(batchRequests))
	fmt.Println("\nRun 'retrieve' to poll for results.")

	return nil
}

// ImageFileInfo contains information about a scanned image file.
type ImageFileInfo struct {
	Path      string
	Name      string
	Extension string
	ModTime   time.Time
}

func scanForImages() ([]ImageFileInfo, error) {
	extensions := config.GetImageExtensions()
	var images []ImageFileInfo

	files, err := fu.ScanDirectory(".")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir {
			continue
		}
		filenameParts := fu.SplitFilename(file.Path)
		ext := strings.ToLower(filenameParts.Extension)
		if !slices.Contains(extensions, ext) {
			continue
		}

		modTime := time.Now()
		fileInfo, err := fu.GetFileInfo(file.Path)
		if err != nil {
			fmt.Printf("Warning: could not read file info for %s: %v; using current time as mtime\n", file.Path, err)
		} else if fileInfo == nil {
			fmt.Printf("Warning: GetFileInfo returned nil for %s; using current time as mtime\n", file.Path)
		} else {
			modTime = fileInfo.ModifiedAt
		}

		images = append(images, ImageFileInfo{
			Path:      file.Path,
			Name:      filenameParts.Name,
			Extension: ext,
			ModTime:   modTime,
		})
	}

	return images, nil
}

// calculateWebURLPath returns the relative path from systemWebRoot to the current working directory.
func calculateWebURLPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	systemWebRoot := config.GetSystemWebRoot()
	if systemWebRoot == "" {
		return "", nil
	}

	absWebRoot, err := filepath.Abs(systemWebRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve web root path: %w", err)
	}

	relPath, err := filepath.Rel(absWebRoot, cwd)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path from %s to %s: %w", absWebRoot, cwd, err)
	}

	if relPath == "." {
		return "", nil
	}

	return filepath.ToSlash(relPath), nil
}

// buildImageURL constructs the full URL for an image.
func buildImageURL(webHost, webURLPath, imagePath string) string {
	if !strings.HasSuffix(webHost, "/") {
		webHost += "/"
	}
	imagePath = strings.TrimPrefix(imagePath, "./")
	if webURLPath == "" {
		return webHost + imagePath
	}
	return webHost + webURLPath + "/" + imagePath
}
