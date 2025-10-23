# Example: Ollama AI Provider Configuration for Hermes

# This example shows how to configure Hermes to use Ollama for local
# AI summarization and embeddings instead of cloud providers.

# AI Provider Configuration
ai {
  # Use Ollama for local AI processing
  provider = "ollama"
  
  # Ollama-specific configuration
  ollama {
    # Ollama API endpoint (default: http://localhost:11434)
    base_url = env("OLLAMA_BASE_URL", "http://localhost:11434")
    
    # Model for document summarization
    # Options: "llama3.2" (3B, fast), "llama3.1" (8B, better quality), "mistral" (7B)
    summarize_model = env("OLLAMA_SUMMARIZE_MODEL", "llama3.2")
    
    # Model for vector embeddings
    # Options: "nomic-embed-text" (768d), "mxbai-embed-large" (1024d), "all-minilm" (384d)
    embedding_model = env("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text")
    
    # Timeout for AI operations (default: 5m)
    timeout = "5m"
  }
}

# Vector Search Configuration (for semantic search)
vector_search {
  # Use Meilisearch for vector indexing (when implemented)
  provider = "meilisearch"
  
  meilisearch {
    host = env("MEILISEARCH_HOST", "http://localhost:7700")
    api_key = env("MEILISEARCH_API_KEY", "")
    
    # Index name for document embeddings
    index_name = "documents_vectors"
    
    # Vector dimensions must match embedding model
    # nomic-embed-text: 768, mxbai-embed-large: 1024, all-minilm: 384
    vector_dimensions = 768
  }
}

# Indexer Configuration
indexer {
  # Enable AI summarization for all documents
  enable_summarization = true
  
  # Enable vector embeddings for semantic search
  enable_embeddings = true
  
  # Embedding configuration
  embeddings {
    # Split documents into chunks for better search
    chunk_size = 200      # Words per chunk
    chunk_overlap = 50    # Words of overlap between chunks
  }
  
  # Summarization configuration
  summarization {
    # What to extract from documents
    extract_key_points = true
    extract_topics = true
    suggest_tags = true
    analyze_status = true
  }
}

# Prerequisites:
#
# 1. Install Ollama:
#    brew install ollama  # macOS
#    Or download from https://ollama.ai/download
#
# 2. Start Ollama service:
#    ollama serve
#
# 3. Pull required models:
#    ollama pull llama3.2
#    ollama pull nomic-embed-text
#
# 4. Verify models are available:
#    ollama list
#
# 5. Start Hermes with this config:
#    ./hermes server -config=config-ollama.hcl

# Environment Variables (optional):
# export OLLAMA_BASE_URL="http://localhost:11434"
# export OLLAMA_SUMMARIZE_MODEL="llama3.2"
# export OLLAMA_EMBEDDING_MODEL="nomic-embed-text"
# export MEILISEARCH_HOST="http://localhost:7700"
# export MEILISEARCH_API_KEY=""  # Optional for local dev

# Usage Examples:
#
# 1. Summarize all documents:
#    ./hermes indexer run --pipeline summarize-all
#
# 2. Generate embeddings for semantic search:
#    ./hermes indexer run --pipeline generate-embeddings
#
# 3. Full AI-enhanced indexing pipeline:
#    ./hermes indexer run --pipeline ai-enhanced
#
# 4. Check Ollama provider status:
#    curl http://localhost:11434/api/version
#
# 5. Monitor summarization progress:
#    ./hermes indexer status --pipeline summarize-all
