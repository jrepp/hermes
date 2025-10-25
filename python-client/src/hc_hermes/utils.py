"""Utilities for working with Markdown documents and frontmatter."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

import frontmatter
import yaml

from hc_hermes.models import DocumentStatus


@dataclass
class ParsedDocument:
    """Parsed document with frontmatter and content."""

    # From frontmatter
    title: str
    doc_type: str | None = None
    product: str | None = None
    summary: str | None = None
    status: DocumentStatus | None = None
    tags: list[str] | None = None
    approvers: list[str] | None = None
    contributors: list[str] | None = None

    # Content
    content: str = ""

    # Raw metadata
    metadata: dict[str, Any] | None = None

    @property
    def frontmatter_dict(self) -> dict[str, Any]:
        """Get frontmatter as dictionary."""
        result: dict[str, Any] = {"title": self.title}

        if self.doc_type:
            result["docType"] = self.doc_type
        if self.product:
            result["product"] = self.product
        if self.summary:
            result["summary"] = self.summary
        if self.status:
            result["status"] = self.status.value
        if self.tags:
            result["tags"] = self.tags
        if self.approvers:
            result["approvers"] = self.approvers
        if self.contributors:
            result["contributors"] = self.contributors

        return result


class DocumentParser:
    """Parser for Markdown documents with YAML frontmatter."""

    def __init__(
        self,
        required_fields: list[str] | None = None,
        optional_fields: list[str] | None = None,
    ) -> None:
        """Initialize document parser.
        
        Args:
            required_fields: List of required frontmatter fields
            optional_fields: List of optional frontmatter fields
        """
        self.required_fields = required_fields or ["title"]
        self.optional_fields = optional_fields or []

    def parse_file(self, path: Path | str) -> ParsedDocument:
        """Parse Markdown file with frontmatter.
        
        Args:
            path: Path to Markdown file
            
        Returns:
            ParsedDocument instance
            
        Raises:
            FileNotFoundError: File not found
            ValueError: Missing required fields or invalid format
        """
        path = Path(path)
        if not path.exists():
            raise FileNotFoundError(f"File not found: {path}")

        with path.open(encoding="utf-8") as f:
            return self.parse_string(f.read())

    def parse_string(self, text: str) -> ParsedDocument:
        """Parse Markdown string with frontmatter.
        
        Args:
            text: Markdown text with YAML frontmatter
            
        Returns:
            ParsedDocument instance
            
        Raises:
            ValueError: Missing required fields or invalid format
        """
        try:
            post = frontmatter.loads(text)
        except Exception as e:
            raise ValueError(f"Failed to parse frontmatter: {e}") from e

        metadata = dict(post.metadata)

        # Validate required fields
        for field in self.required_fields:
            if field not in metadata:
                raise ValueError(f"Missing required field: {field}")

        # Extract fields
        title = metadata.get("title", "")
        doc_type = metadata.get("docType") or metadata.get("doc_type")
        product = metadata.get("product")
        summary = metadata.get("summary")
        status_str = metadata.get("status")
        tags = metadata.get("tags", [])
        approvers = metadata.get("approvers", [])
        contributors = metadata.get("contributors", [])

        # Parse status
        status = None
        if status_str:
            try:
                status = DocumentStatus(status_str)
            except ValueError:
                # Try uppercase
                try:
                    status = DocumentStatus(status_str.upper())
                except ValueError:
                    pass

        return ParsedDocument(
            title=title,
            doc_type=doc_type,
            product=product,
            summary=summary,
            status=status,
            tags=tags if isinstance(tags, list) else [tags] if tags else None,
            approvers=approvers if isinstance(approvers, list) else [approvers] if approvers else None,
            contributors=contributors if isinstance(contributors, list) else [contributors] if contributors else None,
            content=post.content,
            metadata=metadata,
        )


def parse_markdown_document(path: Path | str) -> ParsedDocument:
    """Parse Markdown file with frontmatter (convenience function).
    
    Args:
        path: Path to Markdown file
        
    Returns:
        ParsedDocument instance
    """
    parser = DocumentParser()
    return parser.parse_file(path)


def create_document_template(
    doc_type: str,
    title: str,
    product: str | None = None,
    author: str | None = None,
    summary: str | None = None,
) -> str:
    """Create a document template with frontmatter.
    
    Args:
        doc_type: Document type (RFC, PRD, FRD, etc.)
        title: Document title
        product: Product/area abbreviation
        author: Author email
        summary: Document summary
        
    Returns:
        Markdown template string
    """
    frontmatter_data: dict[str, Any] = {
        "title": title,
        "docType": doc_type,
        "status": "WIP",
    }

    if product:
        frontmatter_data["product"] = product
    if summary:
        frontmatter_data["summary"] = summary
    if author:
        frontmatter_data["contributors"] = [author]

    # Convert to YAML
    frontmatter_yaml = yaml.dump(frontmatter_data, default_flow_style=False, sort_keys=False)

    # Build template
    template = f"""---
{frontmatter_yaml.strip()}
---

# {title}

## Summary

{summary or "Brief description of this document."}

## Background

Provide context and motivation for this document.

## Proposal

Describe the proposed solution or feature.

## Alternatives Considered

What other approaches were considered and why were they not chosen?

## Open Questions

- Question 1?
- Question 2?

## References

- [Related Document](url)
"""

    return template


def extract_frontmatter(text: str) -> tuple[dict[str, Any], str]:
    """Extract frontmatter and content from Markdown text.
    
    Args:
        text: Markdown text with YAML frontmatter
        
    Returns:
        Tuple of (frontmatter_dict, content_string)
    """
    post = frontmatter.loads(text)
    return dict(post.metadata), post.content


def add_frontmatter(content: str, metadata: dict[str, Any]) -> str:
    """Add frontmatter to Markdown content.
    
    Args:
        content: Markdown content
        metadata: Frontmatter metadata dictionary
        
    Returns:
        Markdown text with YAML frontmatter
    """
    post = frontmatter.Post(content, **metadata)
    return frontmatter.dumps(post)
