-- Image files discovered by scanning
CREATE TABLE IF NOT EXISTS image_file (
    id INTEGER PRIMARY KEY,
    file TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    ext TEXT NOT NULL,
    url TEXT,
    size_bytes INTEGER,
    mtime_unix INTEGER,
    status TEXT NOT NULL DEFAULT 'new',  -- new | queued | submitted | completed | error
    last_error TEXT,
    description TEXT,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER NOT NULL
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

-- AI batch submissions
CREATE TABLE IF NOT EXISTS ai_batch (
    id INTEGER PRIMARY KEY,
    provider_batch_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL,                  -- in_progress | ended
    request_count INTEGER,
    submitted_at_unix INTEGER NOT NULL,
    last_checked_at_unix INTEGER,
    completed_at_unix INTEGER,
    raw_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_batch_status ON ai_batch(status);

-- Individual requests within a batch (one per image)
CREATE TABLE IF NOT EXISTS ai_request (
    id INTEGER PRIMARY KEY,
    image_file_id INTEGER NOT NULL REFERENCES image_file(id),
    batch_id INTEGER REFERENCES ai_batch(id),
    custom_id TEXT UNIQUE NOT NULL,
    model TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',  -- queued | submitted | completed | error
    error TEXT,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER
);

CREATE INDEX IF NOT EXISTS idx_req_image ON ai_request(image_file_id);
CREATE INDEX IF NOT EXISTS idx_req_batch ON ai_request(batch_id);
CREATE INDEX IF NOT EXISTS idx_req_custom_id ON ai_request(custom_id);

-- Raw AI response storage
CREATE TABLE IF NOT EXISTS ai_result (
    id INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES ai_request(id),
    output_json TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_result_req ON ai_result(request_id);
