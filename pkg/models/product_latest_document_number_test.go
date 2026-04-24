package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductLatestDocumentNumber(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	t.Run("Get and Upsert", func(t *testing.T) {
		db, tearDownTest := setupTest(t, dsn)
		defer tearDownTest(t)

		t.Run(
			"Get latest product document number which won't exist yet (should error)",
			func(t *testing.T) {
				_, requireT := assert.New(t), require.New(t)
				p := ProductLatestDocumentNumber{}
				err := p.Get(db)
				requireT.Error(err)
			})

		var product Product
		t.Run("Create a product", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			product = Product{
				Name:         "product1",
				Abbreviation: "TEST",
			}
			err := product.FirstOrCreate(db)
			requireT.NoError(err)
			assertT.EqualValues(1, product.ID)
			assertT.Equal("product1", product.Name)
			assertT.Equal("TEST", product.Abbreviation)
		})

		t.Run(
			"Try to upsert a new latest product document number with only a product",
			func(t *testing.T) {
				_, requireT := assert.New(t), require.New(t)
				p := ProductLatestDocumentNumber{
					Product: product,
				}
				err := p.Upsert(db)
				requireT.Error(err)
			})

		// Try to upsert a new latest product document number with only a product
		// and latest document number (should error).
		t.Run(
			"Try to upsert a new latest product document number with only a product"+
				"and latest document number",
			func(t *testing.T) {
				_, requireT := assert.New(t), require.New(t)
				p := ProductLatestDocumentNumber{
					Product:              product,
					LatestDocumentNumber: 5,
				}
				err := p.Upsert(db)
				requireT.Error(err)
			})

		var docType DocumentType
		t.Run("Create a document type", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			docType = DocumentType{
				Name:     "RFC",
				LongName: "Request For Comments",
			}
			err := docType.FirstOrCreate(db)
			requireT.NoError(err)
			assertT.NotEmpty(docType.ID)
			assertT.Equal("RFC", docType.Name)
			assertT.Equal("Request For Comments", docType.LongName)
		})

		t.Run("Try to upsert a new latest product document number without a latest"+
			" document number",
			func(t *testing.T) {
				_, requireT := assert.New(t), require.New(t)
				p := ProductLatestDocumentNumber{
					DocumentType: docType,
					Product:      product,
				}
				err := p.Upsert(db)
				requireT.Error(err)
			})

		t.Run("Insert by upserting a new latest product document number",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)
				p := ProductLatestDocumentNumber{
					DocumentType: DocumentType{
						Name: "RFC",
					},
					LatestDocumentNumber: 5,
					Product: Product{
						Name: "product1",
					},
				}
				err := p.Upsert(db)
				requireT.NoError(err)
				assertT.NotEmpty(p.DocumentTypeID)
				assertT.Equal("RFC", p.DocumentType.Name)
				assertT.Equal("Request For Comments", p.DocumentType.LongName)
				assertT.EqualValues(1, p.ProductID)
				assertT.Equal("product1", p.Product.Name)
				assertT.Equal(5, p.LatestDocumentNumber)
			})

		t.Run("Get the latest product document number", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := ProductLatestDocumentNumber{
				DocumentType: DocumentType{
					Name: "RFC",
				},
				Product: Product{
					Name: "product1",
				},
			}
			err := p.Get(db)
			requireT.NoError(err)
			assertT.NotEmpty(p.DocumentTypeID)
			assertT.Equal("RFC", p.DocumentType.Name)
			assertT.Equal("Request For Comments", p.DocumentType.LongName)
			assertT.EqualValues(1, p.ProductID)
			assertT.Equal("product1", p.Product.Name)
			assertT.Equal(5, p.LatestDocumentNumber)
		})

		t.Run("Update by upserting a latest product document number",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)
				p := ProductLatestDocumentNumber{
					DocumentType: DocumentType{
						Name: "RFC",
					},
					LatestDocumentNumber: 10,
					Product: Product{
						Name: "product1",
					},
				}
				err := p.Upsert(db)
				requireT.NoError(err)
				assertT.NotEmpty(p.DocumentTypeID)
				assertT.Equal("RFC", p.DocumentType.Name)
				assertT.Equal("Request For Comments", p.DocumentType.LongName)
				assertT.EqualValues(1, p.ProductID)
				assertT.Equal("product1", p.Product.Name)
				assertT.Equal(10, p.LatestDocumentNumber)
			})

		t.Run("Get the latest product document number", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := ProductLatestDocumentNumber{
				DocumentType: DocumentType{
					Name: "RFC",
				},
				Product: Product{
					Name: "product1",
				},
			}
			err := p.Get(db)
			requireT.NoError(err)
			assertT.NotEmpty(p.DocumentTypeID)
			assertT.Equal("RFC", p.DocumentType.Name)
			assertT.Equal("Request For Comments", p.DocumentType.LongName)
			assertT.EqualValues(1, p.ProductID)
			assertT.Equal("product1", p.Product.Name)
			assertT.Equal(10, p.LatestDocumentNumber)
		})

		t.Run(
			"Insert by upserting a new latest product document number with a "+
				"document type and product that both don't exist yet",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)
				p := ProductLatestDocumentNumber{
					DocumentType: DocumentType{
						Name:     "NEW",
						LongName: "New Document Type",
					},
					LatestDocumentNumber: 1,
					Product: Product{
						Name:         "New Product",
						Abbreviation: "NP",
					},
				}
				err := p.Upsert(db)
				requireT.NoError(err)
				assertT.NotEmpty(p.DocumentTypeID)
				assertT.Equal("NEW", p.DocumentType.Name)
				assertT.EqualValues(2, p.ProductID)
				assertT.Equal("New Product", p.Product.Name)
				assertT.Equal("NP", p.Product.Abbreviation)
				assertT.Equal(1, p.LatestDocumentNumber)
			})
	})
}
