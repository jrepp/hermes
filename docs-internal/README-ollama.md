# Ollama AI Provider - Local Llama on macOS

**Status**: ✅ Implemented
**File**: `pkg/ai/ollama/provider.go`

## Overview

The Ollama AI provider enables **fully local AI summarization and embeddings** using Llama and other open-source models on macOS (including Apple Silicon), Linux, and Windows. This eliminates API costs and provides privacy-preserving document processing.

## Features

- ✅ **Document Summarization** using Llama 3.2 (3B) or other text generation models
- ✅ **Vector Embeddings** using nomic-embed-text (768 dimensions) or mxbai-embed-large
- ✅ **Chunked Embeddings** with configurable chunk size and overlap
- ✅ **Structured Output** parsing from Llama responses
- ✅ **Configurable Models** for different use cases
- ✅ **Local Execution** with no external API dependencies

## Installation & Setup

### 1. Install Ollama

**macOS**:
```bash
brew install ollama
```

**Or download from**: https://ollama.ai/download

### 2. Start Ollama Service

```bash
ollama serve
```

This starts the Ollama API server at `http://localhost:11434`.

### 3. Pull Required Models

**For Summarization**:
```bash
# Llama 3.2 3B - Fast, good quality (recommended)
ollama pull llama3.2

# Or Llama 3.1 8B - Higher quality, slower
ollama pull llama3.1
```

**For Embeddings**:
```bash
# Nomic Embed Text - 768 dimensions, optimized for search
ollama pull nomic-embed-text

# Or mxbai-embed-large - 1024 dimensions, higher quality
ollama pull mxbai-embed-large
```

### 4. Verify Installation

```bash
# Check available models
ollama list

# Test summarization
ollama run llama3.2 "Summarize: AI is transforming document management."

# Test embeddings
curl http://localhost:11434/api/embeddings -d '{
  "model": "nomic-embed-text",
  "prompt": "test document"
}'
```

## Configuration

### Basic Configuration

```go
import "github.com/hashicorp-forge/hermes/pkg/ai/ollama"

cfg := ollama.DefaultConfig()
// Uses:
// - BaseURL: http://localhost:11434
// - SummarizeModel: llama3.2
// - EmbeddingModel: nomic-embed-text
// - Timeout: 5 minutes

provider, err := ollama.NewProvider(cfg)
if err != nil {
    log.Fatal(err)
}
```

### Custom Configuration

```go
cfg := &ollama.Config{
    BaseURL:        "http://localhost:11434",
    SummarizeModel: "llama3.1",          // Use larger model
    EmbeddingModel: "mxbai-embed-large", // Use higher quality embeddings
    Timeout:        10 * time.Minute,    // Longer timeout for large documents
}

provider, err := ollama.NewProvider(cfg)
```

### Remote Ollama Server

If running Ollama on a different machine:

```go
cfg := &ollama.Config{
    BaseURL:        "http://ollama-server.internal:11434",
    SummarizeModel: "llama3.2",
    EmbeddingModel: "nomic-embed-text",
    Timeout:        5 * time.Minute,
}
```

## Usage Examples

### Document Summarization

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/hashicorp-forge/hermes/pkg/ai"
    "github.com/hashicorp-forge/hermes/pkg/ai/ollama"
)

func main() {
    provider, err := ollama.NewProvider(ollama.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }

    req := &ai.SummarizeRequest{
        Title:            "RFC: Indexer Refactor",
        DocType:          "RFC",
        Content:          "This RFC proposes a comprehensive refactor...",
        ExtractKeyPoints: true,
        ExtractTopics:    true,
        SuggestTags:      true,
        AnalyzeStatus:    true,
    }

    resp, err := provider.Summarize(context.Background(), req)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Model: %s\n", resp.Model)
    fmt.Printf("Tokens: %d\n", resp.TokensUsed)
    fmt.Printf("Summary: %s\n", resp.Summary.ExecutiveSummary)
    fmt.Printf("Key Points: %v\n", resp.Summary.KeyPoints)
    fmt.Printf("Topics: %v\n", resp.Summary.Topics)
    fmt.Printf("Tags: %v\n", resp.Summary.Tags)
    fmt.Printf("Status: %s\n", resp.Summary.SuggestedStatus)
}
```

### Generate Embeddings (Single)

```go
req := &ai.EmbeddingRequest{
    Texts: []string{
        "This document describes the indexer refactor architecture.",
    },
}

resp, err := provider.GenerateEmbedding(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Model: %s\n", resp.Model)
fmt.Printf("Dimensions: %d\n", resp.Dimensions)
fmt.Printf("Embedding: %v\n", resp.Embeddings.ContentEmbedding[:5]) // First 5 values
```

### Generate Chunked Embeddings

```go
req := &ai.EmbeddingRequest{
    Texts: []string{
        "Long document content that needs to be chunked for better search...",
    },
    ChunkSize:    200,  // Words per chunk
    ChunkOverlap: 50,   // Overlap between chunks
}

resp, err := provider.GenerateEmbedding(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

for _, chunk := range resp.Embeddings.Chunks {
    fmt.Printf("Chunk %d: %s\n", chunk.ChunkIndex, chunk.Text[:50])
    fmt.Printf("  Embedding dims: %d\n", len(chunk.Embedding))
}
```

## Integration with Indexer Commands

### Using Ollama in Summarize Command

```go
import (
    "github.com/hashicorp-forge/hermes/pkg/ai/ollama"
    "github.com/hashicorp-forge/hermes/pkg/indexer/commands"
)

// Create Ollama provider
aiProvider, err := ollama.NewProvider(ollama.DefaultConfig())
if err != nil {
    log.Fatal(err)
}

// Create summarize command
summarizeCmd := commands.NewSummarizeCommand(aiProvider, db)

// Use in pipeline
pipeline := indexer.NewPipeline("summarize-documents").
    AddCommand(discoverCmd).
    AddCommand(summarizeCmd).
    AddCommand(indexCmd)

results, err := pipeline.Execute(ctx, baseCtx)
```

### Using Ollama in Embedding Command

```go
// Create embedding command with Ollama
embeddingCmd := commands.NewGenerateEmbeddingCommand(aiProvider, &commands.EmbeddingConfig{
    ChunkSize:    200,
    ChunkOverlap: 50,
})

// Create vector indexing command
vectorIndexCmd := commands.NewIndexVectorCommand(vectorSearchProvider)

// Pipeline for semantic search preparation
pipeline := indexer.NewPipeline("prepare-semantic-search").
    AddCommand(discoverCmd).
    AddCommand(embeddingCmd).
    AddCommand(vectorIndexCmd)

results, err := pipeline.Execute(ctx, baseCtx)
```

## Model Selection Guide

### Summarization Models

| Model | Size | Speed | Quality | Use Case |
|-------|------|-------|---------|----------|
| `llama3.2` | 3B | ⚡⚡⚡ Fast | ⭐⭐⭐ Good | Development, testing, bulk processing |
| `llama3.1` | 8B | ⚡⚡ Medium | ⭐⭐⭐⭐ Great | Production, high-quality summaries |
| `mistral` | 7B | ⚡⚡ Medium | ⭐⭐⭐⭐ Great | Alternative to Llama, code-focused |

### Embedding Models

| Model | Dimensions | Speed | Quality | Use Case |
|-------|------------|-------|---------|----------|
| `nomic-embed-text` | 768 | ⚡⚡⚡ Fast | ⭐⭐⭐ Good | Standard semantic search |
| `mxbai-embed-large` | 1024 | ⚡⚡ Medium | ⭐⭐⭐⭐ Great | High-precision search |
| `all-minilm` | 384 | ⚡⚡⚡⚡ Very Fast | ⭐⭐ Decent | Low-resource environments |

## Performance Characteristics

### Apple Silicon (M1/M2/M3)

**Llama 3.2 3B Summarization**:
- Speed: ~30-50 tokens/second
- Typical document (1000 words): ~5-10 seconds
- Memory: ~3GB

**Nomic Embed Text (768d)**:
- Speed: ~100 embeddings/second
- Typical document: <1 second
- Memory: ~500MB

### Intel/AMD (x86_64)

**Llama 3.2 3B Summarization**:
- Speed: ~10-20 tokens/second (CPU only)
- Typical document (1000 words): ~15-30 seconds
- Memory: ~3GB

**Nomic Embed Text (768d)**:
- Speed: ~50 embeddings/second
- Typical document: ~2 seconds
- Memory: ~500MB

## Error Handling

### Common Errors

**Ollama Not Running**:
```
Error: ollama request failed: dial tcp [::1]:11434: connect: connection refused
Solution: Run `ollama serve` in a separate terminal
```

**Model Not Pulled**:
```
Error: ollama returned status 404: model 'llama3.2' not found
Solution: Run `ollama pull llama3.2`
```

**Out of Memory**:
```
Error: failed to load model: not enough memory
Solution: Use smaller model (llama3.2 instead of llama3.1) or close other apps
```

**Timeout**:
```
Error: context deadline exceeded
Solution: Increase cfg.Timeout for large documents
```

### Retry Logic

```go
func summarizeWithRetry(ctx context.Context, provider ai.Provider, req *ai.SummarizeRequest, maxRetries int) (*ai.SummarizeResponse, error) {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        resp, err := provider.Summarize(ctx, req)
        if err == nil {
            return resp, nil
        }
        
        lastErr = err
        
        // Don't retry on user errors
        if strings.Contains(err.Error(), "model") {
            return nil, err
        }
        
        // Exponential backoff
        time.Sleep(time.Duration(i+1) * 2 * time.Second)
    }
    
    return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
```

## Testing

### Unit Tests

```go
func TestOllamaProvider_Summarize(t *testing.T) {
    // Skip if Ollama not available
    if _, err := http.Get("http://localhost:11434/api/version"); err != nil {
        t.Skip("Ollama not running")
    }
    
    provider, err := ollama.NewProvider(ollama.DefaultConfig())
    require.NoError(t, err)
    
    req := &ai.SummarizeRequest{
        Title:            "Test Document",
        Content:          "This is a test document about AI summarization.",
        ExtractKeyPoints: true,
    }
    
    resp, err := provider.Summarize(context.Background(), req)
    require.NoError(t, err)
    assert.NotEmpty(t, resp.Summary.ExecutiveSummary)
    assert.Greater(t, len(resp.Summary.KeyPoints), 0)
}
```

### Integration Tests

See `pkg/indexer/commands/summarize_test.go` for examples using the mock provider for deterministic tests, and optional integration tests with real Ollama.

## Comparison with Cloud Providers

### Cost

| Provider | Summarization (per 1K docs) | Embeddings (per 1K docs) | Total Cost |
|----------|------------------------------|---------------------------|------------|
| **Ollama** | $0 (free) | $0 (free) | **$0** |
| AWS Bedrock (Claude 3.5 Sonnet) | ~$150 | ~$1 (Titan) | **~$151** |
| OpenAI (GPT-4) | ~$300 | ~$2 (ada-002) | **~$302** |

### Privacy

| Provider | Data Location | Data Retention | Privacy |
|----------|---------------|----------------|---------|
| **Ollama** | Local only | None (ephemeral) | ⭐⭐⭐⭐⭐ Complete |
| AWS Bedrock | AWS data centers | Per AWS policy | ⭐⭐⭐ Depends on config |
| OpenAI | OpenAI servers | 30 days default | ⭐⭐ Sent to third party |

### Performance

| Provider | Latency | Throughput | Availability |
|----------|---------|------------|--------------|
| **Ollama** | <10s local | 30-50 docs/min | ⭐⭐⭐⭐⭐ Always |
| AWS Bedrock | 1-3s API | 100+ docs/min | ⭐⭐⭐⭐ 99.9% SLA |
| OpenAI | 2-5s API | 60+ docs/min | ⭐⭐⭐ Rate limited |

## Production Deployment

### Dedicated Ollama Server

For production use, run Ollama on a dedicated GPU server:

```bash
# On GPU server (NVIDIA/AMD)
docker run -d \
  --gpus all \
  -p 11434:11434 \
  -v ollama:/root/.ollama \
  --name ollama \
  ollama/ollama

# Pull models
docker exec ollama ollama pull llama3.2
docker exec ollama ollama pull nomic-embed-text
```

### Load Balancing

```go
type OllamaPool struct {
    providers []*ollama.Provider
    current   int
    mu        sync.Mutex
}

func NewOllamaPool(urls []string) (*OllamaPool, error) {
    providers := make([]*ollama.Provider, len(urls))
    
    for i, url := range urls {
        cfg := ollama.DefaultConfig()
        cfg.BaseURL = url
        
        p, err := ollama.NewProvider(cfg)
        if err != nil {
            return nil, err
        }
        providers[i] = p
    }
    
    return &OllamaPool{providers: providers}, nil
}

func (p *OllamaPool) GetProvider() *ollama.Provider {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    provider := p.providers[p.current]
    p.current = (p.current + 1) % len(p.providers)
    
    return provider
}
```

## Troubleshooting

### Slow Performance

1. **Check CPU usage**: Ollama uses CPU by default, GPU acceleration much faster
2. **Reduce model size**: Use llama3.2 (3B) instead of llama3.1 (8B)
3. **Reduce chunk size**: Smaller chunks = faster processing
4. **Increase parallelism**: Process multiple documents concurrently

### High Memory Usage

1. **Use smaller models**: Switch to llama3.2 or all-minilm
2. **Limit concurrent requests**: Process documents sequentially
3. **Restart Ollama**: `ollama stop && ollama serve`

### Inconsistent Output Quality

1. **Improve prompts**: More specific instructions in buildSummarizePrompt
2. **Adjust temperature**: Lower temperature (0.5) for more deterministic output
3. **Try different models**: mistral vs llama3.2 vs llama3.1

## Next Steps

1. **Configure Indexer**: Update `config.hcl` to use Ollama provider
2. **Run Integration Tests**: Test with real documents
3. **Monitor Performance**: Track summarization/embedding times
4. **Optimize Prompts**: Refine prompts for better output quality
5. **Setup Vector Search**: Implement Meilisearch vector adapter

## Related Documentation

- [INDEXER_REFACTOR_IMPLEMENTATION.md](./INDEXER_REFACTOR_IMPLEMENTATION.md) - Complete implementation guide
- [README-meilisearch.md](./README-meilisearch.md) - Vector search backend
- [AI Provider Interface](../pkg/ai/provider.go) - Interface specification
- [Ollama Documentation](https://github.com/ollama/ollama) - Official Ollama docs
