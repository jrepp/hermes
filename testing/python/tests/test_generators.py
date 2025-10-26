"""Tests for document generators."""

"""Tests for document generators."""

import sys
import uuid
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

import yaml

from generators import DocumentGenerator, DocumentStatus


class TestDocumentGenerator:
    """Test document generation functionality."""

    def test_generate_uuid(self, generator: DocumentGenerator) -> None:
        """Test UUID generation."""
        doc_uuid = generator.generate_uuid()
        assert isinstance(doc_uuid, str)
        # Verify it's a valid UUID
        parsed = uuid.UUID(doc_uuid)
        assert str(parsed) == doc_uuid

    def test_generate_timestamp(self, generator: DocumentGenerator) -> None:
        """Test timestamp generation."""
        timestamp = generator.generate_timestamp()
        assert isinstance(timestamp, str)
        assert "T" in timestamp  # ISO 8601 format
        assert "Z" in timestamp or "+" in timestamp or "-" in timestamp

    def test_generate_rfc(self, generator: DocumentGenerator) -> None:
        """Test RFC document generation."""
        content = generator.generate_rfc(
            number=1,
            title="Test RFC",
            status=DocumentStatus.DRAFT,
        )

        # Should have frontmatter
        assert content.startswith("---\n")

        # Parse frontmatter
        parts = content.split("---\n")
        assert len(parts) >= 3

        metadata = yaml.safe_load(parts[1])
        assert metadata["title"] == "Test RFC"
        assert metadata["doc_type"] == "RFC"
        assert metadata["status"] == "WIP"
        assert "uuid" in metadata

        # Should have content sections
        body = parts[2]
        assert "# Test RFC" in body
        assert "## Summary" in body
        assert "## Background" in body
        assert "## Proposal" in body

    def test_generate_prd(self, generator: DocumentGenerator) -> None:
        """Test PRD document generation."""
        content = generator.generate_prd(
            number=2,
            title="Test PRD",
            status=DocumentStatus.IN_REVIEW,
        )

        parts = content.split("---\n")
        metadata = yaml.safe_load(parts[1])

        assert metadata["title"] == "Test PRD"
        assert metadata["doc_type"] == "PRD"
        assert metadata["status"] == "In-Review"

        body = parts[2]
        assert "## Executive Summary" in body
        assert "## Problem Statement" in body
        assert "## Requirements" in body

    def test_generate_meeting_notes(self, generator: DocumentGenerator) -> None:
        """Test meeting notes generation."""
        content = generator.generate_meeting_notes(
            number=1,
            title="Test Meeting",
            attendees=["alice@example.com", "bob@example.com"],
        )

        parts = content.split("---\n")
        metadata = yaml.safe_load(parts[1])

        assert metadata["title"] == "Test Meeting"
        assert metadata["doc_type"] == "Meeting Notes"
        assert "alice@example.com" in metadata["attendees"]

        body = parts[2]
        assert "## Agenda" in body
        assert "## Discussion" in body
        assert "## Action Items" in body

    def test_generate_doc_page(self, generator: DocumentGenerator) -> None:
        """Test documentation page generation."""
        content = generator.generate_doc_page(
            title="Test Documentation",
            category="Testing",
        )

        parts = content.split("---\n")
        metadata = yaml.safe_load(parts[1])

        assert metadata["title"] == "Test Documentation"
        assert metadata["doc_type"] == "Documentation"
        assert metadata["category"] == "Testing"

        body = parts[2]
        assert "## Overview" in body
        assert "## Purpose" in body

    def test_uuid_uniqueness(self, generator: DocumentGenerator) -> None:
        """Test that generated UUIDs are unique."""
        uuids = {generator.generate_uuid() for _ in range(100)}
        assert len(uuids) == 100  # All unique

    def test_custom_uuid(self, generator: DocumentGenerator) -> None:
        """Test providing custom UUID."""
        custom_uuid = "12345678-1234-1234-1234-123456789abc"
        content = generator.generate_rfc(number=1, doc_uuid=custom_uuid)

        parts = content.split("---\n")
        metadata = yaml.safe_load(parts[1])

        assert metadata["uuid"] == custom_uuid
