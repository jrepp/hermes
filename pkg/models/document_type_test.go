package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDocumentType(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	t.Run("FirstOrCreate and Get", func(t *testing.T) {
		assertT, requireT := assert.New(t), require.New(t)
		db, tearDownTest := setupTest(t, dsn)
		defer tearDownTest(t)

		// Get document type, which won't exist yet (should error).
		d := DocumentType{
			Name: "DT1",
		}
		err := d.Get(db)
		requireT.Error(err)
		requireT.ErrorIs(gorm.ErrRecordNotFound, err)

		// Create a document type.
		d = DocumentType{
			Name:     "DT1",
			LongName: "DocumentType1",
		}
		err = d.FirstOrCreate(db)
		requireT.NoError(err)
		assertT.EqualValues(1, d.ID)
		assertT.Equal("DT1", d.Name)
		assertT.Equal("DocumentType1", d.LongName)

		// Get the document type.
		d = DocumentType{
			Name: "DT1",
		}
		err = d.Get(db)
		requireT.NoError(err)
		assertT.EqualValues(1, d.ID)
		assertT.Equal("DT1", d.Name)
		assertT.Equal("DocumentType1", d.LongName)

		// Create another document type.
		d = DocumentType{
			Name:     "DT2",
			LongName: "DocumentType2",
		}
		err = d.FirstOrCreate(db)
		requireT.NoError(err)
		assertT.EqualValues(2, d.ID)
		assertT.Equal("DT2", d.Name)
		assertT.Equal("DocumentType2", d.LongName)

		// Get the document type.
		d = DocumentType{
			Name: "DT2",
		}
		err = d.Get(db)
		requireT.NoError(err)
		assertT.EqualValues(2, d.ID)
		assertT.Equal("DT2", d.Name)
		assertT.Equal("DocumentType2", d.LongName)

		// Get all document types.
		ds := DocumentTypes{}
		err = ds.GetAll(db)
		requireT.NoError(err)
		requireT.Len(ds, 2)
		assertT.EqualValues(1, ds[0].ID)
		assertT.Equal("DT1", ds[0].Name)
		assertT.Equal("DocumentType1", ds[0].LongName)
		assertT.EqualValues(2, ds[1].ID)
		assertT.Equal("DT2", ds[1].Name)
		assertT.Equal("DocumentType2", ds[1].LongName)
	})

	t.Run("FirstOrCreate with custom fields", func(t *testing.T) {
		db, tearDownTest := setupTest(t, dsn)
		defer tearDownTest(t)

		t.Run("Create document type", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d := DocumentType{
				Name:     "DT1",
				LongName: "DocumentType1",
				CustomFields: []DocumentTypeCustomField{
					{
						Name: "CustomStringField",
						Type: StringDocumentTypeCustomFieldType,
					},
					{
						Name: "CustomPersonField",
						Type: PersonDocumentTypeCustomFieldType,
					},
					{
						Name: "CustomPeopleField",
						Type: PeopleDocumentTypeCustomFieldType,
					},
				},
			}
			err := d.FirstOrCreate(db)
			requireT.NoError(err)
			assertT.EqualValues(1, d.ID)
			assertT.Equal("DT1", d.Name)
			assertT.Equal("DocumentType1", d.LongName)
			requireT.Len(d.CustomFields, 3)
			assertT.Equal("CustomStringField", d.CustomFields[0].Name)
			assertT.Equal(StringDocumentTypeCustomFieldType, d.CustomFields[0].Type)
			assertT.Equal("CustomPersonField", d.CustomFields[1].Name)
			assertT.Equal(PersonDocumentTypeCustomFieldType, d.CustomFields[1].Type)
			assertT.Equal("CustomPeopleField", d.CustomFields[2].Name)
			assertT.Equal(PeopleDocumentTypeCustomFieldType, d.CustomFields[2].Type)
		})

		t.Run("Get document type", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d := DocumentType{
				Name: "DT1",
			}
			err := d.Get(db)
			requireT.NoError(err)
			assertT.EqualValues(1, d.ID)
			assertT.Equal("DT1", d.Name)
			assertT.Equal("DocumentType1", d.LongName)
			requireT.Len(d.CustomFields, 3)
			assertT.Equal("CustomStringField", d.CustomFields[0].Name)
			assertT.Equal(StringDocumentTypeCustomFieldType, d.CustomFields[0].Type)
			assertT.Equal("CustomPersonField", d.CustomFields[1].Name)
			assertT.Equal(PersonDocumentTypeCustomFieldType, d.CustomFields[1].Type)
			assertT.Equal("CustomPeopleField", d.CustomFields[2].Name)
			assertT.Equal(PeopleDocumentTypeCustomFieldType, d.CustomFields[2].Type)
		})
	})
}
