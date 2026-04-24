package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentGroupReviewModel(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	t.Run("Create and Get", func(t *testing.T) {
		db, tearDownTest := setupTest(t, dsn)
		defer tearDownTest(t)

		t.Run("Create a document type", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			dt := DocumentType{
				Name:     "DT1",
				LongName: "DocumentType1",
			}
			err := dt.FirstOrCreate(db)
			requireT.NoError(err)
		})

		t.Run("Create a product", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			p := Product{
				Name:         "Product1",
				Abbreviation: "P1",
			}
			err := p.FirstOrCreate(db)
			requireT.NoError(err)
		})

		t.Run("Get the review before we create the document", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			dr := DocumentGroupReview{
				Document: Document{
					GoogleFileID: "fileID1",
				},
				Group: Group{
					EmailAddress: "team-a@approver.com",
				},
			}
			err := dr.Get(db)
			requireT.Error(err)
		})

		var d Document
		t.Run("Create a document", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d = Document{
				GoogleFileID: "fileID1",
				ApproverGroups: []*Group{
					{
						EmailAddress: "team-a@approver.com",
					},
					{
						EmailAddress: "team-b@approver.com",
					},
				},
				DocumentType: DocumentType{
					Name: "DT1",
				},
				Product: Product{
					Name: "Product1",
				},
			}
			err := d.Create(db)
			requireT.NoError(err)
			assertT.EqualValues(1, d.ID)
		})

		t.Run("Get the review", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			dr := DocumentGroupReview{
				Document: Document{
					GoogleFileID: "fileID1",
				},
				Group: Group{
					EmailAddress: "team-b@approver.com",
				},
			}
			err := dr.Get(db)
			requireT.NoError(err)
			assertT.EqualValues(1, dr.DocumentID)
			assertT.Equal("fileID1", dr.Document.GoogleFileID)
			assertT.EqualValues(2, dr.GroupID)
			assertT.Equal("team-b@approver.com", dr.Group.EmailAddress)
		})
	})

	t.Run("Find", func(t *testing.T) {
		db, tearDownTest := setupTest(t, dsn)
		defer tearDownTest(t)

		t.Run("Create a document type", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			dt := DocumentType{
				Name:     "DT1",
				LongName: "DocumentType1",
			}
			err := dt.FirstOrCreate(db)
			requireT.NoError(err)
		})

		t.Run("Create a product", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			p := Product{
				Name:         "Product1",
				Abbreviation: "P1",
			}
			err := p.FirstOrCreate(db)
			requireT.NoError(err)
		})

		var d1, d2, d3 Document
		t.Run("Create first document", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d1 = Document{
				GoogleFileID: "fileID1",
				ApproverGroups: []*Group{
					{
						EmailAddress: "team-a@approver.com",
					},
					{
						EmailAddress: "team-b@approver.com",
					},
				},
				DocumentType: DocumentType{
					Name: "DT1",
				},
				Product: Product{
					Name: "Product1",
				},
			}
			err := d1.Create(db)
			requireT.NoError(err)
			assertT.EqualValues(1, d1.ID)
		})

		t.Run("Create second document", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d2 = Document{
				GoogleFileID: "fileID2",
				ApproverGroups: []*Group{
					{
						EmailAddress: "team-a@approver.com",
					},
				},
				DocumentType: DocumentType{
					Name: "DT1",
				},
				Product: Product{
					Name: "Product1",
				},
			}
			err := d2.Create(db)
			requireT.NoError(err)
			assertT.EqualValues(2, d2.ID)
		})

		t.Run("Create third document", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			d3 = Document{
				GoogleFileID: "fileID3",
				ApproverGroups: []*Group{
					{
						EmailAddress: "team-b@approver.com",
					},
				},
				DocumentType: DocumentType{
					Name: "DT1",
				},
				Product: Product{
					Name: "Product1",
				},
			}
			err := d3.Create(db)
			requireT.NoError(err)
			assertT.EqualValues(3, d3.ID)
		})

		t.Run("Find reviews without any search fields", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			var revs DocumentGroupReviews
			err := revs.Find(db, DocumentGroupReview{})
			requireT.Error(err)
		})

		t.Run("Find all reviews for a document", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			var revs DocumentGroupReviews
			err := revs.Find(db, DocumentGroupReview{
				Document: Document{
					GoogleFileID: "fileID1",
				},
			})
			requireT.NoError(err)
			requireT.Len(revs, 2)
			assertT.Equal("team-a@approver.com", revs[0].Group.EmailAddress)
			assertT.Equal("team-b@approver.com", revs[1].Group.EmailAddress)
		})

		t.Run("Find all reviews for a group", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			var revs DocumentGroupReviews
			err := revs.Find(db, DocumentGroupReview{
				Group: Group{
					EmailAddress: "team-b@approver.com",
				},
			})
			requireT.NoError(err)
			requireT.Len(revs, 2)
			assertT.Equal("fileID1", revs[0].Document.GoogleFileID)
			assertT.Equal("fileID3", revs[1].Document.GoogleFileID)
			assertT.Equal("team-b@approver.com", revs[0].Group.EmailAddress)
			assertT.Equal("team-b@approver.com", revs[1].Group.EmailAddress)
		})
	})
}

// TestDocumentGroupReviewDualBackend verifies that DocumentGroupReview CRUD
// operations work with both GoogleFileID and FileID backends.
func TestDocumentGroupReviewDualBackend(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	backends := []struct {
		name    string
		makeDoc func(id string) Document
		getID   func(d Document) string
	}{
		{
			name:    "GoogleFileID",
			makeDoc: func(id string) Document { return Document{GoogleFileID: id} },
			getID:   func(d Document) string { return d.GoogleFileID },
		},
		{
			name:    "FileID",
			makeDoc: func(id string) Document { return Document{FileID: id} },
			getID:   func(d Document) string { return d.FileID },
		},
	}

	for _, backend := range backends {
		backend := backend
		t.Run(backend.name, func(t *testing.T) {
			assert, require := assert.New(t), require.New(t)
			db, tearDownTest := setupTest(t, dsn)
			defer tearDownTest(t)

			// Setup.
			dt := DocumentType{Name: "DT1", LongName: "DocumentType1"}
			require.NoError(dt.FirstOrCreate(db))
			p := Product{Name: "Product1", Abbreviation: "P1"}
			require.NoError(p.FirstOrCreate(db))

			// Create a document with approver groups.
			d := backend.makeDoc("grpReviewTestFile1")
			d.DocumentType = DocumentType{Name: "DT1"}
			d.Product = Product{Name: "Product1"}
			d.ApproverGroups = []*Group{
				{EmailAddress: "team-alpha@test.com"},
				{EmailAddress: "team-beta@test.com"},
			}
			require.NoError(d.Create(db))
			assert.Len(d.ApproverGroups, 2)

			// Get a group review.
			t.Run("Get group review", func(t *testing.T) {
				dr := DocumentGroupReview{
					Document: backend.makeDoc("grpReviewTestFile1"),
					Group:    Group{EmailAddress: "team-beta@test.com"},
				}
				err := dr.Get(db)
				require.NoError(err)
				assert.Equal("grpReviewTestFile1", backend.getID(dr.Document))
				assert.Equal("team-beta@test.com", dr.Group.EmailAddress)
			})

			// Find group reviews for document.
			t.Run("Find group reviews", func(t *testing.T) {
				var revs DocumentGroupReviews
				err := revs.Find(db, DocumentGroupReview{
					Document: backend.makeDoc("grpReviewTestFile1"),
				})
				require.NoError(err)
				require.Len(revs, 2)
			})
		})
	}
}
