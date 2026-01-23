package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	fu "github.com/dxau-dev/fileUtilities"
	du "github.com/dxau-dev/dateUtilities"
	"github.com/dxau-dev/marksDAM/config"
	"github.com/dxau-dev/marksDAM/openai"
	dbOpen "github.com/dxau-dev/marksDAM/sql"
	"github.com/dxau-dev/marksDAM/sql/generated_files"
	sdkOpenai "github.com/openai/openai-go"
)

// Retrieve polls OpenAI for batch results and updates the database
func Retrieve(configPath string) error {
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

	activeBatches, err := queries.GetActiveBatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active batches: %w", err)
	}

	if len(activeBatches) == 0 {
		fmt.Println("No active batches to retrieve")
		return nil
	}

	fmt.Printf("Found %d active batch(es)\n", len(activeBatches))
	client := openai.NewClient()

	for _, batch := range activeBatches {
		fmt.Printf("\nChecking batch: %s\n", batch.OpenaiBatchID)

		apiBatch, err := client.GetBatch(ctx, batch.OpenaiBatchID)
		if err != nil {
			fmt.Printf("  Error fetching batch status: %v\n", err)
			continue
		}

		nowUnix := du.ToUTC(time.Now()).Unix()
		fmt.Printf("  Status: %s\n", apiBatch.Status)

		switch apiBatch.Status {
		case "completed":
			err = processCompletedBatch(ctx, queries, client, batch, apiBatch, nowUnix)
			if err != nil {
				fmt.Printf("  Error processing completed batch: %v\n", err)
			}

		case "failed", "expired", "cancelled":
			batchJSON, _ := json.Marshal(apiBatch)
			err = queries.UpdateBatchFailed(ctx, dbAccess.UpdateBatchFailedParams{
				Status:            string(apiBatch.Status),
				ErrorFileID:       toNullStr(apiBatch.ErrorFileID),
				LastCheckedAtUnix: toNullInt(nowUnix),
				RawJson:           toNullStr(string(batchJSON)),
				ID:                batch.ID,
			})
			if err != nil {
				fmt.Printf("  Error updating batch status: %v\n", err)
			}
			fmt.Printf("  Batch %s: %s\n", apiBatch.Status, batch.OpenaiBatchID)

		default:
			err = queries.UpdateBatchStatus(ctx, dbAccess.UpdateBatchStatusParams{
				Status:            string(apiBatch.Status),
				LastCheckedAtUnix: toNullInt(nowUnix),
				ID:                batch.ID,
			})
			if err != nil {
				fmt.Printf("  Error updating batch status: %v\n", err)
			}
			if apiBatch.RequestCounts.Total > 0 {
				fmt.Printf("  Progress: %d/%d completed, %d failed\n",
					apiBatch.RequestCounts.Completed,
					apiBatch.RequestCounts.Total,
					apiBatch.RequestCounts.Failed)
			}
		}
	}

	return nil
}

func processCompletedBatch(ctx context.Context, queries *dbAccess.Queries, client *openai.Client, batch dbAccess.OpenaiBatch, apiBatch *sdkOpenai.Batch, nowUnix int64) error {
	fmt.Println("  Downloading results...")

	if apiBatch.OutputFileID == "" {
		return fmt.Errorf("no output file ID in completed batch")
	}

	outputData, err := client.DownloadFile(ctx, apiBatch.OutputFileID)
	if err != nil {
		return fmt.Errorf("failed to download output: %w", err)
	}

	responses, err := openai.ParseBatchOutput(outputData)
	if err != nil {
		return fmt.Errorf("failed to parse output: %w", err)
	}

	fmt.Printf("  Processing %d responses...\n", len(responses))

	successCount := 0
	errorCount := 0

	for _, resp := range responses {
		req, err := queries.GetRequestByCustomID(ctx, resp.CustomID)
		if err != nil {
			fmt.Printf("    Warning: could not find request for custom_id %s: %v\n", resp.CustomID, err)
			continue
		}

		if resp.Error != nil {
			errMsg := fmt.Sprintf("%s: %s", resp.Error.Code, resp.Error.Message)
			err = queries.UpdateRequestError(ctx, dbAccess.UpdateRequestErrorParams{
				Error:         toNullStr(errMsg),
				UpdatedAtUnix: toNullInt(nowUnix),
				ID:            req.ID,
			})
			if err != nil {
				fmt.Printf("    Warning: failed to update request error: %v\n", err)
			}
			err = queries.UpdateImageFileError(ctx, dbAccess.UpdateImageFileErrorParams{
				LastError:     toNullStr(errMsg),
				UpdatedAtUnix: nowUnix,
				ID:            req.ImageFileID,
			})
			if err != nil {
				fmt.Printf("    Warning: failed to update image error: %v\n", err)
			}
			errorCount++
			continue
		}

		content, err := openai.ExtractContent(resp)
		if err != nil {
			fmt.Printf("    Warning: failed to extract content for %s: %v\n", resp.CustomID, err)
			errorCount++
			continue
		}

		responseJSON, _ := json.Marshal(resp)
		_, err = queries.InsertResult(ctx, dbAccess.InsertResultParams{
			RequestID:     req.ID,
			OutputJson:    string(responseJSON),
			CreatedAtUnix: nowUnix,
		})
		if err != nil {
			fmt.Printf("    Warning: failed to save result: %v\n", err)
		}

		err = extractAndSaveMeta(ctx, queries, req.ImageFileID, content, nowUnix)
		if err != nil {
			fmt.Printf("    Warning: failed to save metadata: %v\n", err)
		}

		err = queries.UpdateRequestCompleted(ctx, dbAccess.UpdateRequestCompletedParams{
			UpdatedAtUnix: toNullInt(nowUnix),
			ID:            req.ID,
		})
		if err != nil {
			fmt.Printf("    Warning: failed to update request status: %v\n", err)
		}

		err = queries.UpdateImageFileStatus(ctx, dbAccess.UpdateImageFileStatusParams{
			Status:        "completed",
			UpdatedAtUnix: nowUnix,
			ID:            req.ImageFileID,
		})
		if err != nil {
			fmt.Printf("    Warning: failed to update image status: %v\n", err)
		}

		successCount++
	}

	batchJSON, _ := json.Marshal(apiBatch)
	err = queries.UpdateBatchCompleted(ctx, dbAccess.UpdateBatchCompletedParams{
		OutputFileID:      toNullStr(apiBatch.OutputFileID),
		ErrorFileID:       toNullStr(apiBatch.ErrorFileID),
		CompletedAtUnix:   toNullInt(nowUnix),
		LastCheckedAtUnix: toNullInt(nowUnix),
		RawJson:           toNullStr(string(batchJSON)),
		ID:                batch.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to update batch completion: %w", err)
	}

	fmt.Printf("  Completed: %d successful, %d errors\n", successCount, errorCount)
	return nil
}

func extractAndSaveMeta(ctx context.Context, queries *dbAccess.Queries, imageFileID int64, content string, nowUnix int64) error {
	words := strings.Fields(content)

	for _, word := range words {
		word = strings.Trim(word, ".,;:!?\"'()[]{}")
		word = strings.ToLower(word)
		if len(word) < 2 {
			continue
		}

		metaID, err := queries.InsertImageMeta(ctx, word)
		if err != nil {
			existingMeta, err := queries.GetImageMetaByText(ctx, word)
			if err != nil {
				continue
			}
			metaID = existingMeta
		}

		err = queries.InsertMetaMap(ctx, dbAccess.InsertMetaMapParams{
			ImageFileID: imageFileID,
			ImageMetaID: metaID,
		})
		if err != nil {
			continue
		}
	}

	return nil
}

func toNullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func toNullInt(i int64) sql.NullInt64 {
	return sql.NullInt64{Int64: i, Valid: true}
}
