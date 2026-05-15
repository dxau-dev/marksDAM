PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;

-- Create new ai_batch (without file-ID columns)
CREATE TABLE ai_batch (
    id INTEGER PRIMARY KEY,
    provider_batch_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL,
    request_count INTEGER,
    submitted_at_unix INTEGER NOT NULL,
    last_checked_at_unix INTEGER,
    completed_at_unix INTEGER,
    raw_json TEXT
);
INSERT INTO ai_batch (id, provider_batch_id, status, request_count, submitted_at_unix, last_checked_at_unix, completed_at_unix, raw_json)
SELECT id, openai_batch_id,
       CASE status
           WHEN 'completed' THEN 'ended'
           WHEN 'failed'    THEN 'ended'
           WHEN 'expired'   THEN 'ended'
           WHEN 'cancelled' THEN 'ended'
           ELSE 'in_progress'
       END,
       request_count, submitted_at_unix, last_checked_at_unix, completed_at_unix, raw_json
FROM openai_batch;
CREATE INDEX idx_batch_status ON ai_batch(status);

-- Create new ai_request
CREATE TABLE ai_request (
    id INTEGER PRIMARY KEY,
    image_file_id INTEGER NOT NULL REFERENCES image_file(id),
    batch_id INTEGER REFERENCES ai_batch(id),
    custom_id TEXT UNIQUE NOT NULL,
    model TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    error TEXT,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER
);
INSERT INTO ai_request SELECT * FROM openai_request;
CREATE INDEX idx_req_image ON ai_request(image_file_id);
CREATE INDEX idx_req_batch ON ai_request(batch_id);
CREATE INDEX idx_req_custom_id ON ai_request(custom_id);

-- Create new ai_result
CREATE TABLE ai_result (
    id INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES ai_request(id),
    output_json TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL
);
INSERT INTO ai_result SELECT * FROM openai_result;
CREATE UNIQUE INDEX idx_result_req ON ai_result(request_id);

-- Drop old tables in dependency order
DROP TABLE openai_result;
DROP TABLE openai_request;
DROP TABLE openai_batch;

COMMIT;
PRAGMA foreign_keys = ON;
