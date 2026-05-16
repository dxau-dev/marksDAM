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
	markdam "github.com/dxau-dev/marksDAM/anthropic"
	"github.com/dxau-dev/marksDAM/config"
	dbOpen "github.com/dxau-dev/marksDAM/sql"
	"github.com/dxau-dev/marksDAM/sql/generated_files"

	"github.com/anthropics/anthropic-sdk-go"
)

// Retrieve polls Anthropic for batch results and updates the database.
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
	client := markdam.NewClient()

	for _, batch := range activeBatches {
		fmt.Printf("\nChecking batch: %s\n", batch.ProviderBatchID)
		nowUnix := du.ToUTC(time.Now()).Unix()

		apiBatch, err := client.GetBatch(ctx, batch.ProviderBatchID)
		if err != nil {
			fmt.Printf("  Error fetching batch %s status: %v\n", batch.ProviderBatchID, err)
			_ = queries.UpdateBatchStatus(ctx, dbAccess.UpdateBatchStatusParams{
				Status:            batch.Status,
				LastCheckedAtUnix: toNullInt(nowUnix),
				ID:                batch.ID,
			})
			continue
		}

		fmt.Printf("  Status: %s\n", apiBatch.ProcessingStatus)

		switch apiBatch.ProcessingStatus {
		case anthropic.BetaMessageBatchProcessingStatusEnded:
			if err := processEndedBatch(ctx, queries, client, batch, apiBatch, nowUnix); err != nil {
				fmt.Printf("  Error processing ended batch: %v\n", err)
				batchJSON, _ := json.Marshal(apiBatch)
				_ = queries.UpdateBatchEnded(ctx, dbAccess.UpdateBatchEndedParams{
					CompletedAtUnix:   toNullInt(nowUnix),
					LastCheckedAtUnix: toNullInt(nowUnix),
					RawJson:           toNullStr(string(batchJSON)),
					ID:                batch.ID,
				})
			}

		default:
			if err := queries.UpdateBatchStatus(ctx, dbAccess.UpdateBatchStatusParams{
				Status:            string(apiBatch.ProcessingStatus),
				LastCheckedAtUnix: toNullInt(nowUnix),
				ID:                batch.ID,
			}); err != nil {
				fmt.Printf("  Error updating batch status: %v\n", err)
			}
			counts := apiBatch.RequestCounts
			fmt.Printf("  Progress: %d processing, %d succeeded, %d errored\n",
				counts.Processing, counts.Succeeded, counts.Errored)
		}
	}
	return nil
}

func processEndedBatch(ctx context.Context, queries *dbAccess.Queries, client *markdam.Client, batch dbAccess.AiBatch, apiBatch *anthropic.BetaMessageBatch, nowUnix int64) error {
	fmt.Println("  Downloading results...")

	results, err := client.DownloadResults(ctx, batch.ProviderBatchID)
	if err != nil {
		return fmt.Errorf("failed to download results: %w", err)
	}

	fmt.Printf("  Processing %d results...\n", len(results))

	successCount := 0
	errorCount := 0

	for _, res := range results {
		req, err := queries.GetRequestByCustomID(ctx, res.CustomID)
		if err != nil {
			fmt.Printf("    Warning: could not find request for custom_id %s: %v\n", res.CustomID, err)
			continue
		}

		if res.Err != nil {
			errMsg := res.Err.Error()
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

		if len(res.RawJSON) > 0 {
			if _, err := queries.InsertResult(ctx, dbAccess.InsertResultParams{
				RequestID:     req.ID,
				OutputJson:    string(res.RawJSON),
				CreatedAtUnix: nowUnix,
			}); err != nil {
				fmt.Printf("    Warning: failed to save result: %v\n", err)
			}
		}

		if err := queries.UpdateImageFileCompleted(ctx, dbAccess.UpdateImageFileCompletedParams{
			Description:   toNullStr(res.Content),
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

	batchJSON, _ := json.Marshal(apiBatch)
	if err := queries.UpdateBatchEnded(ctx, dbAccess.UpdateBatchEndedParams{
		CompletedAtUnix:   toNullInt(nowUnix),
		LastCheckedAtUnix: toNullInt(nowUnix),
		RawJson:           toNullStr(string(batchJSON)),
		ID:                batch.ID,
	}); err != nil {
		return fmt.Errorf("failed to update batch ended: %w", err)
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
