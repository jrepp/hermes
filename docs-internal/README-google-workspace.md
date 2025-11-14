# Google Workspace Setup Guide

This guide covers setting up Google Workspace integration for Hermes. Choose the setup that matches your needs:

- **Quick Start (Development/Testing)**: Use your personal Google Workspace account for local testing
- **Advanced Setup (Production)**: Configure service accounts with domain-wide delegation for production deployments

## Overview

Google Workspace integration provides:
- **Authentication**: User login via Google OAuth
- **Document Storage**: Google Docs for document creation and editing
- **User Directory**: People API for user lookup and collaboration
- **Email Notifications**: Gmail API for document notifications
- **Group Approvals** (optional): Admin SDK API for Google Groups as approvers

---

## Quick Start: Development/Testing Setup

This setup is perfect for:
- Local development and testing
- Personal testing with your organization's Google Workspace account
- Trying out Hermes features without admin access

**Time Required**: ~15-20 minutes

### Prerequisites

- A Google Workspace account (e.g., `yourname@company.com`)
- Access to create a Google Cloud project (free tier works fine)
- Owner access to create test folders in Google Drive

### Step 1: Create Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click **"New Project"** or use an existing project
3. Give it a name like `hermes-dev-test`
4. Note your Project ID

### Step 2: Enable Required APIs

1. In your Google Cloud project, go to **"APIs & Services" → "Library"**
2. Search for and **enable** each of these APIs (click "Enable" for each):
   - **Google Docs API** (required)
   - **Google Drive API** (required)
   - **Gmail API** (required)
   - **People API** (required)
   - **Admin SDK API** (optional - only if you want to test group approvals)

### Step 3: Configure OAuth Consent Screen

1. Go to **"APIs & Services" → "OAuth consent screen"**
2. Choose **"Internal"** (if available for your org) or **"External"**
3. Fill in the required fields:
   - **App name**: `Hermes Dev`
   - **User support email**: Your email
   - **Developer contact**: Your email
4. Under **"Authorized domains"**, add your organization's domain (e.g., `company.com`)
5. Click **"Save and Continue"**

### Step 4: Add OAuth Scopes

1. On the **"Scopes"** page, click **"Add or Remove Scopes"**
2. Add these scopes (search and check each one):
   ```
   https://www.googleapis.com/auth/admin.directory.group.readonly
   https://www.googleapis.com/auth/directory.readonly
   https://www.googleapis.com/auth/documents
   https://www.googleapis.com/auth/drive
   https://www.googleapis.com/auth/gmail.send
   ```
3. Click **"Update"** and then **"Save and Continue"**

### Step 5: Create OAuth Desktop Credentials

1. Go to **"APIs & Services" → "Credentials"**
2. Click **"Create Credentials" → "OAuth client ID"**
3. Select **"Desktop app"** as the application type
4. Name it `Hermes Desktop Client`
5. Click **"Create"**
6. Click **"Download JSON"** on the confirmation dialog
7. Save the file as `credentials.json` in your Hermes project root directory

### Step 6: Create OAuth Web Application Credentials (for user login)

1. In **"Credentials"**, click **"Create Credentials" → "OAuth client ID"** again
2. This time select **"Web application"**
3. Name it `Hermes Web Client`
4. Under **"Authorized JavaScript origins"**, add:
   - `http://localhost:8000`
   - (Add production URLs when deploying)
5. Under **"Authorized redirect URIs"**, add:
   - `http://localhost:8000/torii/redirect.html`
   - (Add production URLs when deploying)
6. Click **"Create"**
7. **Copy the Client ID** - you'll need it for configuration

### Step 7: Create Test Folders in Google Drive

1. Go to [Google Drive](https://drive.google.com/)
2. Create three new folders (in My Drive or a Shared Drive):
   - `Hermes Test - Published Docs`
   - `Hermes Test - Drafts`
   - `Hermes Test - Shortcuts`
3. For each folder, get its ID from the URL:
   - Open the folder
   - Look at the URL: `https://drive.google.com/drive/folders/FOLDER_ID_HERE`
   - Copy the `FOLDER_ID_HERE` part

### Step 8: Configure Hermes

1. Copy `config-example.hcl` to `config.hcl` if you haven't already
2. Edit your `config.hcl`:

```hcl
google_workspace {
  domain = "company.com"  # Your organization's domain

  # IMPORTANT: Comment out or remove the auth block!
  # Hermes will automatically use credentials.json when auth is not configured
  # auth { ... }  ← DO NOT include this section

  # Paste your folder IDs from Step 7
  docs_folder = "your-docs-folder-id-here"
  drafts_folder = "your-drafts-folder-id-here"
  shortcuts_folder = "your-shortcuts-folder-id-here"

  create_doc_shortcuts = true

  # OAuth configuration for user login
  oauth2 {
    client_id = "your-client-id.apps.googleusercontent.com"  # From Step 6
    hd = "company.com"  # Restrict login to your domain
    redirect_uri = "http://localhost:8000/torii/redirect.html"
  }
}

# Use Dex for authentication (good for testing)
dex {
  disabled = false
  issuer_url = "http://localhost:5556/dex"
  client_id = "hermes"
  redirect_uri = "http://localhost:8000/torii/redirect.html"
}

# Set up search backend (Meilisearch is easiest for local testing)
meilisearch {
  host = "http://localhost:7700"
  api_key = "masterKey123"
  docs_index_name = "docs"
  drafts_index_name = "drafts"
  links_index_name = "links"
  projects_index_name = "projects"
}

# Configure providers
providers {
  auth = "dex"  # or "google" to use Google OAuth
  search = "meilisearch"
  workspace = "google"
}
```

### Step 9: Start Hermes

1. Start required services (if using Dex + Meilisearch):
   ```bash
   cd testing
   make up
   ```

2. Start Hermes server:
   ```bash
   hermes server
   ```

3. **First-time authentication**:
   - Hermes will automatically open your browser
   - Sign in with your Google Workspace account
   - Grant the requested permissions
   - A `token.json` file will be created to store your credentials
   - The browser will show "The token has been recorded and this window can be closed"

4. Access Hermes at `http://localhost:8000`

### Step 10: Test It Out

You should now be able to:
- Create new documents (they'll appear in your Drafts folder)
- Publish documents (they'll move to your Published Docs folder)
- Search for users in your organization
- Collaborate with others

---

## Limitations of Development Setup

When using OAuth desktop credentials (Quick Start above), you have these limitations:

### What Works ✅

- Creating and editing documents in folders you own
- Searching for users in your organization's directory
- Sending email notifications (as yourself)
- Managing permissions on documents you created
- Full Hermes UI and workflow testing

### What Doesn't Work ❌

- **No user impersonation**: All operations happen as YOU, not as individual users
- **Can't create docs as other users**: Documents are always owned by your account
- **Limited to your permissions**: Can only access folders you personally have access to
- **No background operations**: Requires you to keep `token.json` valid
- **Not suitable for production**: Users can't interact with their own documents

### When to Upgrade to Service Account

You need the Advanced Setup (service account) when:
- Deploying to production
- Multiple users need to create/edit their own documents
- Documents should be owned by individual users, not a single service account
- Running background jobs (indexing, notifications) without manual authentication
- Need Hermes to operate as a service, not as a single user

---

## Advanced Setup: Production with Service Accounts

This setup is for production deployments where:
- Multiple users need to interact with Hermes
- Documents should be owned by individual users
- The service needs to run unattended
- You have Google Workspace admin access

**Time Required**: ~45-60 minutes
**Prerequisites**: Google Workspace admin access

### Overview of Service Account Authentication

Service accounts allow Hermes to:
1. **Impersonate users** to create documents on their behalf
2. **Run unattended** without manual authentication
3. **Access organization resources** with proper delegation

This requires two types of credentials:
1. **Service Account** (for backend API operations)
2. **OAuth Web Client** (for user login)

### Part 1: Google Cloud Setup

#### Step 1: Create Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (e.g., `hermes-production`)
3. Enable billing if required for your organization
4. Note your Project ID

#### Step 2: Enable APIs

Enable the following APIs in **"APIs & Services" → "Library"**:

- **Google Docs API**
- **Google Drive API**
- **Gmail API**
- **People API**
- **Admin SDK API** (required for service accounts)

#### Step 3: Configure OAuth Consent Screen

1. Go to **"APIs & Services" → "OAuth consent screen"**
2. Choose **"Internal"** (restricts to your organization)
3. Fill in:
   - **App name**: `Hermes`
   - **User support email**: Support team email
   - **App logo**: Optional
   - **Authorized domains**: Your domain (e.g., `company.com`)
   - **Developer contact**: Your email

#### Step 4: Add OAuth Scopes

Add these scopes for user authentication:

```
https://www.googleapis.com/auth/drive.readonly
https://www.googleapis.com/auth/userinfo.email
https://www.googleapis.com/auth/userinfo.profile
```

Note: Service account scopes are configured separately in domain-wide delegation.

#### Step 5: Create OAuth Web Application

1. Go to **"APIs & Services" → "Credentials"**
2. Click **"Create Credentials" → "OAuth client ID"**
3. Select **"Web application"**
4. Configure:
   - **Name**: `Hermes Production`
   - **Authorized JavaScript origins**:
     - `https://hermes.yourcompany.com`
   - **Authorized redirect URIs**:
     - `https://hermes.yourcompany.com/torii/redirect.html`
5. Click **"Create"**
6. **Save the Client ID** - you'll need it for `config.hcl`

#### Step 6: Create Service Account

1. In **"Credentials"**, click **"Create Credentials" → "Service account"**
2. Fill in:
   - **Service account name**: `hermes-service`
   - **Service account ID**: `hermes-service` (auto-generated)
   - **Description**: `Hermes backend service account for document operations`
3. Click **"Create and Continue"**
4. Skip role assignment (not needed for this setup)
5. Click **"Done"**

#### Step 7: Create Service Account Key

1. Click on the service account you just created
2. Go to the **"Keys"** tab
3. Click **"Add Key" → "Create new key"**
4. Select **"JSON"** format
5. Click **"Create"**
6. Save the JSON file securely - you'll need values from it for `config.hcl`
7. **Keep this file secure** - it grants access to your Google Workspace!

#### Step 8: Get Service Account Client ID

1. Still in the service account details page
2. Copy the **"Unique ID"** (numeric ID) - you'll need this for domain-wide delegation
3. Also note the **email** (e.g., `hermes-service@your-project.iam.gserviceaccount.com`)

### Part 2: Google Workspace Admin Setup

These steps require **Google Workspace admin** access.

#### Step 9: Enable Domain-Wide Delegation

1. Go to [Google Admin Console](https://admin.google.com/)
2. Navigate to **"Security" → "Access and data control" → "API controls"**
3. Scroll to **"Domain-wide delegation"**
4. Click **"Manage Domain Wide Delegation"**
5. Click **"Add new"**
6. Fill in:
   - **Client ID**: Paste the service account Unique ID from Step 8
   - **OAuth scopes**: Paste these scopes (comma-separated):
     ```
     https://www.googleapis.com/auth/directory.readonly,https://www.googleapis.com/auth/documents,https://www.googleapis.com/auth/drive,https://www.googleapis.com/auth/gmail.send,https://www.googleapis.com/auth/admin.directory.group.readonly
     ```
7. Click **"Authorize"**

**Important**: These scopes allow the service account to:
- Read organization directory
- Create/edit documents
- Manage Drive files
- Send emails
- Read group memberships

#### Step 10: Enable Directory Sharing

This allows Hermes to search for users when adding collaborators.

1. In Google Admin Console, go to **"Directory" → "Directory settings"**
2. Click **"Sharing settings"**
3. Ensure **"Enable contact sharing"** is checked
4. Set to **"All users in the domain"** or as per your organization's policy
5. Click **"Save"**

**Note**: Changes may take up to 24 hours to propagate.

#### Step 11: Create Service User (Optional but Recommended)

Create a dedicated user account for the service to impersonate:

1. In Google Admin Console, go to **"Users"**
2. Click **"Add new user"**
3. Create user:
   - **First name**: `Hermes`
   - **Last name**: `Service`
   - **Primary email**: `hermes-service@yourcompany.com`
4. Assign necessary licenses
5. Grant this user access to any shared drives Hermes needs to access

**Why?** This account acts as the "subject" for API operations. All background tasks (like indexing) will run as this user.

### Part 3: Google Drive Organization

#### Step 12: Create Shared Drive

1. Go to [Google Drive](https://drive.google.com/)
2. Click **"Shared drives"** (left sidebar)
3. Click **"New"** to create a new shared drive
4. Name it `Hermes Documents`

#### Step 13: Create Folder Structure

Create this structure in your shared drive:

```
Hermes Documents/
├── Published Docs/       (flat structure - all published documents)
├── Drafts/              (private - draft documents)
└── Document Browser/    (organized shortcuts by type/product)
    ├── RFC/
    ├── PRD/
    └── FRD/
```

**To create folders:**
1. Open the shared drive
2. Click **"New" → "Folder"**
3. Create each folder

#### Step 14: Set Folder Permissions

**Published Docs folder:**
- Share with: All users in organization (read access)
- Service account: Manager access

**Drafts folder:**
- Share with: Service account only (manager access)
- Keep private - Hermes will share individual drafts with authors

**Document Browser folder:**
- Share with: All users in organization (read access)
- Service account: Manager access

**To share folders:**
1. Right-click folder → "Share"
2. Add service account email: `hermes-service@your-project.iam.gserviceaccount.com`
3. Set permission level
4. For organization sharing, click "Done" then "Share with organization"

#### Step 15: Get Folder IDs

For each of the three main folders:
1. Open the folder in Google Drive
2. Look at the URL: `https://drive.google.com/drive/folders/FOLDER_ID`
3. Copy the `FOLDER_ID` portion
4. Save these - you'll need them for `config.hcl`

### Part 4: Hermes Configuration

#### Step 16: Configure config.hcl

Create or update your `config.hcl`:

```hcl
# Base configuration
base_url = "https://hermes.yourcompany.com"

# Google Workspace configuration
google_workspace {
  domain = "yourcompany.com"

  # Service Account Authentication
  auth {
    # Values from the service account JSON key file
    client_email = "hermes-service@your-project.iam.gserviceaccount.com"

    # Private key from JSON file (paste entire key including BEGIN/END lines)
    private_key = <<-EOT
      -----BEGIN PRIVATE KEY-----
      MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC...
      ...paste your entire private key here...
      -----END PRIVATE KEY-----
    EOT

    # User to impersonate for background operations
    subject = "hermes-service@yourcompany.com"

    # Create documents as the requesting user (recommended)
    create_docs_as_user = true

    # Token URL (leave as default)
    token_url = "https://oauth2.googleapis.com/token"
  }

  # OAuth for user authentication
  oauth2 {
    # Client ID from Step 5
    client_id = "your-client-id.apps.googleusercontent.com"

    # Restrict to your domain
    hd = "yourcompany.com"

    # Redirect URI (must match Google Console configuration)
    redirect_uri = "https://hermes.yourcompany.com/torii/redirect.html"
  }

  # Folder IDs from Step 15
  docs_folder = "your-published-docs-folder-id"
  drafts_folder = "your-drafts-folder-id"
  shortcuts_folder = "your-document-browser-folder-id"

  # Create organized shortcuts when publishing
  create_doc_shortcuts = true

  # Optional: Enable Google Groups as approvers
  group_approvals {
    enabled = true
    # search_prefix = "team-"  # Filter groups by prefix
  }

  # Optional: Email notification when user lookup fails
  # user_not_found_email = "admin@yourcompany.com"
}

# Search backend (Algolia example)
algolia {
  application_id = "YOUR_APP_ID"
  write_api_key = "YOUR_ADMIN_KEY"
  search_api_key = "YOUR_SEARCH_KEY"
  docs_index_name = "docs"
  drafts_index_name = "drafts"
  internal_index_name = "internal"
  links_index_name = "links"
  missing_fields_index_name = "missing_fields"
  projects_index_name = "projects"
}

# Database configuration
database {
  dsn = "postgres://user:pass@localhost:5432/hermes?sslmode=require"
}

# Providers
providers {
  auth = "google"
  search = "algolia"
  workspace = "google"
}
```

#### Step 17: Secure Configuration

**Never commit credentials to version control!**

Options for securing credentials:

1. **Environment variables**:
   ```bash
   export GOOGLE_CLIENT_EMAIL="..."
   export GOOGLE_PRIVATE_KEY="..."
   ```

   Reference in config:
   ```hcl
   auth {
     client_email = env("GOOGLE_CLIENT_EMAIL")
     private_key = env("GOOGLE_PRIVATE_KEY")
   }
   ```

2. **Secret management** (AWS Secrets Manager, Vault, etc.)

3. **Kubernetes secrets** (if deploying on K8s)

4. **Restrict file permissions**:
   ```bash
   chmod 600 config.hcl
   ```

### Part 5: Testing and Deployment

#### Step 18: Test Service Account

Test the service account authentication:

```bash
# Set required environment variables
export DATABASE_URL="your-database-connection-string"

# Run the server
hermes server
```

Look for successful startup messages indicating:
- Service account loaded successfully
- APIs initialized
- No authentication errors

#### Step 19: Test User Login

1. Open Hermes in your browser: `https://hermes.yourcompany.com`
2. Click "Sign in with Google"
3. Authenticate with a user account (not the service account)
4. Verify you can access the application

#### Step 20: Test Document Creation

1. Create a new document in Hermes
2. Verify:
   - Document appears in the Drafts folder
   - Document is owned by the correct user (if `create_docs_as_user = true`)
   - You can edit the document
   - Document syncs back to Hermes

#### Step 21: Test Publishing

1. Publish a document
2. Verify:
   - Document moves from Drafts to Published Docs folder
   - Shortcut appears in Document Browser (if enabled)
   - Document is searchable in Hermes
   - Permissions are set correctly

---

## Authentication Architecture

### How It Works

Hermes uses a **dual authentication system**:

#### 1. User Authentication (Frontend)

**Flow**:
```
User → Hermes UI → Google OAuth → Google Sign-In → Hermes Session
```

**Purpose**: Authenticates users logging into Hermes

**Credentials**: OAuth Web Client (from Part 1, Step 5)

**Scopes**:
- `drive.readonly`: View file metadata
- `userinfo.email`: Get user email
- `userinfo.profile`: Get user profile

#### 2. Service Account (Backend)

**Flow**:
```
Hermes Backend → Service Account → Impersonate User → Google APIs
```

**Purpose**: Performs operations on behalf of users

**Credentials**: Service Account with domain-wide delegation

**Scopes**:
- `directory.readonly`: Read organization directory
- `documents`: Create/edit Google Docs
- `drive`: Manage files and folders
- `gmail.send`: Send email notifications
- `admin.directory.group.readonly`: Read group memberships

### User Impersonation

When `create_docs_as_user = true`:

1. User creates document in Hermes
2. Hermes uses service account to impersonate that user
3. Document is created in Google Docs as the user
4. User owns the document (not the service account)

When `create_docs_as_user = false`:

1. All documents are created as the service account
2. Service account is the owner
3. Users are granted editor access
4. Less seamless user experience

**Recommendation**: Always use `create_docs_as_user = true` for production.

---

## Troubleshooting

### OAuth Desktop Setup Issues

#### Error: "credentials.json not found"

**Cause**: File is missing or in wrong location

**Solution**:
- Ensure `credentials.json` is in the Hermes project root
- Check filename is exact (case-sensitive)
- Re-download from Google Cloud Console if needed

#### Error: "Failed to retrieve token"

**Cause**: Browser didn't complete OAuth flow

**Solution**:
- Check browser opens automatically
- Disable pop-up blockers
- Manually visit the URL printed in terminal
- Ensure redirect URI is `http://localhost:9999/callback`

#### Error: "Access denied" when accessing Drive folders

**Cause**: Your account doesn't have access to the folders

**Solution**:
- Verify you're the owner of the test folders
- Check folder IDs are correct in `config.hcl`
- Ensure folders exist and aren't trashed

#### Error: "User not found" when searching

**Cause**: Directory sharing not enabled

**Solution**:
- This requires admin access to enable
- Ask your Google Workspace admin to enable directory sharing
- Alternative: Use Dex for authentication and skip user search features

### Service Account Setup Issues

#### Error: "Service account has no delegation"

**Cause**: Domain-wide delegation not configured correctly

**Solution**:
- Verify you used the service account **Client ID** (numeric), not email
- Check scopes are exactly as specified (no spaces, comma-separated)
- Wait a few minutes for changes to propagate
- Re-authorize if needed

#### Error: "Insufficient permissions"

**Cause**: Missing API scopes or folder permissions

**Solution**:
- Verify all required APIs are enabled in Google Cloud Console
- Check service account has Manager access to all three folders
- Verify service account email in folder sharing settings
- Ensure scopes in domain-wide delegation match requirements

#### Error: "Invalid grant" or "unauthorized_client"

**Cause**: Service account configuration issue

**Solution**:
- Verify `client_email` matches service account email exactly
- Check `private_key` includes BEGIN/END lines and is properly formatted
- Ensure `subject` email exists and has proper licenses
- Verify `subject` user has access to the shared drive

#### Error: "Calendar cannot be found" (misleading error)

**Cause**: Often means subject user doesn't exist or isn't properly licensed

**Solution**:
- Verify `subject` email exists in Google Workspace
- Check user has proper licenses assigned
- Ensure user account is active (not suspended)

### User Authentication Issues

#### Users can't log in with Google

**Cause**: OAuth Web Client misconfigured

**Solution**:
- Verify `client_id` in `config.hcl` matches Google Cloud Console
- Check redirect URI exactly matches (including `/torii/redirect.html`)
- Ensure OAuth consent screen is configured
- For external consent screen, add test users

#### "Access blocked: Authorization Error"

**Cause**: OAuth consent screen not properly configured

**Solution**:
- Verify app is set to "Internal" (for workspace users)
- Check authorized domain is added
- Ensure required scopes are added to consent screen
- Verify user is part of your organization

### Directory and User Lookup Issues

#### Can't find users when adding collaborators

**Cause**: Directory sharing not enabled or People API issue

**Solution**:
- Admin must enable directory sharing (see Step 10)
- Wait up to 24 hours after enabling
- Verify People API is enabled
- Check service account has `directory.readonly` scope

#### Group approvals not working

**Cause**: Missing Admin SDK API or permissions

**Solution**:
- Enable Admin SDK API in Google Cloud Console
- Add `admin.directory.group.readonly` scope to domain-wide delegation
- Verify `group_approvals.enabled = true` in config
- Check service account has necessary permissions

### Document Creation Issues

#### Documents created in wrong folder

**Cause**: Incorrect folder IDs in configuration

**Solution**:
- Verify folder IDs in `config.hcl` match Google Drive
- Check IDs don't have extra characters (quotes, spaces)
- Ensure folders exist and aren't trashed
- Service account must have Manager access to folders

#### Documents not owned by correct user

**Cause**: `create_docs_as_user` setting or impersonation issue

**Solution**:
- Set `create_docs_as_user = true` in config
- Verify service account can impersonate users
- Check user exists in Google Workspace
- Ensure user has proper Drive licenses

#### Notifications not sending

**Cause**: Gmail API or permissions issue

**Solution**:
- Enable Gmail API in Google Cloud Console
- Add `gmail.send` scope to domain-wide delegation
- Verify `subject` user can send emails
- Check user email settings allow API access

### Performance Issues

#### Slow document indexing

**Cause**: Rate limiting or parallel operation limits

**Solution**:
- Adjust `indexer.max_parallel_docs` in config (try 3-5 for Google)
- Monitor API quotas in Google Cloud Console
- Consider requesting quota increases
- Spread indexing operations over time

#### "Quota exceeded" errors

**Cause**: Google API rate limits

**Solution**:
- Check quotas in Google Cloud Console → "APIs & Services" → "Quotas"
- Request quota increases if needed (usually approved quickly)
- Implement backoff in custom scripts
- Reduce `max_parallel_docs` setting

---

## Security Best Practices

### Credential Management

1. **Never commit credentials to Git**:
   ```bash
   # Add to .gitignore
   credentials.json
   token.json
   config.hcl  # if it contains secrets
   ```

2. **Use environment variables** for sensitive data:
   ```hcl
   auth {
     client_email = env("GOOGLE_CLIENT_EMAIL")
     private_key = env("GOOGLE_PRIVATE_KEY")
   }
   ```

3. **Restrict file permissions**:
   ```bash
   chmod 600 credentials.json config.hcl
   ```

4. **Use secret management** in production:
   - AWS Secrets Manager
   - HashiCorp Vault
   - Kubernetes Secrets
   - Google Secret Manager

### Service Account Security

1. **Limit scope access**: Only grant necessary scopes
2. **Rotate keys regularly**: Create new keys, delete old ones
3. **Monitor activity**: Review service account usage in Admin Console
4. **Use dedicated subject user**: Don't use admin accounts
5. **Audit regularly**: Review domain-wide delegation grants

### OAuth Security

1. **Use "Internal" consent screen** when possible
2. **Restrict redirect URIs**: Only add necessary URLs
3. **Validate hosted domain**: Use `hd` parameter in config
4. **Enable HTTPS** in production
5. **Implement session management**: Use proper timeouts

---

## Configuration Reference

### Minimal Development Config (OAuth Desktop)

```hcl
google_workspace {
  domain = "company.com"

  # No auth block - uses credentials.json automatically

  docs_folder = "folder-id-1"
  drafts_folder = "folder-id-2"
  shortcuts_folder = "folder-id-3"

  oauth2 {
    client_id = "xxx.apps.googleusercontent.com"
    hd = "company.com"
    redirect_uri = "http://localhost:8000/torii/redirect.html"
  }
}
```

### Full Production Config (Service Account)

```hcl
google_workspace {
  domain = "company.com"

  auth {
    client_email = "hermes@project.iam.gserviceaccount.com"
    private_key = <<-EOT
      -----BEGIN PRIVATE KEY-----
      ...
      -----END PRIVATE KEY-----
    EOT
    subject = "hermes-service@company.com"
    create_docs_as_user = true
    token_url = "https://oauth2.googleapis.com/token"
  }

  oauth2 {
    client_id = "xxx.apps.googleusercontent.com"
    hd = "company.com"
    redirect_uri = "https://hermes.company.com/torii/redirect.html"
  }

  docs_folder = "folder-id-1"
  drafts_folder = "folder-id-2"
  shortcuts_folder = "folder-id-3"
  temporary_drafts_folder = "folder-id-4"  # Optional

  create_doc_shortcuts = true

  group_approvals {
    enabled = true
    search_prefix = "team-"
  }

  user_not_found_email = "admin@company.com"
}
```

---

## Migration Path

### From Development to Production

When you're ready to move from OAuth desktop to service account:

1. **Complete Advanced Setup** (Steps 1-15)
2. **Update config.hcl**: Add `auth` block with service account
3. **Remove old credentials**: Delete `credentials.json` and `token.json`
4. **Update folders**: Point to production folders (shared drive)
5. **Test thoroughly**: Verify all functionality works
6. **Deploy**: Follow your deployment process

### From Personal Folders to Shared Drive

To migrate from personal Drive folders to shared drive:

1. **Create shared drive** (Step 12-13)
2. **Move documents**:
   ```bash
   # In Google Drive UI: Select → Move → Shared Drive
   ```
3. **Update config.hcl**: Use new folder IDs
4. **Run indexer**: Re-index to update database
5. **Verify**: Check all documents are accessible

---

## Additional Resources

### Official Documentation

- [Google Workspace APIs](https://developers.google.com/workspace)
- [Service Account Authentication](https://developers.google.com/identity/protocols/oauth2/service-account)
- [Domain-Wide Delegation](https://developers.google.com/identity/protocols/oauth2/service-account#delegatingauthority)
- [OAuth 2.0](https://developers.google.com/identity/protocols/oauth2)

### Hermes Documentation

- [Configuration Documentation](CONFIG_HCL_DOCUMENTATION.md)
- [Local Workspace Setup](README-local-workspace.md) - For testing without Google
- [Auth Providers Overview](README-auth-providers.md)
- [Authentication Architecture](AUTH_ARCHITECTURE_DIAGRAMS.md)

### Support

- File issues: [GitHub Issues](https://github.com/hashicorp-forge/hermes/issues)
- Check logs: Set `log_format = "json"` for structured logging
- Enable debug: See application logs for API errors

---

## Quick Reference

### Required Google APIs

- Google Docs API
- Google Drive API
- Gmail API
- People API
- Admin SDK API (for service accounts)

### Required OAuth Scopes (Service Account)

```
https://www.googleapis.com/auth/directory.readonly
https://www.googleapis.com/auth/documents
https://www.googleapis.com/auth/drive
https://www.googleapis.com/auth/gmail.send
https://www.googleapis.com/auth/admin.directory.group.readonly
```

### Required OAuth Scopes (User Login)

```
https://www.googleapis.com/auth/drive.readonly
https://www.googleapis.com/auth/userinfo.email
https://www.googleapis.com/auth/userinfo.profile
```

### Folder IDs

Get from URL: `https://drive.google.com/drive/folders/FOLDER_ID`

### Service Account Client ID

Found in service account details → "Unique ID" (numeric value)

### Testing Checklist

- [ ] APIs enabled in Google Cloud Console
- [ ] OAuth consent screen configured
- [ ] Credentials created (desktop OR web + service account)
- [ ] Folders created in Google Drive
- [ ] Folder IDs added to config.hcl
- [ ] Service account has Manager access to folders (production only)
- [ ] Domain-wide delegation configured (production only)
- [ ] Directory sharing enabled
- [ ] config.hcl properly configured
- [ ] Hermes starts without errors
- [ ] Can authenticate with Google
- [ ] Can create and edit documents
- [ ] Can search for users
- [ ] Documents appear in correct folders
