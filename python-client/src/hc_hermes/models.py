"""Pydantic models for Hermes API V2.

These models represent the data structures used in the Hermes V2 API,
closely matching the Go models in pkg/models/ and internal/api/v2/.
"""

from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class DocumentStatus(str, Enum):
    """Document status values."""

    UNSPECIFIED = "Unspecified"
    WIP = "WIP"
    IN_REVIEW = "In-Review"
    APPROVED = "Approved"
    OBSOLETE = "Obsolete"


class DocumentType(BaseModel):
    """Document type model."""

    model_config = ConfigDict(from_attributes=True)

    id: int | None = None
    name: str
    long_name: str | None = None
    description: str | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None


class Product(BaseModel):
    """Product/area model."""

    model_config = ConfigDict(from_attributes=True)

    id: int | None = None
    name: str
    abbreviation: str
    created_at: datetime | None = None
    updated_at: datetime | None = None


class User(BaseModel):
    """User model."""

    model_config = ConfigDict(from_attributes=True)

    id: int | None = None
    email_address: str
    given_name: str | None = None
    family_name: str | None = None
    name: str | None = None
    photo_url: str | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None

    @property
    def display_name(self) -> str:
        """Get display name (name or email)."""
        return self.name or self.email_address


class Group(BaseModel):
    """Group model."""

    model_config = ConfigDict(from_attributes=True)

    id: int | None = None
    name: str
    email_address: str | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None


class DocumentCustomField(BaseModel):
    """Custom field for documents."""

    model_config = ConfigDict(from_attributes=True)

    name: str
    display_name: str | None = None
    value: str | list[str] | None = None


class DocumentRelatedResource(BaseModel):
    """Related resource (external link or Hermes document)."""

    model_config = ConfigDict(from_attributes=True)

    id: int | None = None
    url: str | None = None
    title: str | None = None
    sort_order: int | None = None
    # For Hermes documents
    related_document_id: int | None = None
    google_file_id: str | None = None


class Document(BaseModel):
    """Document model matching Hermes V2 API response."""

    model_config = ConfigDict(from_attributes=True)

    # Core identifiers
    id: int | None = None
    google_file_id: str | None = Field(
        None,
        description="DEPRECATED: Use document_uuid. Kept for backward compatibility.",
    )
    document_uuid: UUID | None = Field(
        None,
        description="Stable, globally unique document identifier",
    )
    project_uuid: UUID | None = Field(
        None,
        description="Links to workspace project",
    )

    # Provider information
    provider_type: str | None = Field(
        None,
        description="Storage backend (google, local, remote-hermes)",
    )
    provider_document_id: str | None = Field(
        None,
        description="Provider-specific identifier",
    )
    project_id: str | None = Field(
        None,
        description="DEPRECATED: Use project_uuid",
    )

    # Document metadata
    title: str
    doc_number: str | None = Field(None, alias="objectID")
    document_number: int | None = None
    status: DocumentStatus = DocumentStatus.WIP
    summary: str | None = None

    # Timestamps
    created_at: datetime | None = None
    updated_at: datetime | None = None
    document_created_at: datetime | None = None
    document_modified_at: datetime | None = None

    # Relationships
    document_type: DocumentType | None = None
    product: Product | None = None
    owner: User | None = None
    owners: list[User] = Field(default_factory=list)
    approvers: list[User] = Field(default_factory=list)
    approver_groups: list[Group] = Field(default_factory=list)
    contributors: list[User] = Field(default_factory=list)
    custom_fields: list[DocumentCustomField] = Field(default_factory=list)
    related_resources: list[DocumentRelatedResource] = Field(default_factory=list)

    # Flags
    imported: bool = False
    locked: bool = False
    shareable_as_draft: bool = False

    @property
    def full_doc_number(self) -> str | None:
        """Get full document number (e.g., 'TF-123')."""
        if self.product and self.document_number:
            return f"{self.product.abbreviation}-{self.document_number}"
        return self.doc_number


class DocumentContent(BaseModel):
    """Document content (Markdown)."""

    model_config = ConfigDict(from_attributes=True)

    content: str
    document_id: str | None = None


class DocumentPatchRequest(BaseModel):
    """Request body for PATCH /api/v2/documents/:id."""

    model_config = ConfigDict(extra="forbid")

    approvers: list[str] | None = None
    approver_groups: list[str] | None = None
    contributors: list[str] | None = None
    custom_fields: list[DocumentCustomField] | None = None
    owners: list[str] | None = None
    status: DocumentStatus | None = None
    summary: str | None = None
    title: str | None = None


class Review(BaseModel):
    """Document review model."""

    model_config = ConfigDict(from_attributes=True)

    id: int | None = None
    document_id: int
    user: User
    status: str  # "APPROVED", "PENDING", "REJECTED"
    created_at: datetime | None = None
    updated_at: datetime | None = None


class Project(BaseModel):
    """Workspace project model."""

    model_config = ConfigDict(from_attributes=True)

    project_uuid: UUID
    project_id: str
    name: str
    title: str | None = None
    description: str | None = None
    jira_enabled: bool = False
    created_at: datetime | None = None
    updated_at: datetime | None = None


class SearchResult(BaseModel):
    """Search result from Algolia/Meilisearch."""

    model_config = ConfigDict(from_attributes=True, extra="allow")

    # Core fields
    object_id: str = Field(..., alias="objectID")
    title: str
    doc_number: str | None = None
    status: DocumentStatus | None = None
    summary: str | None = None
    product: str | None = None
    doc_type: str | None = None
    owners: list[str] = Field(default_factory=list)
    modified_time: int | None = None

    # Allow additional fields from search index
    extra_fields: dict[str, Any] = Field(default_factory=dict)


class SearchResponse(BaseModel):
    """Response from search API."""

    model_config = ConfigDict(from_attributes=True)

    hits: list[SearchResult]
    nb_hits: int = Field(0, description="Total number of hits")
    page: int = Field(0, description="Current page number")
    nb_pages: int = Field(0, description="Total number of pages")
    hits_per_page: int = Field(20, description="Hits per page")


class ProjectRelatedResource(BaseModel):
    """Related resource for projects."""

    model_config = ConfigDict(from_attributes=True)

    url: str | None = None
    title: str | None = None
    sort_order: int | None = None
    related_document: Document | None = None


class ProjectRelatedResourcesResponse(BaseModel):
    """Response for GET /api/v2/projects/:name/related-resources."""

    model_config = ConfigDict(from_attributes=True)

    external_links: list[ProjectRelatedResource] = Field(default_factory=list)
    hermes_documents: list[Document] = Field(default_factory=list)


class MeProfile(BaseModel):
    """Current user profile."""

    model_config = ConfigDict(from_attributes=True)

    user: User
    subscriptions: list[str] = Field(default_factory=list)


class DocumentReview(BaseModel):
    """Document review for /api/v2/me/reviews."""

    model_config = ConfigDict(from_attributes=True)

    document: Document
    review_requested_at: datetime | None = None


class WebConfig(BaseModel):
    """Web configuration from /api/v2/web/config."""

    model_config = ConfigDict(from_attributes=True)

    auth_provider: str  # "google", "dex", "okta"
    algolia_app_id: str | None = None
    algolia_search_api_key: str | None = None
    analytics_tracking_id: str | None = None
    base_url: str | None = None
    create_docs_link: str | None = None
    dex_issuer_url: str | None = None
    google_oauth2_client_id: str | None = None
    short_link_base_url: str | None = None
