# Hermes Indexer

## Overview

The Hermes indexer is a background service that synchronizes documents between Google Drive, the PostgreSQL database, and Algolia search indexes. It continuously monitors configured Google Drive folders for document changes and updates the search indexes to keep them current.

## What It Does

The indexer performs three primary functions:

1. **Document Indexing**: Monitors Google Drive folders for new or updated documents and syncs them to Algolia
2. **Header Refresh**: Automatically updates document headers with current metadata (titles, status, custom fields)
3. **Search Synchronization**: Keeps Algolia search indexes up-to-date with document content and metadata

## Architecture

### Core Components

- **`internal/indexer/indexer.go`**: Main indexer implementation with the `Run()` loop
- **`internal/indexer/refresh_headers.go`**: Generic header refresh logic
- **`internal/indexer/refresh_docs_headers.go`**: Published document header refresh (legacy, commented out)
- **`internal/indexer/refresh_drafts_headers.go`**: Draft document header refresh (legacy, commented out)
- **`internal/cmd/commands/indexer/indexer.go`**: CLI command implementation

### Data Flow

```
Google Drive (Source)
    ↓
    ↓ (Google Drive API)
    ↓
Indexer Service
    ↓
    ├──→ PostgreSQL Database (metadata & tracking)
    └──→ Algolia (full-text search)
```

### Tracking & State Management

The indexer uses two database tables to track its progress:

- **`indexer_metadata`**: Stores the last full index timestamp
- **`indexer_folders`**: Tracks last indexed timestamp for each monitored folder

This allows the indexer to only process documents that have changed since the last run (incremental updates).

## Configuration

The indexer is configured via the `indexer` block in `config.hcl`:

```hcl
indexer {
  // Maximum number of documents processed in parallel (per folder)
  max_parallel_docs = 5

  // Automatically update document headers for published documents
  update_doc_headers = true

  // Automatically update document headers for draft documents
  update_draft_headers = true

  // Use database as source of truth instead of Algolia for document data
  use_database_for_document_data = false
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_parallel_docs` | int | 5 | Number of documents to process concurrently per folder |
| `update_doc_headers` | bool | true | Enable automatic header updates for published documents |
| `update_draft_headers` | bool | true | Enable automatic header updates for draft documents |
| `use_database_for_document_data` | bool | false | Use PostgreSQL instead of Algolia as the source for document metadata |

### Required Dependencies

The indexer requires:

- **Google Workspace configuration**: `google_workspace.docs_folder` and `google_workspace.drafts_folder`
- **Algolia credentials**: App ID, search/write API keys, and index names
- **PostgreSQL database**: Connection details in `postgres` block
- **Base URL**: Application base URL for generating document links

## Running the Indexer

### Command Line

```bash
# Start the indexer with a config file
./hermes indexer -config=config.hcl

# Dry run mode (not yet implemented in current version)
./hermes indexer -config=config.hcl -dry-run
```

### As a Service

The indexer is designed to run continuously as a background service. It:

1. Runs an indexing cycle every 60 seconds (currently hardcoded)
2. Checks for document updates since the last cycle
3. Processes changed documents in parallel (up to `max_parallel_docs`)
4. Updates tracking timestamps in the database
5. Sleeps for 60 seconds before the next cycle

### Exit Codes

- `0`: Normal operation (runs indefinitely until interrupted)
- `1`: Configuration error, database error, or fatal indexing error

## Indexing Process

### Document Indexing Workflow

For each monitored folder (`docs_folder` and `drafts_folder`):

1. **Query for Changes**: Get documents modified since last indexed timestamp from Google Drive
2. **For Each Changed Document**:
   - Retrieve document metadata from database
   - Get reviews and group reviews from database
   - Export document content from Google Drive as plain text
   - Trim content if exceeds 85KB (Algolia record size limit)
   - Convert to Algolia object format
   - Save to Algolia with redirect details
   - Update `document_modified_at` timestamp in database
3. **Update Tracking**: Save new `last_indexed_at` timestamp for folder

### Header Refresh Workflow

If `update_doc_headers` or `update_draft_headers` is enabled:

1. **Query for Changes**: Get documents modified since last header refresh
2. **Filter Active Documents**: Skip documents modified in the last 30 minutes (to avoid disrupting active editing)
3. **For Each Document**:
   - Get document metadata from database or Algolia
   - Update document header with current metadata (title, status, approvers, custom fields)
   - Save updated document back to Google Drive
4. **Parallel Processing**: Process up to `max_parallel_docs` documents concurrently

### Content Size Limits

- **Maximum content size**: 85,000 bytes per document
- **Algolia hard limit**: 100,000 bytes per record (including all fields)
- Content exceeding the limit is automatically trimmed

## Database Models

The indexer interacts with several database models:

- **`Document`**: Core document metadata (Google File ID, doc type, status, etc.)
- **`DocumentReview`**: Individual reviewer assignments
- **`DocumentGroupReview`**: Group-based reviewer assignments  
- **`IndexerMetadata`**: Global indexer state (last full index time)
- **`IndexerFolder`**: Per-folder tracking (last indexed time by folder ID)

## Monitoring & Logging

The indexer uses structured logging via `hashicorp/go-hclog`:

```
[INFO]  indexer: indexing documents folder: folder_id=... last_indexed_at=...
[INFO]  indexer: indexing document: google_file_id=... folder_id=...
[INFO]  indexer: indexed document: google_file_id=... folder_id=...
[INFO]  indexer: refreshing draft document headers: folder_id=...
[INFO]  indexer: done refreshing draft document headers
[INFO]  indexer: sleeping for a minute before the next indexing run...
```

### Error Handling

The indexer uses `os.Exit(1)` for fatal errors, which causes the process to terminate. In production, it should be run under a process supervisor (systemd, Docker restart policies, Kubernetes, etc.) that will automatically restart it.

Common error scenarios:
- Database connection failures
- Algolia API errors
- Google Drive API rate limits or errors
- Document parsing/conversion errors

## Development & Testing

### Dry Run Mode

The indexer has a `-dry-run` flag defined but it's not fully implemented in the current codebase. When implemented, it should:

- Print document data instead of saving to Algolia
- Skip database updates
- Allow testing without side effects

### Testing Considerations

When testing the indexer:

1. **Use a test environment**: Separate Google Drive folders, Algolia indexes, and database
2. **Monitor API quotas**: Google Drive API has rate limits
3. **Check database state**: Verify `indexer_metadata` and `indexer_folders` timestamps
4. **Validate Algolia data**: Ensure documents appear correctly in search indexes
5. **Test incremental updates**: Modify a document and verify it gets re-indexed

### Local Development Setup

```bash
# 1. Configure testing environment
cp config-example.hcl config.hcl
# Edit config.hcl with test folders and credentials

# 2. Start PostgreSQL
make docker/postgres/start

# 3. Build indexer
make bin

# 4. Run indexer
./build/bin/hermes indexer -config=config.hcl
```

## Performance Tuning

### Parallel Processing

The `max_parallel_docs` setting controls concurrency:

- **Lower values (1-3)**: Reduced load on Google Drive API, slower indexing
- **Higher values (10-20)**: Faster indexing, higher API usage, risk of rate limiting
- **Default (5)**: Balanced approach for most deployments

### Indexing Frequency

Currently hardcoded to 60 seconds. Consider:

- **Faster (30s)**: Near real-time updates, higher API usage
- **Slower (5m)**: Reduced API load, delayed search updates

To modify, edit the `time.Sleep(1 * time.Minute)` line in `indexer.go`.

## Limitations & Known Issues

1. **Fixed sleep interval**: 60-second cycle time is hardcoded (TODO: make configurable)
2. **Hard exits on errors**: Uses `os.Exit(1)` instead of graceful error recovery
3. **No backoff on rate limits**: Doesn't implement exponential backoff for API errors
4. **Content size limit**: Documents larger than 85KB have content truncated
5. **30-minute active edit window**: Documents edited in the last 30 minutes skip header refresh
6. **Google Workspace only**: Currently only supports Google Drive as a document source

## Future Improvements

Potential enhancements identified in code TODOs:

- [ ] Make sleep interval configurable
- [ ] Improve error handling (graceful recovery vs. fatal exits)
- [ ] Implement dry-run mode fully
- [ ] Add exponential backoff for API rate limits
- [ ] Support for alternative document storage backends (see `pkg/workspace/`)
- [ ] Metrics and health check endpoints
- [ ] Configurable content size limits
- [ ] Batch processing optimizations

## Related Documentation

- [Algolia Configuration](./README-algolia.md)
- [Google Workspace Setup](./README-google-workspace.md)
- [PostgreSQL Configuration](./README-postgresql.md)
- [Config.hcl Documentation](./CONFIG_HCL_DOCUMENTATION.md)

## See Also

- **API v1**: `/internal/api/v1/` - REST endpoints that trigger indexing operations
- **Search Package**: `/pkg/search/` - Search abstraction layer (Algolia/Meilisearch)
- **Workspace Package**: `/pkg/workspace/` - Workspace provider abstraction (Google/Local)
- **Document Package**: `/pkg/document/` - Document parsing and conversion logic
