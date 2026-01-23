I have the following SQLite table and index schemas:

```
CREATE TABLE IF NOT EXISTS image_file (
    id INTEGER PRIMARY KEY,
    file TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    ext TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS image_meta (
    id INTEGER PRIMARY KEY,
    meta TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_meta ON image_meta(meta);

CREATE TABLE IF NOT EXISTS meta_map (
    image_file_id INTEGER,
    image_meta_id INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_meta_map ON meta_map(image_file_id,image_meta_id);
```

I need the following queries:
* A query to insert a new file into the image_file table. This query should check to see if the file already exists and, if so, not try the insert. If it doesn’t exist, then insert it.
* A query to insert into the image_meta table with similar criteria as the above point.
* A query that associates rows in the image_meta table with files in the image_file table. 