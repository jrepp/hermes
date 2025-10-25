"""Tests for Pydantic models."""

from uuid import uuid4

from hc_hermes.models import Document, DocumentStatus, DocumentType, Product, User


def test_document_status_enum() -> None:
    """Test DocumentStatus enum values."""
    assert DocumentStatus.WIP.value == "WIP"
    assert DocumentStatus.IN_REVIEW.value == "In-Review"
    assert DocumentStatus.APPROVED.value == "Approved"
    assert DocumentStatus.OBSOLETE.value == "Obsolete"


def test_user_model() -> None:
    """Test User model."""
    user = User(
        id=1,
        email_address="test@example.com",
        given_name="Test",
        family_name="User",
        name="Test User",
    )

    assert user.email_address == "test@example.com"
    assert user.display_name == "Test User"

    # User without name should use email
    user_no_name = User(id=2, email_address="nolabel@example.com")
    assert user_no_name.display_name == "nolabel@example.com"


def test_product_model() -> None:
    """Test Product model."""
    product = Product(id=1, name="terraform", abbreviation="TF")
    assert product.name == "terraform"
    assert product.abbreviation == "TF"


def test_document_type_model() -> None:
    """Test DocumentType model."""
    doc_type = DocumentType(id=1, name="RFC", long_name="Request for Comments")
    assert doc_type.name == "RFC"
    assert doc_type.long_name == "Request for Comments"


def test_document_model() -> None:
    """Test Document model."""
    doc_uuid = uuid4()
    project_uuid = uuid4()

    doc = Document(
        id=1,
        document_uuid=doc_uuid,
        project_uuid=project_uuid,
        title="Test Document",
        document_number=123,
        status=DocumentStatus.WIP,
        summary="This is a test",
        product=Product(id=1, name="terraform", abbreviation="TF"),
        document_type=DocumentType(id=1, name="RFC"),
    )

    assert doc.title == "Test Document"
    assert doc.status == DocumentStatus.WIP
    assert doc.document_uuid == doc_uuid
    assert doc.full_doc_number == "TF-123"


def test_document_model_defaults() -> None:
    """Test Document model with defaults."""
    doc = Document(title="Minimal Doc")

    assert doc.title == "Minimal Doc"
    assert doc.status == DocumentStatus.WIP
    assert doc.locked is False
    assert doc.imported is False
    assert doc.approvers == []
    assert doc.contributors == []


def test_document_full_doc_number() -> None:
    """Test Document.full_doc_number property."""
    # With product and document_number
    doc = Document(
        title="Test",
        product=Product(id=1, name="vault", abbreviation="VLT"),
        document_number=456,
    )
    assert doc.full_doc_number == "VLT-456"

    # With doc_number but no product
    doc2 = Document(title="Test", doc_number="TF-789")
    assert doc2.full_doc_number == "TF-789"

    # Without either
    doc3 = Document(title="Test")
    assert doc3.full_doc_number is None
