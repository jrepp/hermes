-- Core schema migration (works for both PostgreSQL and SQLite)
-- This migration creates all tables using syntax compatible with both databases
-- Database-specific features (extensions, types, etc.) are in separate migration files

-- HermesInstance table (must be first - other tables reference it)
CREATE TABLE IF NOT EXISTS hermes_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    instance_uuid TEXT NOT NULL UNIQUE,
    instance_name TEXT NOT NULL,
    instance_url TEXT,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_hermes_instances_deleted_at ON hermes_instances(deleted_at);
CREATE INDEX IF NOT EXISTS idx_hermes_instances_uuid ON hermes_instances(instance_uuid);

-- DocumentType table
CREATE TABLE IF NOT EXISTS document_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    name TEXT NOT NULL UNIQUE,
    long_name TEXT,
    description TEXT
);

CREATE INDEX IF NOT EXISTS idx_document_types_deleted_at ON document_types(deleted_at);

-- DocumentTypeCustomField table
CREATE TABLE IF NOT EXISTS document_type_custom_fields (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    document_type_id INTEGER REFERENCES document_types(id),
    name TEXT NOT NULL,
    type TEXT,
    display_name TEXT,
    read_only INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_document_type_custom_fields_deleted_at ON document_type_custom_fields(deleted_at);

-- Product table
CREATE TABLE IF NOT EXISTS products (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    name TEXT NOT NULL,
    abbreviation TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);

-- User table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    email_address TEXT NOT NULL UNIQUE,
    photo_url TEXT,
    given_name TEXT,
    family_name TEXT
);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- Group table
CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    email_address TEXT NOT NULL UNIQUE,
    name TEXT
);

CREATE INDEX IF NOT EXISTS idx_groups_deleted_at ON groups(deleted_at);

-- WorkspaceProject table (for distributed projects)
CREATE TABLE IF NOT EXISTS workspace_projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    project_uuid TEXT NOT NULL UNIQUE,
    project_id TEXT NOT NULL,
    project_name TEXT,
    status TEXT,
    providers TEXT,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_workspace_projects_deleted_at ON workspace_projects(deleted_at);
CREATE INDEX IF NOT EXISTS idx_workspace_projects_uuid ON workspace_projects(project_uuid);
CREATE INDEX IF NOT EXISTS idx_workspace_projects_project_id ON workspace_projects(project_id);

-- Document table (main document entity)
CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    google_file_id TEXT NOT NULL UNIQUE,
    document_uuid TEXT,
    project_uuid TEXT,
    provider_type TEXT,
    provider_document_id TEXT,
    project_id TEXT,
    document_type_id INTEGER REFERENCES document_types(id),
    product_id INTEGER REFERENCES products(id),
    owner_id INTEGER REFERENCES users(id),
    document_number INTEGER,
    title TEXT,
    summary TEXT,
    document_created_at TIMESTAMP,
    document_modified_at TIMESTAMP,
    status INTEGER DEFAULT 0,
    imported INTEGER DEFAULT 0,
    locked INTEGER DEFAULT 0,
    shareable_as_draft INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_documents_deleted_at ON documents(deleted_at);
CREATE INDEX IF NOT EXISTS idx_documents_google_file_id ON documents(google_file_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_uuid ON documents(document_uuid) WHERE document_uuid IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_documents_project_uuid ON documents(project_uuid);
CREATE INDEX IF NOT EXISTS idx_documents_provider_doc_id ON documents(provider_document_id);
CREATE INDEX IF NOT EXISTS latest_product_number ON documents(product_id, document_number);

-- DocumentCustomField table
CREATE TABLE IF NOT EXISTS document_custom_fields (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    document_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    document_type_custom_field_id INTEGER REFERENCES document_type_custom_fields(id),
    value TEXT
);

CREATE INDEX IF NOT EXISTS idx_document_custom_fields_deleted_at ON document_custom_fields(deleted_at);

-- DocumentFileRevision table
CREATE TABLE IF NOT EXISTS document_file_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    document_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    google_drive_file_revision_id TEXT NOT NULL,
    name TEXT,
    mime_type TEXT
);

CREATE INDEX IF NOT EXISTS idx_document_file_revisions_deleted_at ON document_file_revisions(deleted_at);

-- DocumentRevision table (for versioning and migration tracking)
CREATE TABLE IF NOT EXISTS document_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    document_uuid TEXT NOT NULL,
    project_uuid TEXT,
    provider_type TEXT,
    provider_document_id TEXT,
    content_hash TEXT,
    revision_timestamp TIMESTAMP,
    status TEXT,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_document_revisions_deleted_at ON document_revisions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_document_revisions_uuid ON document_revisions(document_uuid);
CREATE INDEX IF NOT EXISTS idx_document_revisions_project_uuid ON document_revisions(project_uuid);

-- DocumentRelatedResource table
CREATE TABLE IF NOT EXISTS document_related_resources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    document_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    resource_type TEXT,
    sort_order INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_document_related_resources_deleted_at ON document_related_resources(deleted_at);

-- DocumentRelatedResourceExternalLink table
CREATE TABLE IF NOT EXISTS document_related_resource_external_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    document_related_resource_id INTEGER REFERENCES document_related_resources(id) ON DELETE CASCADE,
    name TEXT,
    url TEXT,
    sort_order INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_doc_related_res_ext_links_deleted_at ON document_related_resource_external_links(deleted_at);

-- DocumentRelatedResourceHermesDocument table
CREATE TABLE IF NOT EXISTS document_related_resource_hermes_documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    document_related_resource_id INTEGER REFERENCES document_related_resources(id) ON DELETE CASCADE,
    document_id INTEGER REFERENCES documents(id),
    sort_order INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_doc_related_res_hermes_docs_deleted_at ON document_related_resource_hermes_documents(deleted_at);

-- Project table
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    creator TEXT,
    jira_issue_id TEXT,
    modified_time INTEGER,
    status TEXT,
    title TEXT
);

CREATE INDEX IF NOT EXISTS idx_projects_deleted_at ON projects(deleted_at);

-- ProjectRelatedResource table
CREATE TABLE IF NOT EXISTS project_related_resources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    resource_type TEXT,
    sort_order INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_project_related_resources_deleted_at ON project_related_resources(deleted_at);

-- ProjectRelatedResourceExternalLink table
CREATE TABLE IF NOT EXISTS project_related_resource_external_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    project_related_resource_id INTEGER REFERENCES project_related_resources(id) ON DELETE CASCADE,
    name TEXT,
    url TEXT,
    sort_order INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_proj_related_res_ext_links_deleted_at ON project_related_resource_external_links(deleted_at);

-- ProjectRelatedResourceHermesDocument table
CREATE TABLE IF NOT EXISTS project_related_resource_hermes_documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    project_related_resource_id INTEGER REFERENCES project_related_resources(id) ON DELETE CASCADE,
    document_id INTEGER REFERENCES documents(id),
    sort_order INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_proj_related_res_hermes_docs_deleted_at ON project_related_resource_hermes_documents(deleted_at);

-- IndexerFolder table
CREATE TABLE IF NOT EXISTS indexer_folders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    google_drive_id TEXT NOT NULL UNIQUE,
    last_indexed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_indexer_folders_deleted_at ON indexer_folders(deleted_at);

-- IndexerMetadata table
CREATE TABLE IF NOT EXISTS indexer_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    last_full_index_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_indexer_metadata_deleted_at ON indexer_metadata(deleted_at);

-- ProductLatestDocumentNumber table
CREATE TABLE IF NOT EXISTS product_latest_document_numbers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    product_id INTEGER REFERENCES products(id) ON DELETE CASCADE,
    document_number INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_product_latest_doc_numbers_deleted_at ON product_latest_document_numbers(deleted_at);

-- Join tables (many-to-many relationships)

-- DocumentReviews (document approvers)
CREATE TABLE IF NOT EXISTS document_reviews (
    document_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    status TEXT,
    PRIMARY KEY (document_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_document_reviews_deleted_at ON document_reviews(deleted_at);

-- DocumentGroupReviews (document approver groups)
CREATE TABLE IF NOT EXISTS document_group_reviews (
    document_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    group_id INTEGER REFERENCES groups(id) ON DELETE CASCADE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    status TEXT,
    PRIMARY KEY (document_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_document_group_reviews_deleted_at ON document_group_reviews(deleted_at);

-- DocumentContributors (users who contributed to document)
CREATE TABLE IF NOT EXISTS document_contributors (
    document_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (document_id, user_id)
);

-- RecentlyViewedDocs (user's recently viewed documents)
CREATE TABLE IF NOT EXISTS recently_viewed_docs (
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    document_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
    viewed_at TIMESTAMP,
    PRIMARY KEY (user_id, document_id)
);

-- RecentlyViewedProjects (user's recently viewed projects)
CREATE TABLE IF NOT EXISTS recently_viewed_projects (
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    viewed_at TIMESTAMP,
    PRIMARY KEY (user_id, project_id)
);
