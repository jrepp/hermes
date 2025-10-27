# Ollama Integration for Setup Wizard

**Status**: ✅ Implemented  
**Branch**: `jrepp/dev-tidy`  
**Related**: RFC-083, Setup Wizard  

## Overview

This document describes the Ollama AI integration added to the Hermes setup wizard. Users can now configure local AI summarization during the initial setup flow with connection validation.

## Features

- ✅ **Ollama URL Configuration**: Configure Ollama API endpoint (default: http://localhost:11434)
- ✅ **Model Selection**: Choose summarization model (default: llama3.2)
- ✅ **Connection Validation**: Test Ollama connection and verify model availability
- ✅ **Real-time Feedback**: Visual success/error indicators for validation
- ✅ **Optional Configuration**: Ollama setup is completely optional

## Implementation Details

### Backend Changes

#### 1. Config Structure (`internal/config/config.go`)

Added new `Ollama` configuration type:

```go
// Ollama configures Hermes to work with Ollama for local AI summarization.
type Ollama struct {
    // URL is the Ollama API URL (e.g., "http://localhost:11434").
    URL string `hcl:"url"`

    // SummarizeModel is the model for document summarization (e.g., "llama3.2").
    SummarizeModel string `hcl:"summarize_model,optional"`

    // EmbeddingModel is the model for vector embeddings (e.g., "nomic-embed-text").
    EmbeddingModel string `hcl:"embedding_model,optional"`
}
```

Added to main `Config` struct:
```go
// Ollama configures Hermes to work with Ollama for local AI summarization.
Ollama *Ollama `hcl:"ollama,block"`
```

#### 2. Setup API (`internal/api/v2/setup.go`)

**Request Structure**:
```go
type SetupConfigRequest struct {
    WorkspacePath string `json:"workspace_path"`
    UpstreamURL   string `json:"upstream_url,omitempty"`
    OllamaURL     string `json:"ollama_url,omitempty"`
    OllamaModel   string `json:"ollama_model,omitempty"`
}

type OllamaValidationRequest struct {
    URL   string `json:"url"`
    Model string `json:"model"`
}

type OllamaValidationResponse struct {
    Valid   bool   `json:"valid"`
    Message string `json:"message"`
    Version string `json:"version,omitempty"`
}
```

**Validation Handler** (`OllamaValidateHandler`):
- Endpoint: `POST /api/v2/setup/validate-ollama`
- Validates Ollama connection by calling `/api/version`
- Checks model availability via `/api/tags`
- Returns validation status and helpful error messages

**Config Generation**:
```go
func generateConfigFile(workspacePath, upstreamURL, ollamaURL, ollamaModel string) error {
    cfg := config.GenerateSimplifiedConfig(workspacePath)
    
    if ollamaURL != "" {
        cfg.Ollama = &config.Ollama{
            URL:            ollamaURL,
            SummarizeModel: ollamaModel,
        }
    }
    
    return config.WriteConfig(cfg, "config.hcl")
}
```

#### 3. Endpoint Registration (`internal/cmd/commands/server/server.go`)

Added to both Algolia and non-Algolia endpoint lists:
```go
endpoint{"/api/v2/setup/validate-ollama", apiv2.OllamaValidateHandler(c.Log)}
```

### Frontend Changes

#### 1. Component Logic (`web/app/components/setup-wizard.ts`)

**New Tracked Properties**:
```typescript
@tracked ollamaURL = 'http://localhost:11434';
@tracked ollamaModel = 'llama3.2';
@tracked isValidatingOllama = false;
@tracked ollamaValidationMessage = '';
@tracked ollamaValidationSuccess = false;
```

**New Actions**:
```typescript
@action updateOllamaURL(event: Event)
@action updateOllamaModel(event: Event)
@action async validateOllama(event: Event)
```

**Updated Submit**:
```typescript
body: JSON.stringify({
    workspace_path: this.workspacePath,
    upstream_url: this.upstreamURL,
    ollama_url: this.ollamaURL,
    ollama_model: this.ollamaModel,
})
```

#### 2. Template UI (`web/app/components/setup-wizard.hbs`)

Added new section after Upstream Server:

```handlebars
{{! Ollama AI Section }}
<div class="border-t pt-6">
  <h3 class="text-lg font-medium text-gray-900 mb-4">
    Local AI with Ollama (Optional)
  </h3>
  
  <!-- URL Input -->
  <input id="ollama-url" type="url" value={{this.ollamaURL}} ... />
  
  <!-- Model Input -->
  <input id="ollama-model" type="text" value={{this.ollamaModel}} ... />
  
  <!-- Validation Button -->
  <button type="button" {{on "click" this.validateOllama}}>
    Test Connection
  </button>
  
  <!-- Validation Feedback -->
  {{#if this.ollamaValidationMessage}}
    <div class="{{if this.ollamaValidationSuccess 'bg-green-50' 'bg-yellow-50'}}">
      {{this.ollamaValidationMessage}}
    </div>
  {{/if}}
</div>
```

**UI Features**:
- Default values (localhost:11434, llama3.2)
- Inline help text with `ollama pull` command
- Test Connection button with loading spinner
- Success/warning colored feedback messages
- Link to ollama.ai for installation

## User Flow

1. **Initial Setup**: User starts Hermes without config.hcl
2. **Setup Wizard**: Redirects to `/setup` route
3. **Ollama Section**: User can optionally configure:
   - Ollama URL (default: http://localhost:11434)
   - Model name (default: llama3.2)
4. **Validation**: User clicks "Test Connection"
   - Success: Shows "Connected to Ollama {version}" in green
   - Model missing: Shows "Model 'llama3.2' not found. Run: ollama pull llama3.2"
   - Connection failed: Shows error message with details
5. **Submit**: Configuration saved to config.hcl with Ollama block

## Generated Config Example

If user provides Ollama configuration, the generated `config.hcl` includes:

```hcl
ollama {
  url             = "http://localhost:11434"
  summarize_model = "llama3.2"
}
```

## API Endpoints

### POST /api/v2/setup/validate-ollama

**Request**:
```json
{
  "url": "http://localhost:11434",
  "model": "llama3.2"
}
```

**Response (Success)**:
```json
{
  "valid": true,
  "message": "Connected to Ollama 0.1.0",
  "version": "0.1.0"
}
```

**Response (Model Not Found)**:
```json
{
  "valid": false,
  "message": "Model 'llama3.2' not found. Run: ollama pull llama3.2",
  "version": "0.1.0"
}
```

**Response (Connection Failed)**:
```json
{
  "valid": false,
  "message": "Could not connect to Ollama at http://localhost:11434: connection refused"
}
```

## Validation Logic

The validation handler performs these checks:

1. **URL Required**: Returns error if URL is empty
2. **Connection Test**: Calls `GET {url}/api/version`
   - Success: Extracts version number
   - Failure: Returns connection error
3. **Model Verification** (if model specified): Calls `GET {url}/api/tags`
   - Parses list of available models
   - Checks if requested model exists (partial match)
   - Returns helpful `ollama pull` command if missing

## Testing

### Manual Testing Checklist

- [ ] Start Hermes without config.hcl
- [ ] Verify redirect to /setup
- [ ] Fill in workspace path
- [ ] Enter Ollama URL and model
- [ ] Click "Test Connection" (without Ollama running)
  - Should show connection error
- [ ] Start Ollama: `ollama serve`
- [ ] Click "Test Connection" (with Ollama running but no model)
  - Should show "Model not found" with pull command
- [ ] Pull model: `ollama pull llama3.2`
- [ ] Click "Test Connection" again
  - Should show success message with version
- [ ] Submit form
- [ ] Verify `config.hcl` contains Ollama block
- [ ] Verify redirect to main app

### Automated Testing

**Backend**:
```bash
# Test validation endpoint
curl -X POST http://localhost:8000/api/v2/setup/validate-ollama \
  -H "Content-Type: application/json" \
  -d '{"url": "http://localhost:11434", "model": "llama3.2"}'
```

**Build Validation**:
```bash
make bin          # Backend compilation
cd web && yarn build  # Frontend build
make build        # Full project build
```

## Dependencies

**Ollama Package**: `pkg/ai/ollama/provider.go`
- Config: `BaseURL`, `SummarizeModel`, `EmbeddingModel`
- Used by: Document summarization features (future integration)

**External**:
- Ollama API (http://localhost:11434)
- Models: llama3.2, llama3.1, nomic-embed-text, mxbai-embed-large

## Future Enhancements

- [ ] Model dropdown populated from `/api/tags`
- [ ] Embedding model configuration
- [ ] Test summarization before submitting
- [ ] Auto-detect Ollama installation
- [ ] Show available disk space for models
- [ ] One-click model installation

## Related Documentation

- [docs-internal/README-ollama.md](README-ollama.md) - Ollama provider documentation
- [docs-internal/SETUP_WIZARD_GUIDE.md](SETUP_WIZARD_GUIDE.md) - Setup wizard overview
- [RFC-083](rfc/rfc-083-simplified-local-mode.md) - Simplified local mode RFC

## Files Modified

**Backend**:
- `internal/config/config.go` - Added Ollama config type
- `internal/api/v2/setup.go` - Added validation handler and config generation
- `internal/cmd/commands/server/server.go` - Registered validation endpoint

**Frontend**:
- `web/app/components/setup-wizard.ts` - Added Ollama properties and validation action
- `web/app/components/setup-wizard.hbs` - Added Ollama UI section

## Commit Message

```
feat(setup): add Ollama AI configuration to setup wizard

**Prompt Used**:
"update the local config to also allow for a local configuration step for
ollama that validates the local ollama is working"

**AI Implementation Summary**:
- Added Ollama config type to internal/config/config.go (URL, SummarizeModel,
  EmbeddingModel fields)
- Implemented OllamaValidateHandler in internal/api/v2/setup.go
  - Validates Ollama connection via /api/version endpoint
  - Checks model availability via /api/tags endpoint
  - Returns detailed error messages with helpful ollama pull commands
- Updated generateConfigFile to create Ollama block when URL provided
- Registered /api/v2/setup/validate-ollama endpoint in server.go
- Extended setup wizard component with Ollama fields (URL, model)
- Added validateOllama action with loading states and success/error feedback
- Created Ollama UI section in setup template with:
  - URL and model inputs with sensible defaults (localhost:11434, llama3.2)
  - Test Connection button with spinner animation
  - Color-coded validation feedback (green success, yellow warning)
  - Inline help with ollama pull command and installation link

**Human Review Notes**: None - AI-generated code compiled and built successfully

**Verification**:
- Backend: `make bin` - ✅ Success
- Frontend: `cd web && yarn build` - ✅ Success (expected env var warnings)
- Full build: `make build` - ✅ Success
- No compilation errors, no TypeScript errors
```
