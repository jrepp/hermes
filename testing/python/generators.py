"""Document generator for Hermes testing scenarios.

Generates realistic RFC, PRD, Meeting Notes, and documentation pages
with proper YAML frontmatter and Hermes UUIDs.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from enum import Enum
from typing import Any

import yaml


class DocumentType(str, Enum):
    """Document types supported by generator."""

    RFC = "RFC"
    PRD = "PRD"
    MEETING = "Meeting Notes"
    DOC_PAGE = "Documentation"


class DocumentStatus(str, Enum):
    """Document status values."""

    DRAFT = "WIP"
    IN_REVIEW = "In-Review"
    APPROVED = "Approved"
    OBSOLETE = "Obsolete"


class DocumentGenerator:
    """Generate test documents with proper frontmatter and content."""

    @staticmethod
    def generate_uuid() -> str:
        """Generate a UUID for document identification."""
        return str(uuid.uuid4())

    @staticmethod
    def generate_timestamp(offset_days: int = 0) -> str:
        """Generate ISO 8601 timestamp.

        Args:
            offset_days: Days offset from now (negative for past)

        Returns:
            ISO 8601 formatted timestamp
        """
        dt = datetime.now(timezone.utc)
        if offset_days != 0:
            from datetime import timedelta

            dt = dt + timedelta(days=offset_days)
        return dt.isoformat()

    @staticmethod
    def _format_frontmatter(metadata: dict[str, Any]) -> str:
        """Format YAML frontmatter.

        Args:
            metadata: Dictionary of metadata fields

        Returns:
            YAML frontmatter string with delimiters
        """
        yaml_str = yaml.dump(
            metadata,
            default_flow_style=False,
            allow_unicode=True,
            sort_keys=False,
        )
        return f"---\n{yaml_str}---\n\n"

    def generate_rfc(
        self,
        number: int,
        doc_uuid: str | None = None,
        title: str | None = None,
        status: DocumentStatus = DocumentStatus.DRAFT,
        author: str = "alice@example.com",
        created: str | None = None,
        product: str = "Test Product",
    ) -> str:
        """Generate RFC document.

        Args:
            number: RFC number
            doc_uuid: Document UUID (generated if None)
            title: Document title
            status: Document status
            author: Author email
            created: Creation timestamp (generated if None)
            product: Product name

        Returns:
            Complete RFC document with frontmatter and content
        """
        doc_uuid = doc_uuid or self.generate_uuid()
        title = title or f"RFC-{number:03d}: Test RFC"
        created = created or self.generate_timestamp()

        metadata = {
            "uuid": doc_uuid,
            "title": title,
            "doc_type": "RFC",
            "status": status.value,
            "product": product,
            "authors": [author],
            "created_at": created,
            "modified_at": created,
            "tags": ["testing", "rfc", "distributed"],
        }

        content = f"""# {title}

## Summary

This is a test RFC document for distributed testing scenarios in Hermes.
This document validates indexing, search, and multi-workspace functionality.

**Status**: {status.value}
**Author**: {author}
**Created**: {created}

## Background

This RFC was generated as part of the Hermes distributed testing framework
to simulate realistic document authoring workflows across multiple workspaces.

## Proposal

The proposal section contains searchable content that will be indexed by
the Hermes indexer and made available through the search API.

### Key Points

1. **Distributed Architecture**: Documents can exist in multiple workspaces
2. **UUID-based Identity**: Each document has a stable, globally unique identifier
3. **Search Integration**: All documents are indexed and searchable
4. **Migration Support**: Documents can migrate between providers

## Implementation

Implementation details go here with specific technical requirements.

### Phase 1: Foundation
- Initialize document structure
- Set up frontmatter metadata
- Configure UUID tracking

### Phase 2: Integration
- Index document content
- Enable search functionality
- Validate across projects

## Testing

This document itself serves as test data for:
- Document generation
- Workspace seeding
- Indexing validation
- Search functionality
- Cross-project queries

## References

- Hermes Documentation: https://github.com/hashicorp/hermes
- Testing Guide: See testing/python/README.md
"""

        return self._format_frontmatter(metadata) + content

    def generate_prd(
        self,
        number: int,
        doc_uuid: str | None = None,
        title: str | None = None,
        status: DocumentStatus = DocumentStatus.DRAFT,
        author: str = "bob@example.com",
        created: str | None = None,
        product: str = "Test Product",
    ) -> str:
        """Generate PRD document.

        Args:
            number: PRD number
            doc_uuid: Document UUID (generated if None)
            title: Document title
            status: Document status
            author: Author email
            created: Creation timestamp (generated if None)
            product: Product name

        Returns:
            Complete PRD document with frontmatter and content
        """
        doc_uuid = doc_uuid or self.generate_uuid()
        title = title or f"PRD-{number:03d}: Test Product Requirements"
        created = created or self.generate_timestamp()

        metadata = {
            "uuid": doc_uuid,
            "title": title,
            "doc_type": "PRD",
            "status": status.value,
            "product": product,
            "authors": [author],
            "created_at": created,
            "modified_at": created,
            "tags": ["testing", "prd", "requirements", "distributed"],
        }

        content = f"""# {title}

## Executive Summary

This PRD defines requirements for testing distributed document management
in the Hermes system. It serves as both documentation and test data.

**Status**: {status.value}
**Owner**: {author}
**Created**: {created}

## Problem Statement

We need comprehensive testing infrastructure to validate distributed document
workflows including:
- Multi-workspace document management
- Cross-provider migration
- Conflict detection and resolution
- Search functionality across projects

## Goals and Non-Goals

### Goals
- Generate realistic test documents
- Automate scenario validation
- Test distributed indexing
- Validate search functionality

### Non-Goals
- Production data migration
- Real user workflows
- Performance benchmarking at scale

## User Stories

### Story 1: Document Author
As a document author, I want to create documents in my workspace so that
they are indexed and searchable across the organization.

**Acceptance Criteria**:
- Document saved with proper frontmatter
- UUID assigned automatically
- Content indexed within 5 minutes
- Appears in search results

### Story 2: System Administrator
As an admin, I want to migrate documents between providers while maintaining
UUID consistency.

**Acceptance Criteria**:
- Same UUID preserved across migration
- Content hash tracking detects changes
- Conflicts flagged for resolution
- No data loss during migration

## Requirements

### Functional Requirements
1. **FR-1**: Generate documents with valid YAML frontmatter
2. **FR-2**: Assign globally unique UUIDs to all documents
3. **FR-3**: Support RFC, PRD, Meeting Notes, and Doc Pages
4. **FR-4**: Include searchable, realistic content

### Non-Functional Requirements
1. **NFR-1**: Generation speed >100 docs/second
2. **NFR-2**: Documents valid according to Hermes schema
3. **NFR-3**: Cross-platform UUID generation
4. **NFR-4**: Deterministic output for testing

## Success Metrics

- Test document generation time < 1 second
- 100% of generated documents index successfully
- Search returns all expected documents
- Zero UUID collisions across 10,000 documents

## Timeline

- **Week 1**: Document generator implementation
- **Week 2**: Scenario orchestration
- **Week 3**: Validation framework
- **Week 4**: Integration testing

## Open Questions

- Should we support custom metadata fields?
- What is the maximum realistic document size?
- How many documents should we generate for performance testing?

## Appendix

See `testing/python/README.md` for implementation details.
"""

        return self._format_frontmatter(metadata) + content

    def generate_meeting_notes(
        self,
        number: int,
        doc_uuid: str | None = None,
        title: str | None = None,
        attendees: list[str] | None = None,
        date: str | None = None,
        created: str | None = None,
    ) -> str:
        """Generate Meeting Notes document.

        Args:
            number: Meeting number
            doc_uuid: Document UUID (generated if None)
            title: Meeting title
            attendees: List of attendee emails
            date: Meeting date (uses created if None)
            created: Creation timestamp (generated if None)

        Returns:
            Complete meeting notes document with frontmatter and content
        """
        doc_uuid = doc_uuid or self.generate_uuid()
        title = title or f"Meeting-{number:03d}: Test Team Sync"
        created = created or self.generate_timestamp()
        date = date or created
        attendees = attendees or ["alice@example.com", "bob@example.com"]

        metadata = {
            "uuid": doc_uuid,
            "title": title,
            "doc_type": "Meeting Notes",
            "status": "Approved",
            "attendees": attendees,
            "meeting_date": date,
            "created_at": created,
            "modified_at": created,
            "tags": ["testing", "meeting", "sync"],
        }

        content = f"""# {title}

**Date**: {date}
**Attendees**: {", ".join(attendees)}

## Agenda

1. Testing infrastructure review
2. Distributed scenario planning
3. Python client integration
4. Action items

## Discussion

### Testing Infrastructure Review

Reviewed the current state of the Hermes testing environment and identified
areas for improvement using Python-based automation.

**Key Points**:
- Replace bash scripts with Python for better maintainability
- Leverage hc-hermes client for API interactions
- Add pytest integration for automated validation
- Improve error handling and reporting

### Distributed Scenario Planning

Discussed scenarios to test:
- Basic document creation and indexing
- Multi-workspace management
- Migration with conflict detection
- Multi-author collaboration

### Python Client Integration

The hc-hermes Python client provides:
- Type-safe API access
- Async and sync interfaces
- Frontmatter parsing utilities
- CLI tool for manual testing

## Action Items

- [ ] @alice: Implement document generator in Python
- [ ] @bob: Create scenario orchestration framework
- [ ] @alice: Add pytest fixtures for testing
- [ ] @bob: Update Makefile with Python targets

## Next Meeting

Next sync scheduled for next week to review implementation progress.

## Notes

This meeting notes document serves as test data for the Hermes distributed
testing framework. It validates document generation, indexing, and search.
"""

        return self._format_frontmatter(metadata) + content

    def generate_doc_page(
        self,
        title: str,
        doc_uuid: str | None = None,
        category: str = "Testing",
        author: str = "charlie@example.com",
        created: str | None = None,
    ) -> str:
        """Generate documentation page.

        Args:
            title: Page title
            doc_uuid: Document UUID (generated if None)
            category: Documentation category
            author: Author email
            created: Creation timestamp (generated if None)

        Returns:
            Complete documentation page with frontmatter and content
        """
        doc_uuid = doc_uuid or self.generate_uuid()
        created = created or self.generate_timestamp()

        metadata = {
            "uuid": doc_uuid,
            "title": title,
            "doc_type": "Documentation",
            "status": "Approved",
            "category": category,
            "authors": [author],
            "created_at": created,
            "modified_at": created,
            "tags": ["docs", "testing", category.lower()],
        }

        content = f"""# {title}

**Category**: {category}
**Author**: {author}
**Last Updated**: {created}

## Overview

This documentation page is generated as part of the Hermes distributed
testing framework to validate documentation indexing and search.

## Purpose

Documentation pages serve multiple purposes in the testing framework:
- Validate non-RFC/PRD document types
- Test documentation search functionality
- Provide reference material for test scenarios
- Demonstrate frontmatter parsing

## Usage

Documentation is automatically indexed by the Hermes indexer and made
available through search. Users can:

1. **Search by keyword**: Find docs using full-text search
2. **Filter by category**: Browse docs by category
3. **View by author**: See all docs from specific authors
4. **Access by UUID**: Retrieve docs by stable identifier

## Testing Scenarios

This page supports these test scenarios:
- Document generation with custom metadata
- Category-based filtering
- Documentation search validation
- Cross-workspace documentation discovery

## Related Documents

- See testing/python/README.md for framework documentation
- See testing/DISTRIBUTED_TESTING_ENHANCEMENTS.md for design
- See testing/IMPLEMENTATION_SUMMARY.md for implementation notes

## Appendix

Additional searchable content to increase document diversity and test
search relevance ranking across different document types.

### Keywords

Testing, distributed, documentation, indexing, search, validation,
automation, Python, pytest, scenarios, workspace, migration.
"""

        return self._format_frontmatter(metadata) + content


# Convenience instance
generator = DocumentGenerator()
