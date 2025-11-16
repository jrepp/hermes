package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestConnectionPoolDefaults tests that connection pool defaults are applied correctly.
func TestConnectionPoolDefaults(t *testing.T) {
	// Use SQLite for testing (no external database needed)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Get underlying SQL DB
	sqlDB, err := db.DB()
	require.NoError(t, err)

	// Apply default connection pool settings (mimicking Connect function behavior)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	// Get stats
	stats := sqlDB.Stats()

	// Verify defaults were applied
	assert.Equal(t, 25, stats.MaxOpenConnections, "max open connections should be 25")

	// Note: MaxIdleConns and other settings don't appear in Stats directly,
	// but we verify they were set without error
}

// TestConnectionPoolCustomSettings tests that custom connection pool settings are respected.
func TestConnectionPoolCustomSettings(t *testing.T) {
	// Use SQLite for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Get underlying SQL DB
	sqlDB, err := db.DB()
	require.NoError(t, err)

	// Apply custom connection pool settings
	customMaxIdle := 5
	customMaxOpen := 50
	customLifetime := 3 * time.Minute
	customIdleTime := 7 * time.Minute

	sqlDB.SetMaxIdleConns(customMaxIdle)
	sqlDB.SetMaxOpenConns(customMaxOpen)
	sqlDB.SetConnMaxLifetime(customLifetime)
	sqlDB.SetConnMaxIdleTime(customIdleTime)

	// Get stats
	stats := sqlDB.Stats()

	// Verify custom settings were applied
	assert.Equal(t, customMaxOpen, stats.MaxOpenConnections, "max open connections should match custom value")
}

// TestGetPoolStats tests the GetPoolStats function.
func TestGetPoolStats(t *testing.T) {
	// Use SQLite for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Configure connection pool
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(25)

	// Get pool stats
	poolStats, err := GetPoolStats(db)
	require.NoError(t, err)
	require.NotNil(t, poolStats)

	// Verify stats are reasonable
	assert.Equal(t, 25, poolStats.MaxOpenConnections, "max open connections should be 25")
	assert.GreaterOrEqual(t, poolStats.OpenConnections, 0, "open connections should be non-negative")
	assert.GreaterOrEqual(t, poolStats.InUse, 0, "in-use connections should be non-negative")
	assert.GreaterOrEqual(t, poolStats.Idle, 0, "idle connections should be non-negative")
	assert.Equal(t, poolStats.OpenConnections, poolStats.InUse+poolStats.Idle, "open = in-use + idle")
}

// TestConnectionPoolUnderLoad tests connection pool behavior under concurrent load.
func TestConnectionPoolUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	// Use SQLite for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Configure small connection pool to test pooling behavior
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetMaxOpenConns(5)

	// Simulate concurrent queries using SQLite's built-in sqlite_master table
	const numQueries = 20
	done := make(chan bool, numQueries)

	for i := 0; i < numQueries; i++ {
		go func(id int) {
			// Each goroutine performs a simple query against SQLite's system table
			var count int64
			err := db.Raw("SELECT COUNT(*) FROM sqlite_master").Scan(&count).Error
			if err != nil {
				t.Errorf("query %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all queries to complete
	for i := 0; i < numQueries; i++ {
		<-done
	}

	// Get final stats
	poolStats, err := GetPoolStats(db)
	require.NoError(t, err)

	// Verify connection pool behaved correctly
	assert.LessOrEqual(t, poolStats.OpenConnections, 5, "should not exceed max open connections")
	assert.GreaterOrEqual(t, poolStats.WaitCount, int64(0), "wait count should be non-negative")

	// Log stats for visibility
	t.Logf("Connection pool stats after load test:")
	t.Logf("  Max open: %d", poolStats.MaxOpenConnections)
	t.Logf("  Open: %d", poolStats.OpenConnections)
	t.Logf("  In use: %d", poolStats.InUse)
	t.Logf("  Idle: %d", poolStats.Idle)
	t.Logf("  Wait count: %d", poolStats.WaitCount)
	t.Logf("  Wait duration: %v", poolStats.WaitDuration)
}

// TestConnectionPoolStatsFields tests that all PoolStats fields are populated.
func TestConnectionPoolStatsFields(t *testing.T) {
	// Use SQLite for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Configure connection pool
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	// Get pool stats
	poolStats, err := GetPoolStats(db)
	require.NoError(t, err)

	// Verify all fields are present (even if zero)
	assert.GreaterOrEqual(t, poolStats.MaxOpenConnections, 0)
	assert.GreaterOrEqual(t, poolStats.OpenConnections, 0)
	assert.GreaterOrEqual(t, poolStats.InUse, 0)
	assert.GreaterOrEqual(t, poolStats.Idle, 0)
	assert.GreaterOrEqual(t, poolStats.WaitCount, int64(0))
	assert.GreaterOrEqual(t, poolStats.WaitDuration, time.Duration(0))
	assert.GreaterOrEqual(t, poolStats.MaxIdleClosed, int64(0))
	assert.GreaterOrEqual(t, poolStats.MaxIdleTimeClosed, int64(0))
	assert.GreaterOrEqual(t, poolStats.MaxLifetimeClosed, int64(0))
}
