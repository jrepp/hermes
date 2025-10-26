# OAuth Authentication for Test Automation

## Overview

This guide explains how to authenticate Python test scenarios against Hermes instances that use OAuth/OIDC authentication providers (Google OAuth, Dex, Okta).

## Authentication Methods

### 1. Environment Variable Token (Simplest)

For environments where you already have a valid token:

```bash
# Set the token as an environment variable
export HERMES_AUTH_TOKEN="your-bearer-token-here"

# Run scenarios (will automatically use the token)
cd testing/python
PYTHONPATH=/path/to/hermes/python-client/src:. python3 scenario_basic.py
```

The `TestingConfig` in `config.py` automatically reads `HERMES_AUTH_TOKEN`:

```python
hermes_auth_token: Optional[str] = Field(
    default_factory=lambda: os.getenv("HERMES_AUTH_TOKEN")
)
```

### 2. Dex OIDC with Static Users (Testing)

The testing environment (`./testing/dex-config.yaml`) includes static test users:

**Static Users**:
- **alice@example.com** / password: `password`
- **bob@example.com** / password: `password`
- **admin@example.com** / password: `admin`

**Getting a Token** (manual flow for testing):

```bash
# 1. Start the testing environment
cd testing
make up

# 2. Get auth configuration
curl -s http://localhost:8001/api/v2/web/config | jq '{
  dex_issuer_url,
  dex_client_id,
  dex_redirect_url,
  auth_provider
}'

# Output:
# {
#   "dex_issuer_url": "http://localhost:5558/dex",
#   "dex_client_id": "hermes-testing",
#   "dex_redirect_url": "http://localhost:8001/auth/callback",
#   "auth_provider": "dex"
# }

# 3. Get a token via browser (interactive)
# Navigate to: http://localhost:4201
# Click "Sign In" → Login with alice@example.com / password
# Extract token from browser DevTools:
#   - Open DevTools → Application → Local Storage → http://localhost:4201
#   - Copy the value of 'hermes.authToken' or 'token' key

# 4. Use the token
export HERMES_AUTH_TOKEN="<token-from-browser>"
cd testing/python
PYTHONPATH=../../python-client/src:. python3 scenario_basic.py
```

### 3. OAuth Client Credentials Flow (Automated)

For fully automated testing without user interaction, use OAuth 2.0 Client Credentials flow.

#### 3.1 Dex Client Credentials (Recommended for Testing)

**Update Dex Configuration** (`testing/dex-config.yaml`):

```yaml
# Add client credentials client
staticClients:
  # Existing web client
  - id: hermes-testing
    redirectURIs:
      - 'http://localhost:8001/auth/callback'
      - 'http://localhost:4201/auth/callback'
    name: 'Hermes Testing'
    secret: hermes-testing-secret
    
  # NEW: Service account client for automation
  - id: hermes-automation
    name: 'Hermes Test Automation'
    secret: automation-secret-key-change-me
    # No redirect URIs - this is for client credentials only
```

**Python Client Credentials Helper**:

```python
# testing/python/auth_helper.py
"""OAuth authentication helpers for test automation."""

import httpx
from typing import Optional


def get_client_credentials_token(
    issuer_url: str,
    client_id: str,
    client_secret: str,
    scope: str = "openid email profile",
) -> str:
    """Get OAuth token using client credentials flow.
    
    Args:
        issuer_url: OIDC issuer URL (e.g., http://localhost:5558/dex)
        client_id: OAuth client ID
        client_secret: OAuth client secret
        scope: OAuth scopes
        
    Returns:
        Access token
        
    Raises:
        httpx.HTTPError: If token request fails
    """
    token_url = f"{issuer_url}/token"
    
    response = httpx.post(
        token_url,
        data={
            "grant_type": "client_credentials",
            "client_id": client_id,
            "client_secret": client_secret,
            "scope": scope,
        },
        headers={"Content-Type": "application/x-www-form-urlencoded"},
    )
    response.raise_for_status()
    
    token_data = response.json()
    return token_data["access_token"]


def get_dex_token_for_testing() -> str:
    """Get Dex token for local testing environment.
    
    Returns:
        Access token for hermes-automation client
    """
    return get_client_credentials_token(
        issuer_url="http://localhost:5558/dex",
        client_id="hermes-automation",
        client_secret="automation-secret-key-change-me",
    )
```

**Usage in Scenarios**:

```python
# scenario_basic.py
from auth_helper import get_dex_token_for_testing

# Get token automatically
token = get_dex_token_for_testing()

# Override config
import os
os.environ["HERMES_AUTH_TOKEN"] = token

# Now run scenario
from scenarios import runner
runner.run_basic_scenario()
```

#### 3.2 Google OAuth Service Account (Production)

For production Google Workspace environments, use service account authentication:

**Prerequisites**:
1. Create a service account in Google Cloud Console
2. Enable Domain-Wide Delegation
3. Grant service account access to Google Workspace APIs
4. Download service account JSON key

**Environment Setup**:

```bash
# Set service account credentials
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
export GOOGLE_WORKSPACE_ADMIN_EMAIL="admin@yourdomain.com"
```

**Python Helper**:

```python
# testing/python/auth_helper.py (continued)

import json
from google.oauth2 import service_account
from google.auth.transport import requests


def get_google_access_token(
    service_account_file: str,
    scopes: list[str] = None,
    subject: str = None,
) -> str:
    """Get Google OAuth token using service account.
    
    Args:
        service_account_file: Path to service account JSON
        scopes: OAuth scopes (defaults to Hermes scopes)
        subject: User to impersonate (for domain-wide delegation)
        
    Returns:
        Access token
    """
    if scopes is None:
        scopes = [
            "openid",
            "email",
            "profile",
            "https://www.googleapis.com/auth/drive.readonly",
        ]
    
    credentials = service_account.Credentials.from_service_account_file(
        service_account_file,
        scopes=scopes,
    )
    
    if subject:
        credentials = credentials.with_subject(subject)
    
    # Get token
    credentials.refresh(requests.Request())
    return credentials.token
```

### 4. Skip Authentication (Development Only)

For local development, you can configure Hermes to skip authentication:

**Update `config.hcl`**:

```hcl
# WARNING: Development only, never use in production
server {
  skip_auth = true
}
```

**Restart Hermes**:

```bash
make bin
./hermes server -config=config.hcl
```

**Run scenarios without token**:

```bash
cd testing/python
# No HERMES_AUTH_TOKEN needed
PYTHONPATH=../../python-client/src:. python3 scenario_basic.py
```

## Token Management

### Token Expiration

OAuth tokens expire. Handle token refresh in long-running scenarios:

```python
# testing/python/validation.py (add to HermesValidator)

from datetime import datetime, timedelta
from typing import Optional

class HermesValidator:
    def __init__(self, client: Hermes | None = None) -> None:
        self.client = client or Hermes(...)
        self._token_expires_at: Optional[datetime] = None
        self._token_refresh_callback: Optional[callable] = None
    
    def set_token_refresh(self, callback: callable, expires_in: int = 3600):
        """Set callback to refresh token before expiration.
        
        Args:
            callback: Function that returns new token
            expires_in: Token lifetime in seconds
        """
        self._token_refresh_callback = callback
        self._token_expires_at = datetime.now() + timedelta(seconds=expires_in)
    
    def _ensure_valid_token(self):
        """Refresh token if expired."""
        if self._token_refresh_callback and self._token_expires_at:
            if datetime.now() >= self._token_expires_at:
                new_token = self._token_refresh_callback()
                self.client._async_client._config.auth_token = new_token
                self._token_expires_at = datetime.now() + timedelta(seconds=3600)
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Start testing environment
        run: |
          cd testing
          docker compose up -d
          
      - name: Wait for services
        run: |
          timeout 60 bash -c 'until curl -s http://localhost:8001/health; do sleep 2; done'
          
      - name: Get automation token
        run: |
          # Option 1: Use client credentials
          TOKEN=$(python3 -c "from testing.python.auth_helper import get_dex_token_for_testing; print(get_dex_token_for_testing())")
          echo "HERMES_AUTH_TOKEN=$TOKEN" >> $GITHUB_ENV
          
      - name: Run scenarios
        run: |
          cd testing/python
          PYTHONPATH=../../python-client/src:. python3 scenario_basic.py
          PYTHONPATH=../../python-client/src:. python3 scenario_migration.py
          PYTHONPATH=../../python-client/src:. python3 scenario_multi_author.py
```

### Secrets Management

**Never commit tokens or secrets!**

Use environment variables or secret management:

```bash
# Local: .env file (gitignored)
HERMES_AUTH_TOKEN=your-token-here
HERMES_CLIENT_ID=hermes-automation
HERMES_CLIENT_SECRET=secret-key

# CI: GitHub Secrets
# Settings → Secrets → Actions → New repository secret
# - HERMES_AUTH_TOKEN
# - DEX_CLIENT_SECRET
# - GOOGLE_SERVICE_ACCOUNT_JSON (base64 encoded)
```

## Troubleshooting

### "Authentication failed: Unauthorized"

1. **Check token is set**:
   ```bash
   echo $HERMES_AUTH_TOKEN
   ```

2. **Verify token format**:
   ```bash
   # Token should be a JWT (3 base64 parts separated by dots)
   echo $HERMES_AUTH_TOKEN | awk -F. '{print NF}'
   # Should output: 3
   ```

3. **Check token expiration**:
   ```bash
   # Decode JWT payload (use jwt.io or jwt-cli)
   echo $HERMES_AUTH_TOKEN | cut -d. -f2 | base64 -d | jq '.exp'
   # Compare to current time: date +%s
   ```

4. **Verify Hermes is using correct auth provider**:
   ```bash
   curl -s http://localhost:8001/api/v2/web/config | jq '.auth_provider'
   # Should match your provider: "dex", "google", or "okta"
   ```

### "Event loop is closed"

This is now fixed in `validation.py`. The issue occurred because the sync `Hermes` client uses `asyncio.run()` which closes the event loop after each call. The fix creates a fresh client instance for each retry.

### "Client credentials flow not supported"

Some OIDC providers don't support client credentials for user-facing APIs. Alternatives:

1. **Resource Owner Password Credentials** (less secure):
   ```python
   def get_password_grant_token(issuer_url, client_id, username, password):
       response = httpx.post(
           f"{issuer_url}/token",
           data={
               "grant_type": "password",
               "client_id": client_id,
               "username": username,
               "password": password,
               "scope": "openid email profile",
           }
       )
       return response.json()["access_token"]
   ```

2. **Device Flow** (better UX for CLI):
   - User approves on separate device
   - Good for interactive testing

3. **Pre-generated Long-lived Token** (simplest):
   - Generate token via browser
   - Store in secret manager
   - Rotate periodically

## Best Practices

1. **Use client credentials in CI/CD** - Fully automated, no user interaction
2. **Use short-lived tokens** - Reduce impact of token leakage
3. **Rotate secrets regularly** - Change client secrets monthly
4. **Log authentication attempts** - Monitor for unauthorized access
5. **Use environment variables** - Never hardcode tokens in code
6. **Test token refresh** - Ensure long-running scenarios handle expiration
7. **Separate test accounts** - Don't use production credentials in tests

## Examples

### Complete Automated Scenario

```python
#!/usr/bin/env python3
"""Fully automated test scenario with client credentials auth."""

import os
import sys
from pathlib import Path

# Add python-client to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "python-client/src"))
sys.path.insert(0, str(Path(__file__).parent))

from auth_helper import get_dex_token_for_testing
from scenarios import runner

def main():
    """Run automated test scenario."""
    # Get token automatically
    try:
        token = get_dex_token_for_testing()
        os.environ["HERMES_AUTH_TOKEN"] = token
        print("✓ Obtained authentication token")
    except Exception as e:
        print(f"❌ Failed to get auth token: {e}")
        print("Make sure Dex is running: cd testing && make up")
        sys.exit(1)
    
    # Run scenario
    try:
        results = runner.run_basic_scenario(count=20, wait_for_indexing=True)
        print(f"\n✅ Scenario completed successfully: {results}")
    except Exception as e:
        print(f"\n❌ Scenario failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
```

Save as `testing/python/scenario_automated.py` and run:

```bash
cd testing
make up  # Start services

cd python
python3 scenario_automated.py
```

## References

- [OAuth 2.0 Client Credentials](https://oauth.net/2/grant-types/client-credentials/)
- [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html)
- [Dex Documentation](https://dexidp.io/docs/)
- [Google OAuth Service Accounts](https://developers.google.com/identity/protocols/oauth2/service-account)
- [Okta Client Credentials](https://developer.okta.com/docs/guides/implement-grant-type/clientcreds/main/)
