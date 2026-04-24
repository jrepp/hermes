package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentTypeCustomFieldModel(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	t.Run("Upsert and Get", func(t *testing.T) {
		db, tearDownTest := setupTest(t, dsn)
		defer tearDownTest(t)

		t.Run("Get a document type custom field before any exist",
			func(t *testing.T) {
				requireT := require.New(t)

				d := DocumentTypeCustomField{
					Name: "CustomField1",
					DocumentType: DocumentType{
						Name: "DT1",
					},
				}
				err := d.Get(db)
				requireT.Error(err)
			})

		t.Run("Upsert an empty document type custom field",
			func(t *testing.T) {
				requireT := require.New(t)

				p := DocumentTypeCustomField{}
				err := p.Upsert(db)
				requireT.Error(err)
			})

		t.Run("Create a document type", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d := DocumentType{
				Name:     "DT1",
				LongName: "DocumentType1",
			}
			err := d.FirstOrCreate(db)
			requireT.NoError(err)
			assertT.EqualValues(1, d.ID)
			assertT.Equal("DT1", d.Name)
			assertT.Equal("DocumentType1", d.LongName)
		})

		t.Run("Create a first document type custom field using Upsert",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)

				d := DocumentTypeCustomField{
					Name: "CustomField1",
					DocumentType: DocumentType{
						Name: "DT1",
					},
					Type: StringDocumentTypeCustomFieldType,
				}
				err := d.Upsert(db)
				requireT.NoError(err)
				assertT.EqualValues(1, d.ID)
				assertT.Equal("CustomField1", d.Name)
				requireT.NotNil(d.DocumentType)
				assertT.EqualValues(1, d.DocumentType.ID)
				assertT.Equal("DT1", d.DocumentType.Name)
			})

		t.Run("Get an empty document type custom field",
			func(t *testing.T) {
				requireT := require.New(t)

				p := DocumentTypeCustomField{}
				err := p.Get(db)
				requireT.Error(err)
			})

		t.Run("Upsert the first document type custom field with no changes",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)

				d := DocumentTypeCustomField{
					Name: "CustomField1",
					DocumentType: DocumentType{
						Name: "DT1",
					},
					Type: StringDocumentTypeCustomFieldType,
				}
				err := d.Upsert(db)
				requireT.NoError(err)
				assertT.EqualValues(1, d.ID)
				assertT.Equal("CustomField1", d.Name)
				requireT.NotNil(d.DocumentType)
				assertT.EqualValues(1, d.DocumentType.ID)
				assertT.Equal("DT1", d.DocumentType.Name)
			})

		t.Run("Create a second document type custom field (same document type) "+
			"using Upsert",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)

				d := DocumentTypeCustomField{
					Name: "CustomField2",
					DocumentType: DocumentType{
						Name: "DT1",
					},
					Type: StringDocumentTypeCustomFieldType,
				}
				err := d.Upsert(db)
				requireT.NoError(err)
				assertT.EqualValues(2, d.ID)
				assertT.Equal("CustomField2", d.Name)
				requireT.NotNil(d.DocumentType)
				assertT.EqualValues(1, d.DocumentType.ID)
				assertT.Equal("DT1", d.DocumentType.Name)
			})

		t.Run("Create a second document type", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d := DocumentType{
				Name:     "DT2",
				LongName: "DocumentType2",
			}
			err := d.FirstOrCreate(db)
			requireT.NoError(err)
			assertT.EqualValues(2, d.ID)
			assertT.Equal("DT2", d.Name)
			assertT.Equal("DocumentType2", d.LongName)
		})

		t.Run("Create a third document type custom field (same name) "+
			"using Upsert",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)

				d := DocumentTypeCustomField{
					Name: "CustomField1",
					DocumentType: DocumentType{
						Name: "DT2",
					},
					Type: StringDocumentTypeCustomFieldType,
				}
				err := d.Upsert(db)
				requireT.NoError(err)
				assertT.EqualValues(3, d.ID)
				assertT.Equal("CustomField1", d.Name)
				requireT.NotNil(d.DocumentType)
				assertT.EqualValues(2, d.DocumentType.ID)
				assertT.Equal("DT2", d.DocumentType.Name)
			})
	})
}
