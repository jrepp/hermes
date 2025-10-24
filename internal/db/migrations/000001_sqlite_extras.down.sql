-- Rollback SQLite-specific enhancements

-- Reset to default settings
PRAGMA mmap_size = 0;
PRAGMA synchronous = FULL;
PRAGMA temp_store = DEFAULT;
PRAGMA journal_mode = DELETE;
PRAGMA foreign_keys = OFF;
