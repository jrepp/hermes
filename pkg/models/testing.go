package models

import (
	"log"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/test"
)

func setupTest(t *testing.T, dsn string) (
	db *gorm.DB, tearDownFunc func(t *testing.T),
) {
	// Create test database.
	db, dbName, err := test.CreateTestDatabase(t, dsn)
	require.NoError(t, err)

	// Enable citext extension.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, err = sqlDB.Exec("CREATE EXTENSION IF NOT EXISTS citext;")
	require.NoError(t, err)

	// Migrate test database.
	err = db.AutoMigrate(ToAutoMigrate()...)
	require.NoError(t, err)

	// AutoMigrate builds the schema from struct tags, and a partial unique
	// index cannot be expressed as one. Without these the test schema would
	// enforce no uniqueness on file identifiers at all, so a test asserting
	// that a duplicate is rejected would pass against production and fail
	// here -- or worse, the reverse. They mirror migration 000017.
	for _, stmt := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_google_file_id_unique
		   ON documents (google_file_id) WHERE google_file_id <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_file_id_unique
		   ON documents (file_id) WHERE file_id IS NOT NULL AND file_id <> ''`,
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}

	return db, func(_ *testing.T) {
		// TODO: add back and make configurable.
		// err := test.DropTestDatabase(dsn, dbName)
		// require.NoError(t, err)
		//nolint:gosec // dbName is test-controlled, not user input
		log.Printf("would have dropped test database %q here", dbName)
	}
}

// requireDocumentType creates the document type every Document needs.
//
// Document.getAssociations resolves DocumentType unconditionally and has since
// the database APIs were first added, so a document created without one fails
// with "error getting document type: record not found". Tests that are not
// about document types still have to supply one; this keeps that from being
// four lines of noise in each of them.
func requireDocumentType(t *testing.T, db *gorm.DB, name string) DocumentType {
	t.Helper()

	dt := DocumentType{Name: name, LongName: name + " long name"}
	require.NoError(t, dt.FirstOrCreate(db))

	return dt
}

// requireProduct creates the product every persisted Document needs.
//
// documents.product_id is a nullable foreign key, but Document.ProductID is a
// plain uint rather than a pointer, so a document with no product is written
// with product_id = 0 -- which is not NULL and not a product, and the foreign
// key rejects it. Until that field becomes a pointer, every document a test
// persists needs a real product.
func requireProduct(t *testing.T, db *gorm.DB, name, abbreviation string) Product {
	t.Helper()

	p := Product{Name: name, Abbreviation: abbreviation}
	require.NoError(t, p.FirstOrCreate(db))

	return p
}
