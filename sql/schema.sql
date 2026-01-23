-- Image files discovered by scanning
CREATE TABLE IF NOT EXISTS image_file (
    id INTEGER PRIMARY KEY,
    file TEXT UNIQUE NOT NULL,          -- Relative path from scan directory (e.g., "job-10/animals/mycat.jpg")
    name TEXT NOT NULL,                  -- Filename without extension
    ext TEXT NOT NULL,                   -- File extension
    url TEXT,                            -- Full URL (webHost + file)
    size_bytes INTEGER,                  -- File size in bytes
    mtime_unix INTEGER,                  -- File modification time (Unix timestamp, UTC)
    status TEXT NOT NULL DEFAULT 'new',  -- new | queued | submitted | completed | error
    last_error TEXT,                     -- Error message if status is 'error'
    created_at_unix INTEGER NOT NULL,    -- Record creation time (UTC)
    updated_at_unix INTEGER NOT NULL     -- Record last update time (UTC)
);

CREATE INDEX IF NOT EXISTS idx_image_status ON image_file(status);

-- Metadata/tags associated with images
CREATE TABLE IF NOT EXISTS image_meta (
    id INTEGER PRIMARY KEY,
    meta TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_meta ON image_meta(meta);

-- Many-to-many mapping between images and metadata
CREATE TABLE IF NOT EXISTS meta_map (
    image_file_id INTEGER NOT NULL REFERENCES image_file(id),
    image_meta_id INTEGER NOT NULL REFERENCES image_meta(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_meta_map ON meta_map(image_file_id, image_meta_id);

-- OpenAI batch submissions
CREATE TABLE IF NOT EXISTS openai_batch (
    id INTEGER PRIMARY KEY,
    openai_batch_id TEXT UNIQUE NOT NULL,    -- OpenAI's batch ID
    input_file_id TEXT NOT NULL,             -- OpenAI file ID for uploaded JSONL
    output_file_id TEXT,                     -- OpenAI file ID for results (when completed)
    error_file_id TEXT,                      -- OpenAI file ID for errors (if any)
    endpoint TEXT NOT NULL,                  -- API endpoint (e.g., "/v1/chat/completions")
    status TEXT NOT NULL,                    -- validating | in_progress | finalizing | completed | failed | expired | cancelled
    request_count INTEGER,                   -- Number of requests in batch
    submitted_at_unix INTEGER NOT NULL,      -- Submission time (UTC)
    last_checked_at_unix INTEGER,            -- Last poll time (UTC)
    completed_at_unix INTEGER,               -- Completion time (UTC)
    raw_json TEXT                            -- Full batch object JSON for debugging
);

CREATE INDEX IF NOT EXISTS idx_batch_status ON openai_batch(status);

-- Individual requests within a batch (one per image)
CREATE TABLE IF NOT EXISTS openai_request (
    id INTEGER PRIMARY KEY,
    image_file_id INTEGER NOT NULL REFERENCES image_file(id),
    batch_id INTEGER REFERENCES openai_batch(id),
    custom_id TEXT UNIQUE NOT NULL,          -- Maps to OpenAI custom_id field
    model TEXT NOT NULL,                     -- Model used for this request
    status TEXT NOT NULL DEFAULT 'queued',   -- queued | submitted | completed | error
    error TEXT,                              -- Error message if failed
    created_at_unix INTEGER NOT NULL,        -- Request creation time (UTC)
    updated_at_unix INTEGER                  -- Last update time (UTC)
);

CREATE INDEX IF NOT EXISTS idx_req_image ON openai_request(image_file_id);
CREATE INDEX IF NOT EXISTS idx_req_batch ON openai_request(batch_id);
CREATE INDEX IF NOT EXISTS idx_req_custom_id ON openai_request(custom_id);

-- Raw OpenAI response storage
CREATE TABLE IF NOT EXISTS openai_result (
    id INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES openai_request(id),
    output_json TEXT NOT NULL,               -- Raw JSON response from OpenAI
    created_at_unix INTEGER NOT NULL         -- Result storage time (UTC)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_result_req ON openai_result(request_id);
