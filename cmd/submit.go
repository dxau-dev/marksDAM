package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	fu "github.com/dxau-dev/fileUtilities"
	du "github.com/dxau-dev/dateUtilities"
	"github.com/dxau-dev/marksDAM/config"
	"github.com/dxau-dev/marksDAM/openai"
	dbOpen "github.com/dxau-dev/marksDAM/sql"
	"github.com/dxau-dev/marksDAM/sql/generated_files"
)

// Submit scans the current directory for images and submits them to OpenAI batch API
func Submit(configPath string) error {
	if err := config.SetConfigDir(configPath); err != nil {
		return fmt.Errorf("failed to set config directory: %w", err)
	}

	if err := config.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dbPath := config.GetDBLocation()
	if !fu.FileDirExists(dbPath) {
		return fmt.Errorf("database not found at %s. Run 'setup' first", dbPath)
	}

	db, err := dbOpen.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dbOpen.CloseDB(db)

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

	for _, img := range imageFiles {
		url := webHost + img.Path
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

	fmt.Printf("Creating batch requests for %d images...\n", len(pendingImages))

	model := config.GetModel()
	prompt := config.GetPrompt()
	detail := config.GetDetail()

	var batchRequests []openai.BatchRequest
	var requestImageIDs []int64

	for _, img := range pendingImages {
		customID := fmt.Sprintf("img-%d", img.ID)
		imageURL := img.Url.String
		if imageURL == "" {
			imageURL = webHost + img.File
		}

		req := openai.BuildBatchRequest(customID, imageURL, model, prompt, detail)
		batchRequests = append(batchRequests, req)
		requestImageIDs = append(requestImageIDs, img.ID)

		_, err := queries.InsertRequest(ctx, dbAccess.InsertRequestParams{
			ImageFileID:   img.ID,
			CustomID:      customID,
			Model:         model,
			CreatedAtUnix: nowUnix,
		})
		if err != nil {
			fmt.Printf("Warning: failed to create request for image %d: %v\n", img.ID, err)
		}
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

	batchJSON, _ := json.Marshal(batch)

	batchID, err := queries.InsertBatch(ctx, dbAccess.InsertBatchParams{
		OpenaiBatchID:   batch.ID,
		InputFileID:     inputFileID,
		Endpoint:        string(batch.Endpoint),
		Status:          string(batch.Status),
		RequestCount:    sql.NullInt64{Int64: int64(len(batchRequests)), Valid: true},
		SubmittedAtUnix: nowUnix,
		RawJson:         sql.NullString{String: string(batchJSON), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to save batch to database: %w", err)
	}

	err = queries.AttachRequestsToBatch(ctx, dbAccess.AttachRequestsToBatchParams{
		BatchID:       sql.NullInt64{Int64: batchID, Valid: true},
		UpdatedAtUnix: sql.NullInt64{Int64: nowUnix, Valid: true},
	})
	if err != nil {
		fmt.Printf("Warning: failed to attach requests to batch: %v\n", err)
	}

	for _, imgID := range requestImageIDs {
		err = queries.UpdateImageFileStatus(ctx, dbAccess.UpdateImageFileStatusParams{
			Status:        "submitted",
			UpdatedAtUnix: nowUnix,
			ID:            imgID,
		})
		if err != nil {
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

// ImageFileInfo contains information about a scanned image file
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
		if slices.Contains(extensions, ext) {
			fileInfo, err := fu.GetFileInfo(file.Path)
			modTime := time.Now()
			if err == nil && fileInfo != nil {
				modTime = fileInfo.ModifiedAt
			}
			images = append(images, ImageFileInfo{
				Path:      file.Path,
				Name:      filenameParts.Name,
				Extension: ext,
				ModTime:   modTime,
			})
		}
	}

	return images, nil
}
