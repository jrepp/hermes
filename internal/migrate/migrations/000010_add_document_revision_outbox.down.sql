-- RFC-088: Rollback event-driven indexer tables

-- Drop pipeline executions table (has FK to outbox)
DROP TABLE IF EXISTS document_revision_pipeline_executions;

-- Drop outbox table (has FK to document_revisions)
DROP TABLE IF EXISTS document_revision_outbox;
