# Semantic Search API Documentation
## RFC-088 Semantic and Hybrid Search Endpoints

**Version**: 2.0
**Base URL**: `/api/v2`
**Authentication**: Required (Bearer token or session)

---

## Overview

The Semantic Search API provides vector-based similarity search, hybrid search (combining keyword and semantic), and related document discovery. These endpoints enable finding documents based on meaning rather than just keyword matching.

**Key Features**:
- **Semantic Search**: Find documents by meaning using OpenAI embeddings
- **Hybrid Search**: Combine keyword (Meilisearch) and semantic search for best results
- **Similar Documents**: Discover related documents based on content similarity
- **Filtering**: Filter by document IDs, types, or similarity threshold

---

## Endpoints

### 1. Semantic Search

Find documents using vector similarity search.

**Endpoint**: `POST /api/v2/search/semantic`

**Request Body**:
```json
{
  "query": "machine learning algorithms",
  "limit": 10,
  "minSimilarity": 0.7,
  "documentIds": ["doc1", "doc2"],
  "documentTypes": ["RFC", "PRD"]
}
```

**Request Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query text |
| `limit` | integer | No | Max results (default: 10, max: 100) |
| `minSimilarity` | float | No | Minimum similarity score 0-1 (default: 0) |
| `documentIds` | array | No | Filter to specific document IDs |
| `documentTypes` | array | No | Filter to specific document types |

**Response**:
```json
{
  "results": [
    {
      "documentId": "doc123",
      "documentUuid": "550e8400-e29b-41d4-a716-446655440000",
      "title": "Machine Learning Best Practices",
      "excerpt": "This document covers advanced ML algorithms...",
      "similarity": 0.92,
      "chunkIndex": 0,
      "chunkText": "Machine learning algorithms are..."
    }
  ],
  "query": "machine learning algorithms",
  "count": 1
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `documentId` | string | Document identifier |
| `documentUuid` | string | Document UUID |
| `title` | string | Document title |
| `excerpt` | string | Relevant excerpt |
| `similarity` | float | Cosine similarity score (0-1, higher is better) |
| `chunkIndex` | integer | Chunk index if document was chunked |
| `chunkText` | string | Text of the matched chunk |

**Example - cURL**:
```bash
curl -X POST https://hermes.example.com/api/v2/search/semantic \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "kubernetes deployment strategies",
    "limit": 5,
    "minSimilarity": 0.75
  }'
```

**Example - JavaScript/Node.js**:
```javascript
const response = await fetch('https://hermes.example.com/api/v2/search/semantic', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    query: 'kubernetes deployment strategies',
    limit: 5,
    minSimilarity: 0.75,
  }),
});

const data = await response.json();
console.log(`Found ${data.count} documents`);
data.results.forEach(result => {
  console.log(`${result.title} (similarity: ${result.similarity})`);
});
```

**Example - Python**:
```python
import requests

response = requests.post(
    'https://hermes.example.com/api/v2/search/semantic',
    headers={
        'Authorization': f'Bearer {token}',
        'Content-Type': 'application/json',
    },
    json={
        'query': 'kubernetes deployment strategies',
        'limit': 5,
        'minSimilarity': 0.75,
    }
)

data = response.json()
print(f"Found {data['count']} documents")
for result in data['results']:
    print(f"{result['title']} (similarity: {result['similarity']})")
```

**Example - Go**:
```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type SemanticSearchRequest struct {
    Query         string  `json:"query"`
    Limit         int     `json:"limit,omitempty"`
    MinSimilarity float64 `json:"minSimilarity,omitempty"`
}

type SemanticSearchResponse struct {
    Results []struct {
        DocumentID string  `json:"documentId"`
        Title      string  `json:"title"`
        Similarity float64 `json:"similarity"`
    } `json:"results"`
    Count int `json:"count"`
}

func semanticSearch(token, query string) (*SemanticSearchResponse, error) {
    req := SemanticSearchRequest{
        Query:         query,
        Limit:         5,
        MinSimilarity: 0.75,
    }

    body, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST",
        "https://hermes.example.com/api/v2/search/semantic",
        bytes.NewBuffer(body))

    httpReq.Header.Set("Authorization", "Bearer "+token)
    httpReq.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result SemanticSearchResponse
    json.NewDecoder(resp.Body).Decode(&result)

    return &result, nil
}
```

**Status Codes**:
- `200 OK` - Success
- `400 Bad Request` - Invalid request (empty query, invalid limit)
- `401 Unauthorized` - Missing or invalid authentication
- `503 Service Unavailable` - Semantic search not configured

---

### 2. Hybrid Search

Combine keyword and semantic search for optimal results.

**Endpoint**: `POST /api/v2/search/hybrid`

**Request Body**:
```json
{
  "query": "database performance optimization",
  "limit": 10,
  "weights": {
    "keywordWeight": 0.4,
    "semanticWeight": 0.4,
    "boostBoth": 0.2
  }
}
```

**Request Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query text |
| `limit` | integer | No | Max results (default: 10, max: 100) |
| `weights` | object | No | Search weight configuration |
| `weights.keywordWeight` | float | No | Keyword search weight (default: 0.4) |
| `weights.semanticWeight` | float | No | Semantic search weight (default: 0.4) |
| `weights.boostBoth` | float | No | Bonus for appearing in both (default: 0.2) |

**Weight Presets**:
- **Balanced** (default): `{keywordWeight: 0.4, semanticWeight: 0.4, boostBoth: 0.2}`
- **Keyword-focused**: `{keywordWeight: 0.7, semanticWeight: 0.2, boostBoth: 0.1}`
- **Semantic-focused**: `{keywordWeight: 0.2, semanticWeight: 0.7, boostBoth: 0.1}`

**Response**:
```json
{
  "results": [
    {
      "documentId": "doc456",
      "documentUuid": "660e8400-e29b-41d4-a716-446655440000",
      "title": "PostgreSQL Performance Tuning",
      "type": "RFC",
      "keywordScore": 0.85,
      "semanticScore": 0.78,
      "hybridScore": 0.82,
      "matchedInBoth": true
    }
  ],
  "query": "database performance optimization",
  "count": 1
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `keywordScore` | float | Meilisearch relevance score (0-1) |
| `semanticScore` | float | Vector similarity score (0-1) |
| `hybridScore` | float | Combined weighted score (0-1) |
| `matchedInBoth` | boolean | Document appeared in both searches |

**Example - cURL**:
```bash
curl -X POST https://hermes.example.com/api/v2/search/hybrid \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "API rate limiting strategies",
    "limit": 10,
    "weights": {
      "keywordWeight": 0.5,
      "semanticWeight": 0.5,
      "boostBoth": 0
    }
  }'
```

**Example - JavaScript**:
```javascript
// Balanced search (default)
const balanced = await hybridSearch('API design patterns', {
  keywordWeight: 0.4,
  semanticWeight: 0.4,
  boostBoth: 0.2
});

// Keyword-focused (for exact term matching)
const keywordFocused = await hybridSearch('RFC-088', {
  keywordWeight: 0.7,
  semanticWeight: 0.2,
  boostBoth: 0.1
});

// Semantic-focused (for conceptual search)
const semanticFocused = await hybridSearch('how to scale microservices', {
  keywordWeight: 0.2,
  semanticWeight: 0.7,
  boostBoth: 0.1
});

async function hybridSearch(query, weights) {
  const response = await fetch('/api/v2/search/hybrid', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ query, limit: 10, weights }),
  });
  return response.json();
}
```

**When to Use Hybrid Search**:
- **General purpose search**: Use balanced weights
- **Looking for specific terms**: Use keyword-focused weights
- **Looking for concepts**: Use semantic-focused weights
- **Acronyms/codes**: Use keyword-focused (e.g., "RFC-088", "HTTP-500")
- **Natural language questions**: Use semantic-focused (e.g., "how do I...")

**Status Codes**:
- `200 OK` - Success (may include partial results if one search fails)
- `400 Bad Request` - Invalid request
- `401 Unauthorized` - Missing or invalid authentication
- `503 Service Unavailable` - Both searches unavailable

---

### 3. Similar Documents

Find documents similar to a given document.

**Endpoint**: `GET /api/v2/documents/{documentId}/similar`

**URL Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `documentId` | string | Yes | Source document ID (in URL path) |

**Query Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limit` | integer | No | Max results (default: 10, max: 100) |

**Response**:
```json
{
  "results": [
    {
      "documentId": "doc789",
      "documentUuid": "770e8400-e29b-41d4-a716-446655440000",
      "title": "Related Document Title",
      "excerpt": "This document discusses similar topics...",
      "similarity": 0.88
    }
  ],
  "sourceDocumentId": "doc123",
  "count": 1
}
```

**Example - cURL**:
```bash
curl -X GET "https://hermes.example.com/api/v2/documents/doc123/similar?limit=5" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Example - JavaScript**:
```javascript
async function findSimilarDocuments(documentId, limit = 10) {
  const response = await fetch(
    `/api/v2/documents/${documentId}/similar?limit=${limit}`,
    {
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    }
  );

  const data = await response.json();
  return data.results;
}

// Usage
const similar = await findSimilarDocuments('doc123', 5);
similar.forEach(doc => {
  console.log(`${doc.title} (${(doc.similarity * 100).toFixed(1)}% similar)`);
});
```

**Use Cases**:
- "Related documents" sidebar on document view pages
- "You might also be interested in..." recommendations
- Document clustering and organization
- Duplicate/similar document detection

**Status Codes**:
- `200 OK` - Success
- `401 Unauthorized` - Missing or invalid authentication
- `404 Not Found` - Source document not found or has no embeddings
- `503 Service Unavailable` - Semantic search not configured

---

## Error Handling

### Error Response Format

```json
{
  "error": "Error message description",
  "code": "ERROR_CODE",
  "details": {
    "field": "Additional context"
  }
}
```

### Common Errors

**Empty Query**:
```json
{
  "error": "query cannot be empty",
  "code": "INVALID_REQUEST"
}
```

**Invalid Limit**:
```json
{
  "error": "limit must be between 1 and 100",
  "code": "INVALID_REQUEST"
}
```

**Service Unavailable**:
```json
{
  "error": "semantic search not configured",
  "code": "SERVICE_UNAVAILABLE"
}
```

### Error Handling Example

```javascript
async function searchWithErrorHandling(query) {
  try {
    const response = await fetch('/api/v2/search/semantic', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ query, limit: 10 }),
    });

    if (!response.ok) {
      const error = await response.json();

      switch (response.status) {
        case 400:
          console.error('Invalid request:', error.error);
          break;
        case 401:
          console.error('Authentication required');
          // Redirect to login
          break;
        case 503:
          console.error('Search service unavailable:', error.error);
          // Fall back to keyword search
          break;
        default:
          console.error('Unexpected error:', error);
      }

      return null;
    }

    return await response.json();
  } catch (err) {
    console.error('Network error:', err);
    return null;
  }
}
```

---

## Best Practices

### 1. Query Optimization

**Good Queries**:
- Natural language: "How do I deploy a microservice?"
- Specific concepts: "database indexing strategies"
- Domain terms: "Kubernetes pod autoscaling"

**Poor Queries**:
- Too short: "db" (not enough context)
- Too generic: "things" (no semantic meaning)
- Special characters only: "!!!" (no semantic content)

### 2. Limit Configuration

- **Default (10)**: Good for most searches
- **Small (5)**: For "top results" or preview displays
- **Large (50-100)**: For comprehensive result sets or analysis

### 3. Similarity Threshold

- **High (0.8-1.0)**: Very similar documents only (strict matching)
- **Medium (0.6-0.8)**: Moderately similar (recommended default)
- **Low (0.4-0.6)**: Loosely related (broader exploration)
- **No threshold (0)**: All results sorted by relevance

### 4. Hybrid Search Weights

Choose weights based on query type:

**Keyword-focused (0.7/0.2/0.1)**:
- Exact terms: "RFC-088", "CVE-2023-1234"
- Codes/IDs: "DOC-456", "TICKET-789"
- Acronyms: "API", "SLA", "RBAC"

**Balanced (0.4/0.4/0.2)**:
- General search: "database performance"
- Mixed queries: "Kubernetes best practices"
- Default choice when unsure

**Semantic-focused (0.2/0.7/0.1)**:
- Questions: "How do I scale my application?"
- Concepts: "authentication security patterns"
- Natural language: "ways to improve code quality"

### 5. Caching

Consider caching search results for:
- Common queries
- Static document collections
- Expensive searches (high limit)

```javascript
const cache = new Map();
const CACHE_TTL = 5 * 60 * 1000; // 5 minutes

async function cachedSearch(query, options) {
  const cacheKey = JSON.stringify({ query, options });
  const cached = cache.get(cacheKey);

  if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
    return cached.results;
  }

  const results = await semanticSearch(query, options);
  cache.set(cacheKey, { results, timestamp: Date.now() });

  return results;
}
```

### 6. Performance

- Use appropriate `limit` values (don't fetch more than needed)
- Set `minSimilarity` to filter low-relevance results
- Use `documentIds` filter when searching specific documents
- Consider hybrid search for better relevance

---

## Rate Limiting

**Current Limits** (may vary by deployment):
- 100 requests per minute per user
- 1000 requests per hour per user

**Headers**:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1699564800
```

**Rate Limit Exceeded**:
```json
{
  "error": "rate limit exceeded",
  "code": "RATE_LIMIT_EXCEEDED",
  "retryAfter": 60
}
```

---

## Monitoring and Metrics

### Performance Metrics

- **p50 latency**: ~30-50ms (typical)
- **p95 latency**: ~100-150ms
- **p99 latency**: ~200-300ms

### Health Check

Check service availability:
```bash
curl https://hermes.example.com/health
```

---

## Additional Resources

- [Performance Tuning Guide](../deployment/performance-tuning.md)
- [Best Practices](../guides/best-practices.md)
- [Troubleshooting](../guides/troubleshooting.md)
- [Production Deployment](../deployment/production-checklist.md)

---

*Last Updated: November 15, 2025*
*API Version: 2.0*
*RFC-088 Implementation*
