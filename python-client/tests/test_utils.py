"""Tests for frontmatter utilities."""

from pathlib import Path
from textwrap import dedent

import pytest

from hc_hermes.models import DocumentStatus
from hc_hermes.utils import (
    add_frontmatter,
    create_document_template,
    extract_frontmatter,
    parse_markdown_document,
)


def test_parse_markdown_document(tmp_path: Path) -> None:
    """Test parsing Markdown document with frontmatter."""
    content = dedent(
        """
        ---
        title: Test RFC
        docType: RFC
        product: terraform
        status: WIP
        summary: This is a test document
        tags:
          - test
          - rfc
        approvers:
          - reviewer@example.com
        ---

        # Test RFC

        This is the content of the document.
        """
    ).strip()

    file_path = tmp_path / "test.md"
    file_path.write_text(content)

    parsed = parse_markdown_document(file_path)

    assert parsed.title == "Test RFC"
    assert parsed.doc_type == "RFC"
    assert parsed.product == "terraform"
    assert parsed.status == DocumentStatus.WIP
    assert parsed.summary == "This is a test document"
    assert parsed.tags == ["test", "rfc"]
    assert parsed.approvers == ["reviewer@example.com"]
    assert "This is the content" in parsed.content


def test_parse_missing_file() -> None:
    """Test parsing non-existent file raises error."""
    with pytest.raises(FileNotFoundError):
        parse_markdown_document("/nonexistent/file.md")


def test_parse_missing_required_field() -> None:
    """Test parsing document without required fields raises error."""
    from hc_hermes.utils import DocumentParser

    content = dedent(
        """
        ---
        docType: RFC
        ---

        Content without title
        """
    ).strip()

    parser = DocumentParser(required_fields=["title"])
    with pytest.raises(ValueError, match="Missing required field: title"):
        parser.parse_string(content)


def test_create_document_template() -> None:
    """Test creating document template."""
    template = create_document_template(
        doc_type="RFC",
        title="Test RFC",
        product="vault",
        author="author@example.com",
        summary="A test RFC",
    )

    assert "title: Test RFC" in template
    assert "docType: RFC" in template
    assert "product: vault" in template
    assert "status: WIP" in template
    assert "contributors:\n- author@example.com" in template
    assert "# Test RFC" in template
    assert "A test RFC" in template


def test_extract_frontmatter() -> None:
    """Test extracting frontmatter from Markdown."""
    content = dedent(
        """
        ---
        title: Test
        product: terraform
        ---

        Content here
        """
    ).strip()

    metadata, text = extract_frontmatter(content)

    assert metadata["title"] == "Test"
    assert metadata["product"] == "terraform"
    assert "Content here" in text


def test_add_frontmatter() -> None:
    """Test adding frontmatter to content."""
    content = "# Test Document\n\nContent here"
    metadata = {"title": "Test", "product": "vault"}

    result = add_frontmatter(content, metadata)

    assert "---" in result
    assert "title: Test" in result
    assert "product: vault" in result
    assert "Content here" in result
