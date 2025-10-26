"""OAuth authentication helpers for test automation.

Provides utilities for obtaining OAuth tokens via client credentials,
service accounts, and other automated flows for testing scenarios.
"""

from __future__ import annotations

import json
import os

import httpx


class AuthError(Exception):
    """Authentication error."""

    pass


def get_client_credentials_token(
    issuer_url: str,
    client_id: str,
    client_secret: str,
    scope: str = "openid email profile",
    timeout: int = 30,
) -> str:
    """Get OAuth token using client credentials flow.

    This is the recommended method for automated testing as it doesn't
    require user interaction or browser-based login flows.

    Args:
        issuer_url: OIDC issuer URL (e.g., http://localhost:5558/dex)
        client_id: OAuth client ID
        client_secret: OAuth client secret
        scope: OAuth scopes (space-separated)
        timeout: Request timeout in seconds

    Returns:
        Access token (JWT)

    Raises:
        AuthError: If token request fails
        httpx.HTTPError: If HTTP request fails

    Example:
        >>> token = get_client_credentials_token(
        ...     issuer_url="http://localhost:5558/dex",
        ...     client_id="hermes-automation",
        ...     client_secret="automation-secret-key",
        ... )
        >>> os.environ["HERMES_AUTH_TOKEN"] = token
    """
    token_url = f"{issuer_url.rstrip('/')}/token"

    try:
        response = httpx.post(
            token_url,
            data={
                "grant_type": "client_credentials",
                "client_id": client_id,
                "client_secret": client_secret,
                "scope": scope,
            },
            headers={"Content-Type": "application/x-www-form-urlencoded"},
            timeout=timeout,
        )
        response.raise_for_status()
    except httpx.HTTPError as e:
        raise AuthError(
            f"Failed to get token from {token_url}: {e}\n"
            f"Make sure the auth provider is running and client credentials are correct."
        ) from e

    try:
        token_data = response.json()
        return token_data["access_token"]
    except (KeyError, json.JSONDecodeError) as e:
        raise AuthError(
            f"Invalid token response from {token_url}: {response.text}"
        ) from e


def get_password_grant_token(
    issuer_url: str,
    client_id: str,
    username: str,
    password: str,
    client_secret: str | None = None,
    scope: str = "openid email profile",
    timeout: int = 30,
) -> str:
    """Get OAuth token using resource owner password credentials flow.

    WARNING: This flow is less secure than client credentials and should
    only be used in testing environments with test accounts.

    Args:
        issuer_url: OIDC issuer URL
        client_id: OAuth client ID
        username: User's email/username
        password: User's password
        client_secret: Optional client secret
        scope: OAuth scopes
        timeout: Request timeout in seconds

    Returns:
        Access token (JWT)

    Raises:
        AuthError: If authentication fails

    Example:
        >>> token = get_password_grant_token(
        ...     issuer_url="http://localhost:5558/dex",
        ...     client_id="hermes-testing",
        ...     username="alice@example.com",
        ...     password="password",
        ... )
    """
    token_url = f"{issuer_url.rstrip('/')}/token"

    data = {
        "grant_type": "password",
        "client_id": client_id,
        "username": username,
        "password": password,
        "scope": scope,
    }

    if client_secret:
        data["client_secret"] = client_secret

    try:
        response = httpx.post(
            token_url,
            data=data,
            headers={"Content-Type": "application/x-www-form-urlencoded"},
            timeout=timeout,
        )
        response.raise_for_status()
    except httpx.HTTPError as e:
        raise AuthError(
            f"Failed to authenticate {username} at {token_url}: {e}"
        ) from e

    try:
        token_data = response.json()
        return token_data["access_token"]
    except (KeyError, json.JSONDecodeError) as e:
        raise AuthError(
            f"Invalid token response from {token_url}: {response.text}"
        ) from e


def get_dex_token_for_testing(
    issuer_url: str | None = None,
    client_id: str | None = None,
    username: str | None = None,
    password: str | None = None,
) -> str:
    """Get Dex token for local testing environment.

    Uses password grant with static test users since Dex doesn't support
    client_credentials grant type.

    Environment Variables:
        DEX_ISSUER_URL: Dex issuer URL (default: http://localhost:5558/dex)
        DEX_CLIENT_ID: OAuth client ID (default: hermes-testing)
        DEX_TEST_USERNAME: Test user email (default: test@hermes.local)
        DEX_TEST_PASSWORD: Test user password (default: password)

    Args:
        issuer_url: Override Dex issuer URL
        client_id: Override client ID
        username: Override test username
        password: Override test password

    Returns:
        Access token for local testing

    Raises:
        AuthError: If authentication fails

    Example:
        >>> # Using defaults (test@hermes.local)
        >>> token = get_dex_token_for_testing()
        >>>
        >>> # With specific user
        >>> token = get_dex_token_for_testing(
        ...     username="admin@hermes.local",
        ...     password="password",
        ... )
    """
    return get_password_grant_token(
        issuer_url=issuer_url or os.getenv("DEX_ISSUER_URL", "http://localhost:5558/dex"),
        client_id=client_id or os.getenv("DEX_CLIENT_ID", "hermes-testing"),
        username=username or os.getenv("DEX_TEST_USERNAME", "test@hermes.local"),
        password=password or os.getenv("DEX_TEST_PASSWORD", "password"),
    )


def get_dex_user_token(
    username: str = "alice@example.com",
    password: str = "password",
    issuer_url: str | None = None,
    client_id: str | None = None,
) -> str:
    """Get Dex token for static test user (password grant).

    Default test users in testing/dex-config.yaml:
    - alice@example.com / password
    - bob@example.com / password
    - admin@example.com / admin

    Args:
        username: Test user email
        password: Test user password
        issuer_url: Override Dex issuer URL
        client_id: Override client ID

    Returns:
        Access token for test user

    Raises:
        AuthError: If authentication fails

    Example:
        >>> # Authenticate as Alice
        >>> token = get_dex_user_token("alice@example.com", "password")
        >>>
        >>> # Authenticate as Bob
        >>> token = get_dex_user_token("bob@example.com", "password")
    """
    return get_password_grant_token(
        issuer_url=issuer_url or os.getenv("DEX_ISSUER_URL", "http://localhost:5558/dex"),
        client_id=client_id or os.getenv("DEX_CLIENT_ID", "hermes-testing"),
        username=username,
        password=password,
        client_secret=os.getenv("DEX_CLIENT_SECRET"),  # Optional
    )


def get_google_service_account_token(
    service_account_file: str | None = None,
    scopes: list[str] | None = None,
    subject: str | None = None,
) -> str:
    """Get Google OAuth token using service account.

    Requires google-auth package: pip install google-auth

    Environment Variables:
        GOOGLE_APPLICATION_CREDENTIALS: Path to service account JSON
        GOOGLE_WORKSPACE_ADMIN_EMAIL: Admin email for domain-wide delegation

    Args:
        service_account_file: Path to service account JSON (uses env var if None)
        scopes: OAuth scopes (defaults to Hermes-required scopes)
        subject: User email to impersonate (for domain-wide delegation)

    Returns:
        Access token

    Raises:
        AuthError: If authentication fails
        ImportError: If google-auth not installed

    Example:
        >>> token = get_google_service_account_token(
        ...     service_account_file="/path/to/credentials.json",
        ...     subject="admin@example.com",
        ... )
    """
    try:
        from google.auth.transport import requests
        from google.oauth2 import service_account
    except ImportError as e:
        raise ImportError(
            "google-auth required for service account authentication.\n"
            "Install with: pip install google-auth"
        ) from e

    sa_file = service_account_file or os.getenv("GOOGLE_APPLICATION_CREDENTIALS")
    if not sa_file:
        raise AuthError(
            "Service account file required. Set GOOGLE_APPLICATION_CREDENTIALS "
            "or pass service_account_file parameter."
        )

    if scopes is None:
        scopes = [
            "openid",
            "email",
            "profile",
            "https://www.googleapis.com/auth/drive.readonly",
        ]

    try:
        credentials = service_account.Credentials.from_service_account_file(
            sa_file,
            scopes=scopes,
        )

        if subject:
            credentials = credentials.with_subject(subject)
        elif os.getenv("GOOGLE_WORKSPACE_ADMIN_EMAIL"):
            credentials = credentials.with_subject(os.getenv("GOOGLE_WORKSPACE_ADMIN_EMAIL"))

        # Get token
        credentials.refresh(requests.Request())
        return credentials.token

    except Exception as e:
        raise AuthError(
            f"Failed to get Google service account token: {e}"
        ) from e


def validate_token(token: str) -> dict:
    """Validate JWT token format and extract claims.

    Does NOT verify signature - only decodes and validates structure.

    Args:
        token: JWT token string

    Returns:
        Dictionary with token claims (payload)

    Raises:
        AuthError: If token format is invalid

    Example:
        >>> claims = validate_token(token)
        >>> print(f"Token expires: {claims.get('exp')}")
        >>> print(f"Subject: {claims.get('sub')}")
    """
    import base64

    try:
        # JWT has 3 parts: header.payload.signature
        parts = token.split(".")
        if len(parts) != 3:
            raise AuthError(
                f"Invalid JWT format: expected 3 parts, got {len(parts)}"
            )

        # Decode payload (add padding if needed)
        payload = parts[1]
        payload += "=" * (4 - len(payload) % 4)  # Add padding
        payload_bytes = base64.urlsafe_b64decode(payload)
        claims = json.loads(payload_bytes)

        return claims

    except Exception as e:
        raise AuthError(f"Failed to decode JWT token: {e}") from e
