package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

func setupProjectSearchOutboxTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.ProjectRelatedResource{},
		&models.ProjectRelatedResourceExternalLink{},
		&models.ProjectRelatedResourceHermesDocument{},
		&models.SearchOutboxSequence{},
		&models.SearchOutboxEvent{},
	))

	return db
}

func TestCreateProjectWithSearchOutbox(t *testing.T) {
	db := setupProjectSearchOutboxTestDB(t)
	proj := &models.Project{
		Creator: models.User{
			EmailAddress: "owner@example.com",
		},
		Title: "Project One",
	}

	require.NoError(t, createProjectWithSearchOutbox(db, proj))
	assert.NotZero(t, proj.ID)

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, models.SearchEventProjectCreated, events[0].EventType)
	assert.Equal(t, models.SearchAggregateProject, events[0].AggregateType)
	assert.Equal(t, models.SearchIndexProjects, events[0].IndexName)
	assert.Equal(t, models.SearchOutboxOperationUpsert, events[0].Operation)
	assert.Equal(t, int64(1), events[0].Sequence)
	assert.Equal(t, "Project One", events[0].Payload["title"])
	assert.Equal(t, "owner@example.com", events[0].Payload["creator"])
}

func TestUpdateProjectWithSearchOutbox(t *testing.T) {
	db := setupProjectSearchOutboxTestDB(t)
	proj := &models.Project{
		Creator: models.User{
			EmailAddress: "owner@example.com",
		},
		Title: "Project One",
	}
	require.NoError(t, createProjectWithSearchOutbox(db, proj))

	patch := &models.Project{
		Model: gorm.Model{
			ID: proj.ID,
		},
		Title: "Project Renamed",
	}
	require.NoError(t, updateProjectWithSearchOutbox(db, patch))

	var events []models.SearchOutboxEvent
	require.NoError(t, db.Order("sequence ASC").Find(&events).Error)
	require.Len(t, events, 2)
	assert.Equal(t, models.SearchEventProjectCreated, events[0].EventType)
	assert.Equal(t, int64(1), events[0].Sequence)
	assert.Equal(t, models.SearchEventProjectUpdated, events[1].EventType)
	assert.Equal(t, int64(2), events[1].Sequence)
	assert.Equal(t, "Project Renamed", events[1].Payload["title"])
	assert.Equal(t, "owner@example.com", events[1].Payload["creator"])
}
