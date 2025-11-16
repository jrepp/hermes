-- RFC-088: Enable pgvector for efficient vector similarity search
--
-- This migration enables the pgvector extension and adds vector similarity
-- search capabilities to document_embeddings.
--
-- Prerequisites:
--   - PostgreSQL 12+
--   - pgvector extension installed (https://github.com/pgvector/pgvector)
--
-- Benefits:
--   - ~10x faster similarity search vs JSONB
--   - Native PostgreSQL vector operations
--   - Indexing support (IVFFlat, HNSW)

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Add vector column for efficient similarity search
-- Keep JSONB column for backward compatibility
ALTER TABLE document_embeddings
  ADD COLUMN IF NOT EXISTS embedding_vector vector(1536);

-- Migrate existing JSONB embeddings to vector format
-- This handles embeddings with 1536 dimensions
UPDATE document_embeddings
SET embedding_vector = embedding::text::vector(1536)
WHERE embedding_vector IS NULL
  AND dimensions = 1536
  AND jsonb_array_length(embedding) = 1536;

-- Create index for fast similarity search using cosine distance
-- IVFFlat index: good for datasets with 10k+ vectors
CREATE INDEX IF NOT EXISTS idx_embeddings_vector_cosine
  ON document_embeddings
  USING ivfflat (embedding_vector vector_cosine_ops)
  WITH (lists = 100);

-- Alternative index for L2 (Euclidean) distance
-- Uncomment if you prefer L2 distance over cosine similarity
-- CREATE INDEX IF NOT EXISTS idx_embeddings_vector_l2
--   ON document_embeddings
--   USING ivfflat (embedding_vector vector_l2_ops)
--   WITH (lists = 100);

-- Alternative: HNSW index for better recall but slower inserts
-- Uncomment for production with large datasets
-- CREATE INDEX IF NOT EXISTS idx_embeddings_vector_hnsw
--   ON document_embeddings
--   USING hnsw (embedding_vector vector_cosine_ops)
--   WITH (m = 16, ef_construction = 64);

-- Add a check constraint to ensure vector dimensions match
-- This prevents inserting vectors with wrong dimensions
ALTER TABLE document_embeddings
  ADD CONSTRAINT check_vector_dimensions
  CHECK (
    embedding_vector IS NULL OR
    array_length(embedding_vector::real[], 1) = dimensions
  );

-- Function to automatically sync JSONB to vector on insert/update
CREATE OR REPLACE FUNCTION sync_embedding_to_vector()
RETURNS TRIGGER AS $$
BEGIN
  -- Only sync if dimensions match vector column size
  IF NEW.dimensions = 1536 AND NEW.embedding IS NOT NULL THEN
    NEW.embedding_vector := NEW.embedding::text::vector(1536);
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to keep JSONB and vector in sync
CREATE TRIGGER sync_embedding_vector
  BEFORE INSERT OR UPDATE ON document_embeddings
  FOR EACH ROW
  EXECUTE FUNCTION sync_embedding_to_vector();

-- Comments for documentation
COMMENT ON COLUMN document_embeddings.embedding_vector IS
  'RFC-088: Native pgvector column for fast similarity search (1536 dimensions)';

COMMENT ON INDEX idx_embeddings_vector_cosine IS
  'RFC-088: IVFFlat index for cosine similarity search (~10x faster than JSONB)';
