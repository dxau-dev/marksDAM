package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	du "github.com/dxau-dev/dateUtilities"
	"github.com/dxau-dev/marksDAM/config"
	"github.com/dxau-dev/marksDAM/openai"
	dbOpen "github.com/dxau-dev/marksDAM/sql"
	"github.com/dxau-dev/marksDAM/sql/generated_files"
	sdkOpenai "github.com/openai/openai-go"
)

// Retrieve polls OpenAI for batch results and updates the database.
func Retrieve(configPath string) error {
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
		nowUnix := du.ToUTC(time.Now()).Unix()

		apiBatch, err := client.GetBatch(ctx, batch.OpenaiBatchID)
		if err != nil {
			// E4: include batch ID; E3: record the attempt timestamp.
			fmt.Printf("  Error fetching batch %s status: %v\n", batch.OpenaiBatchID, err)
			_ = queries.UpdateBatchStatus(ctx, dbAccess.UpdateBatchStatusParams{
				Status:            batch.Status,
				LastCheckedAtUnix: toNullInt(nowUnix),
				ID:                batch.ID,
			})
			continue
		}

		fmt.Printf("  Status: %s\n", apiBatch.Status)

		switch apiBatch.Status {
		case "completed":
			if err := processCompletedBatch(ctx, queries, client, batch, apiBatch, nowUnix); err != nil {
				fmt.Printf("  Error processing completed batch: %v\n", err)
				// B4: mark terminal so this batch is not retried on every retrieve run.
				batchJSON, marshalErr := json.Marshal(apiBatch)
				if marshalErr != nil {
					fmt.Printf("  Warning: failed to marshal batch JSON: %v\n", marshalErr)
				}
				_ = queries.UpdateBatchFailed(ctx, dbAccess.UpdateBatchFailedParams{
					Status:            "failed",
					ErrorFileID:       toNullStr(apiBatch.ErrorFileID),
					LastCheckedAtUnix: toNullInt(nowUnix),
					RawJson:           toNullStr(string(batchJSON)),
					ID:                batch.ID,
				})
			}

		case "failed", "expired", "cancelled":
			batchJSON, marshalErr := json.Marshal(apiBatch)
			if marshalErr != nil {
				fmt.Printf("  Warning: failed to marshal batch JSON: %v\n", marshalErr)
			}
			if err := queries.UpdateBatchFailed(ctx, dbAccess.UpdateBatchFailedParams{
				Status:            string(apiBatch.Status),
				ErrorFileID:       toNullStr(apiBatch.ErrorFileID),
				LastCheckedAtUnix: toNullInt(nowUnix),
				RawJson:           toNullStr(string(batchJSON)),
				ID:                batch.ID,
			}); err != nil {
				fmt.Printf("  Error updating batch status: %v\n", err)
			}
			fmt.Printf("  Batch %s: %s\n", apiBatch.Status, batch.OpenaiBatchID)

		default:
			if err := queries.UpdateBatchStatus(ctx, dbAccess.UpdateBatchStatusParams{
				Status:            string(apiBatch.Status),
				LastCheckedAtUnix: toNullInt(nowUnix),
				ID:                batch.ID,
			}); err != nil {
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
			if err := queries.UpdateRequestError(ctx, dbAccess.UpdateRequestErrorParams{
				Error:         toNullStr(errMsg),
				UpdatedAtUnix: toNullInt(nowUnix),
				ID:            req.ID,
			}); err != nil {
				fmt.Printf("    Warning: failed to update request error: %v\n", err)
			}
			if err := queries.UpdateImageFileError(ctx, dbAccess.UpdateImageFileErrorParams{
				LastError:     toNullStr(errMsg),
				UpdatedAtUnix: nowUnix,
				ID:            req.ImageFileID,
			}); err != nil {
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

		// B2: handle json.Marshal error; skip InsertResult if marshal fails (output_json is NOT NULL).
		responseJSON, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			fmt.Printf("    Warning: failed to marshal response JSON for %s: %v\n", resp.CustomID, marshalErr)
		} else {
			if _, err := queries.InsertResult(ctx, dbAccess.InsertResultParams{
				RequestID:     req.ID,
				OutputJson:    string(responseJSON),
				CreatedAtUnix: nowUnix,
			}); err != nil {
				fmt.Printf("    Warning: failed to save result: %v\n", err)
			}
		}

		if err := queries.UpdateImageFileCompleted(ctx, dbAccess.UpdateImageFileCompletedParams{
			Description:   toNullStr(content),
			UpdatedAtUnix: nowUnix,
			ID:            req.ImageFileID,
		}); err != nil {
			fmt.Printf("    Warning: failed to update image: %v\n", err)
		}

		if err := queries.UpdateRequestCompleted(ctx, dbAccess.UpdateRequestCompletedParams{
			UpdatedAtUnix: toNullInt(nowUnix),
			ID:            req.ID,
		}); err != nil {
			fmt.Printf("    Warning: failed to update request status: %v\n", err)
		}

		successCount++
	}

	// B2: handle json.Marshal error.
	batchJSON, marshalErr := json.Marshal(apiBatch)
	if marshalErr != nil {
		fmt.Printf("  Warning: failed to marshal batch JSON: %v\n", marshalErr)
	}
	if err := queries.UpdateBatchCompleted(ctx, dbAccess.UpdateBatchCompletedParams{
		OutputFileID:      toNullStr(apiBatch.OutputFileID),
		ErrorFileID:       toNullStr(apiBatch.ErrorFileID),
		CompletedAtUnix:   toNullInt(nowUnix),
		LastCheckedAtUnix: toNullInt(nowUnix),
		RawJson:           toNullStr(string(batchJSON)),
		ID:                batch.ID,
	}); err != nil {
		return fmt.Errorf("failed to update batch completion: %w", err)
	}

	fmt.Printf("  Completed: %d successful, %d errors\n", successCount, errorCount)
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
