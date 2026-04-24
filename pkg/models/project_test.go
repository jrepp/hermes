package models

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProject(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	t.Run("Create, Get, and Update", func(t *testing.T) {
		db, tearDownTest := setupTest(t, dsn)
		defer tearDownTest(t)

		t.Run("Create a project without a Creator", func(t *testing.T) {
			assertT, _ := assert.New(t), require.New(t)
			p := Project{
				Title: "Title1",
			}
			err := p.Create(db)
			assertT.Error(err)
			assertT.Empty(p.ID)
		})

		t.Run("Create a project without a Title", func(t *testing.T) {
			assertT, _ := assert.New(t), require.New(t)
			p := Project{
				Creator: User{
					EmailAddress: "a@a.com",
				},
			}
			err := p.Create(db)
			assertT.Error(err)
			assertT.Empty(p.ID)
		})

		t.Run("Create a minimal project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{
				Creator: User{
					EmailAddress: "a@a.com",
				},
				Title: "Title1",
			}
			err := p.Create(db)
			requireT.NoError(err)
			assertT.Equal("a@a.com", p.Creator.EmailAddress)
			assertT.EqualValues(1, p.Creator.ID)
			assertT.EqualValues(1, p.CreatorID)
			assertT.EqualValues(1, p.ID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("Title1", p.Title)
		})

		t.Run("Get the project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{}

			err := p.Get(db, 1)
			requireT.NoError(err)
			assertT.Equal("a@a.com", p.Creator.EmailAddress)
			assertT.EqualValues(1, p.Creator.ID)
			assertT.EqualValues(1, p.CreatorID)
			assertT.EqualValues(1, p.ID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("Title1", p.Title)
		})

		t.Run("Update the project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{
				Model: gorm.Model{
					ID: 1,
				},
				Title: "UpdatedTitle1",
			}

			err := p.Update(db)
			requireT.NoError(err)
			assertT.Equal("a@a.com", p.Creator.EmailAddress)
			assertT.EqualValues(1, p.Creator.ID)
			assertT.EqualValues(1, p.CreatorID)
			assertT.EqualValues(1, p.ID)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.NotEqualf(p.ProjectCreatedAt, p.ProjectModifiedAt,
				"ProjectModifiedAt should not be equal to ProjectCreatedAt")
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("UpdatedTitle1", p.Title)
		})

		t.Run("Get the project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{}

			err := p.Get(db, 1)
			requireT.NoError(err)
			assertT.Equal("a@a.com", p.Creator.EmailAddress)
			assertT.EqualValues(1, p.Creator.ID)
			assertT.EqualValues(1, p.CreatorID)
			assertT.EqualValues(1, p.ID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("UpdatedTitle1", p.Title)
		})

		t.Run("Update a project that doesn't exist", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			p := Project{
				Model: gorm.Model{
					ID: 10,
				},
				Title: "UpdatedTitle1",
			}

			err := p.Update(db)
			requireT.Error(err)
		})

		t.Run("Create a second project with all fields", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{
				Creator: User{
					EmailAddress: "b@b.com",
				},
				Description: &[]string{"Description2"}[0],
				JiraIssueID: &[]string{"JiraIssueID2"}[0],
				Title:       "Title2",
			}
			err := p.Create(db)
			requireT.NoError(err)
			assertT.Equal("b@b.com", p.Creator.EmailAddress)
			assertT.EqualValues(2, p.Creator.ID)
			assertT.EqualValues(2, p.CreatorID)
			assertT.EqualValues(2, p.ID)
			requireT.NotNil(p.Description)
			assertT.Equal("Description2", *p.Description)
			requireT.NotNil(p.JiraIssueID)
			assertT.Equal("JiraIssueID2", *p.JiraIssueID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("Title2", p.Title)
		})

		t.Run("Get the second project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{}

			err := p.Get(db, 2)
			requireT.NoError(err)
			assertT.Equal("b@b.com", p.Creator.EmailAddress)
			assertT.EqualValues(2, p.Creator.ID)
			assertT.EqualValues(2, p.CreatorID)
			assertT.EqualValues(2, p.ID)
			requireT.NotNil(p.Description)
			assertT.Equal("Description2", *p.Description)
			requireT.NotNil(p.JiraIssueID)
			assertT.Equal("JiraIssueID2", *p.JiraIssueID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("Title2", p.Title)
		})

		t.Run("Update the second project to remove optional fields",
			func(t *testing.T) {
				assertT, requireT := assert.New(t), require.New(t)
				p := Project{
					Model: gorm.Model{
						ID: 2,
					},
					Description: &[]string{""}[0],
					JiraIssueID: &[]string{""}[0],
				}

				err := p.Update(db)
				requireT.NoError(err)
				assertT.Equal("b@b.com", p.Creator.EmailAddress)
				assertT.EqualValues(2, p.Creator.ID)
				assertT.EqualValues(2, p.CreatorID)
				assertT.EqualValues(2, p.ID)
				requireT.NotNil(p.Description)
				assertT.Equal("", *p.Description)
				requireT.NotNil(p.JiraIssueID)
				assertT.Equal("", *p.JiraIssueID)
				assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
				assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
				assertT.Equal(ActiveProjectStatus, p.Status)
				assertT.Equal("Title2", p.Title)
			})

		t.Run("Get the second project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{}

			err := p.Get(db, 2)
			requireT.NoError(err)
			assertT.Equal("b@b.com", p.Creator.EmailAddress)
			assertT.EqualValues(2, p.Creator.ID)
			assertT.EqualValues(2, p.CreatorID)
			assertT.EqualValues(2, p.ID)
			requireT.NotNil(p.Description)
			assertT.Equal("", *p.Description)
			requireT.NotNil(p.JiraIssueID)
			assertT.Equal("", *p.JiraIssueID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("Title2", p.Title)
		})

		t.Run("Update the second project again", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{
				Model: gorm.Model{
					ID: 2,
				},
				Description: &[]string{"UpdatedDescription2"}[0],
				JiraIssueID: &[]string{"UpdatedJiraIssueID2"}[0],
				Title:       "UpdatedTitle2",
			}

			err := p.Update(db)
			requireT.NoError(err)
			assertT.Equal("b@b.com", p.Creator.EmailAddress)
			assertT.EqualValues(2, p.Creator.ID)
			assertT.EqualValues(2, p.CreatorID)
			assertT.EqualValues(2, p.ID)
			requireT.NotNil(p.Description)
			assertT.Equal("UpdatedDescription2", *p.Description)
			requireT.NotNil(p.JiraIssueID)
			assertT.Equal("UpdatedJiraIssueID2", *p.JiraIssueID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("UpdatedTitle2", p.Title)
		})

		t.Run("Get the second project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{}

			err := p.Get(db, 2)
			requireT.NoError(err)
			assertT.Equal("b@b.com", p.Creator.EmailAddress)
			assertT.EqualValues(2, p.Creator.ID)
			assertT.EqualValues(2, p.CreatorID)
			assertT.EqualValues(2, p.ID)
			requireT.NotNil(p.Description)
			assertT.Equal("UpdatedDescription2", *p.Description)
			requireT.NotNil(p.JiraIssueID)
			assertT.Equal("UpdatedJiraIssueID2", *p.JiraIssueID)
			assertT.WithinDuration(time.Now(), p.ProjectCreatedAt, 1*time.Second)
			assertT.WithinDuration(time.Now(), p.ProjectModifiedAt, 1*time.Second)
			assertT.Equal(ActiveProjectStatus, p.Status)
			assertT.Equal("UpdatedTitle2", p.Title)
		})
	})
}

func TestProjectReplaceRelatedResources(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	t.Run("Get and Replace", func(t *testing.T) {
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

		t.Run("Create documents", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			d := Document{
				GoogleFileID: "GoogleFileID1",
				DocumentType: DocumentType{
					Name: "DT1",
				},
				Product: Product{
					Name: "Product1",
				},
			}
			err := d.Create(db)
			requireT.NoError(err)

			d = Document{
				GoogleFileID: "GoogleFileID2",
				DocumentType: DocumentType{
					Name: "DT1",
				},
				Product: Product{
					Name: "Product1",
				},
			}
			err = d.Create(db)
			requireT.NoError(err)

			d = Document{
				GoogleFileID: "GoogleFileID3",
				DocumentType: DocumentType{
					Name: "DT1",
				},
				Product: Product{
					Name: "Product1",
				},
			}
			err = d.Create(db)
			requireT.NoError(err)
		})

		t.Run("Create a project", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			p := Project{
				Creator: User{
					EmailAddress: "a@a.com",
				},
				Title: "Title1",
			}
			err := p.Create(db)
			requireT.NoError(err)
			requireT.EqualValues(1, p.ID)
		})

		t.Run("Add external link related resources", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)

			rr := ProjectRelatedResourceExternalLink{
				RelatedResource: ProjectRelatedResource{
					ProjectID: 1,
					SortOrder: 1,
				},
				Name: "Name1",
				URL:  "URL1",
			}
			err := rr.Create(db)
			requireT.NoError(err)

			rr = ProjectRelatedResourceExternalLink{
				RelatedResource: ProjectRelatedResource{
					ProjectID: 1,
					SortOrder: 2,
				},
				Name: "Name2",
				URL:  "URL2",
			}
			err = rr.Create(db)
			requireT.NoError(err)

			rr = ProjectRelatedResourceExternalLink{
				RelatedResource: ProjectRelatedResource{
					ProjectID: 1,
					SortOrder: 3,
				},
				Name: "Name3",
				URL:  "URL3",
			}
			err = rr.Create(db)
			requireT.NoError(err)
		})

		t.Run("Get the project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{}

			err := p.Get(db, 1)
			requireT.NoError(err)
			assertT.Len(p.RelatedResources, 3)
		})

		t.Run("Replace related resources", func(t *testing.T) {
			_, requireT := assert.New(t), require.New(t)
			p := Project{
				Model: gorm.Model{
					ID: 1,
				},
			}
			err := p.ReplaceRelatedResources(db,
				[]ProjectRelatedResourceExternalLink{
					{
						RelatedResource: ProjectRelatedResource{
							ProjectID: 1,
							SortOrder: 1,
						},
						Name: "Name4",
						URL:  "URL4",
					},
				},
				[]ProjectRelatedResourceHermesDocument{
					{
						RelatedResource: ProjectRelatedResource{
							ProjectID: 1,
							SortOrder: 2,
						},
						Document: Document{
							GoogleFileID: "GoogleFileID1",
						},
					},
					{
						RelatedResource: ProjectRelatedResource{
							ProjectID: 1,
							SortOrder: 3,
						},
						Document: Document{
							GoogleFileID: "GoogleFileID3",
						},
					},
				},
			)
			requireT.NoError(err)
		})

		t.Run("Get the project", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{}

			err := p.Get(db, 1)
			requireT.NoError(err)
			assertT.Len(p.RelatedResources, 3)
		})

		t.Run("Get typed related resources", func(t *testing.T) {
			assertT, requireT := assert.New(t), require.New(t)
			p := Project{
				Model: gorm.Model{
					ID: 1,
				},
			}
			elrrs, hdrrs, err := p.GetRelatedResources(db)
			requireT.NoError(err)
			requireT.Len(elrrs, 1)
			assertT.Equal("Name4", elrrs[0].Name)
			assertT.Equal("URL4", elrrs[0].URL)
			assertT.Equal(1, elrrs[0].RelatedResource.SortOrder)
			requireT.Len(hdrrs, 2)
			assertT.Equal("GoogleFileID1", hdrrs[0].Document.GoogleFileID)
			assertT.Equal(2, hdrrs[0].RelatedResource.SortOrder)
			assertT.Equal("GoogleFileID3", hdrrs[1].Document.GoogleFileID)
			assertT.Equal(3, hdrrs[1].RelatedResource.SortOrder)
		})
	})
}
