-- RFC-088: Rollback pgvector extension

-- Drop trigger and function
DROP TRIGGER IF EXISTS sync_embedding_vector ON document_embeddings;
DROP FUNCTION IF EXISTS sync_embedding_to_vector();

-- Drop constraint
ALTER TABLE document_embeddings
  DROP CONSTRAINT IF EXISTS check_vector_dimensions;

-- Drop indexes
DROP INDEX IF EXISTS idx_embeddings_vector_cosine;
DROP INDEX IF EXISTS idx_embeddings_vector_l2;
DROP INDEX IF EXISTS idx_embeddings_vector_hnsw;

-- Drop vector column
ALTER TABLE document_embeddings
  DROP COLUMN IF EXISTS embedding_vector;

-- Drop extension (only if no other tables use it)
-- Commented out for safety - uncomment if you're sure
-- DROP EXTENSION IF EXISTS vector;
