-- ============================================================================
-- Image File Queries
-- ============================================================================

-- name: InsertImageFile :one
INSERT INTO image_file (file, name, ext, url, size_bytes, mtime_unix, status, created_at_unix, updated_at_unix)
VALUES (?, ?, ?, ?, ?, ?, 'new', ?, ?)
ON CONFLICT(file) DO UPDATE SET
    url = excluded.url,
    size_bytes = excluded.size_bytes,
    mtime_unix = excluded.mtime_unix,
    updated_at_unix = excluded.updated_at_unix
RETURNING id;

-- name: GetImageFileByID :one
SELECT * FROM image_file WHERE id = ?;

-- name: GetImageFileByPath :one
SELECT * FROM image_file WHERE file = ?;

-- name: GetPendingImages :many
SELECT * FROM image_file WHERE status IN ('new', 'error') ORDER BY id;

-- name: GetQueuedImages :many
SELECT * FROM image_file WHERE status = 'queued' ORDER BY id;

-- name: UpdateImageFileStatus :exec
UPDATE image_file SET status = ?, updated_at_unix = ? WHERE id = ?;

-- name: UpdateImageFileError :exec
UPDATE image_file SET status = 'error', last_error = ?, updated_at_unix = ? WHERE id = ?;

-- name: UpdateImageFileCompleted :exec
UPDATE image_file SET status = 'completed', description = ?, updated_at_unix = ? WHERE id = ?;

-- name: MarkImagesQueued :exec
UPDATE image_file SET status = 'queued', updated_at_unix = ? WHERE status = 'new';

-- ============================================================================
-- Image Meta Queries
-- ============================================================================

-- name: InsertImageMeta :one
INSERT OR IGNORE INTO image_meta (meta)
VALUES (?)
RETURNING id;

-- name: GetImageMetaByText :one
SELECT id FROM image_meta WHERE meta = ?;

-- name: InsertMetaMap :exec
INSERT OR IGNORE INTO meta_map (image_file_id, image_meta_id)
VALUES (?, ?);

-- name: GetMetaForImage :many
SELECT im.meta FROM image_meta im
JOIN meta_map mm ON im.id = mm.image_meta_id
WHERE mm.image_file_id = ?;

-- name: DeleteMetaMapForImage :exec
DELETE FROM meta_map WHERE image_file_id = ?;

-- ============================================================================
-- AI Batch Queries
-- ============================================================================

-- name: InsertBatch :one
INSERT INTO ai_batch (provider_batch_id, status, request_count, submitted_at_unix, raw_json)
VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: GetBatchByID :one
SELECT * FROM ai_batch WHERE id = ?;

-- name: GetActiveBatches :many
SELECT * FROM ai_batch WHERE status != 'ended' ORDER BY id;

-- name: UpdateBatchStatus :exec
UPDATE ai_batch SET status = ?, last_checked_at_unix = ? WHERE id = ?;

-- name: UpdateBatchEnded :exec
UPDATE ai_batch SET status = 'ended', completed_at_unix = ?, last_checked_at_unix = ?, raw_json = ? WHERE id = ?;

-- ============================================================================
-- AI Request Queries
-- ============================================================================

-- name: InsertRequest :one
INSERT INTO ai_request (image_file_id, custom_id, model, status, created_at_unix)
VALUES (?, ?, ?, 'queued', ?)
RETURNING id;

-- name: GetRequestByID :one
SELECT * FROM ai_request WHERE id = ?;

-- name: GetRequestByCustomID :one
SELECT * FROM ai_request WHERE custom_id = ?;

-- name: GetRequestsForBatch :many
SELECT * FROM ai_request WHERE batch_id = ?;

-- name: GetQueuedRequests :many
SELECT * FROM ai_request WHERE status = 'queued' AND batch_id IS NULL ORDER BY id;

-- name: AttachRequestsToBatch :exec
UPDATE ai_request SET batch_id = ?, status = 'submitted', updated_at_unix = ? WHERE status = 'queued' AND batch_id IS NULL;

-- name: UpdateRequestStatus :exec
UPDATE ai_request SET status = ?, updated_at_unix = ? WHERE id = ?;

-- name: UpdateRequestError :exec
UPDATE ai_request SET status = 'error', error = ?, updated_at_unix = ? WHERE id = ?;

-- name: UpdateRequestCompleted :exec
UPDATE ai_request SET status = 'completed', updated_at_unix = ? WHERE id = ?;

-- ============================================================================
-- AI Result Queries
-- ============================================================================

-- name: InsertResult :one
INSERT INTO ai_result (request_id, output_json, created_at_unix)
VALUES (?, ?, ?)
RETURNING id;

-- name: GetResultByRequestID :one
SELECT * FROM ai_result WHERE request_id = ?;
