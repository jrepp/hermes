package database

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds configuration for database connection.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string

	// Connection pool settings (RFC-088 optimization)
	MaxIdleConns    int           // Maximum idle connections in pool (default: 10)
	MaxOpenConns    int           // Maximum open connections (default: 25)
	ConnMaxLifetime time.Duration // Maximum connection lifetime (default: 5 minutes)
	ConnMaxIdleTime time.Duration // Maximum connection idle time (default: 10 minutes)
}

// Connect establishes a database connection using the provided configuration.
// This is the shared database connection logic used by all binaries.
func Connect(cfg Config, log hclog.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)

	// Create GORM config with optional logger
	gormConfig := &gorm.Config{}
	if log != nil {
		gormConfig.Logger = NewGormLogger(log.Named("gorm"))
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pooling for optimal performance (RFC-088)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Apply connection pool settings with sensible defaults
	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 10 // Default: maintain 10 idle connections
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)

	maxOpenConns := cfg.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 25 // Default: allow up to 25 concurrent connections
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)

	connMaxLifetime := cfg.ConnMaxLifetime
	if connMaxLifetime == 0 {
		connMaxLifetime = 5 * time.Minute // Default: recycle connections after 5 minutes
	}
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	connMaxIdleTime := cfg.ConnMaxIdleTime
	if connMaxIdleTime == 0 {
		connMaxIdleTime = 10 * time.Minute // Default: close idle connections after 10 minutes
	}
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	if log != nil {
		log.Info("connected to database with connection pooling",
			"host", cfg.Host,
			"database", cfg.DBName,
			"max_idle_conns", maxIdleConns,
			"max_open_conns", maxOpenConns,
			"conn_max_lifetime", connMaxLifetime,
			"conn_max_idle_time", connMaxIdleTime,
		)
	}

	return db, nil
}

// PoolStats holds database connection pool statistics.
type PoolStats struct {
	MaxOpenConnections int           // Maximum number of open connections to the database
	OpenConnections    int           // The number of established connections both in use and idle
	InUse              int           // The number of connections currently in use
	Idle               int           // The number of idle connections
	WaitCount          int64         // The total number of connections waited for
	WaitDuration       time.Duration // The total time blocked waiting for a new connection
	MaxIdleClosed      int64         // The total number of connections closed due to SetMaxIdleConns
	MaxIdleTimeClosed  int64         // The total number of connections closed due to SetConnMaxIdleTime
	MaxLifetimeClosed  int64         // The total number of connections closed due to SetConnMaxLifetime
}

// GetPoolStats returns connection pool statistics from a GORM DB instance.
// Useful for monitoring and debugging connection pool performance.
func GetPoolStats(db *gorm.DB) (*PoolStats, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	stats := sqlDB.Stats()
	return &PoolStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxIdleTimeClosed:  stats.MaxIdleTimeClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}, nil
}

// gormHclogAdapter adapts hclog.Logger to gorm.logger.Interface.
type gormHclogAdapter struct {
	logger hclog.Logger
	level  logger.LogLevel
}

// NewGormLogger creates a new GORM logger that uses hclog.
func NewGormLogger(log hclog.Logger) logger.Interface {
	return &gormHclogAdapter{
		logger: log,
		level:  logger.Info,
	}
}

// LogMode sets the log level for GORM queries.
func (g *gormHclogAdapter) LogMode(level logger.LogLevel) logger.Interface {
	return &gormHclogAdapter{
		logger: g.logger,
		level:  level,
	}
}

// Info logs info messages.
func (g *gormHclogAdapter) Info(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= logger.Info && g.logger != nil {
		g.logger.Info(msg, data...)
	}
}

// Warn logs warning messages.
func (g *gormHclogAdapter) Warn(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= logger.Warn && g.logger != nil {
		g.logger.Warn(msg, data...)
	}
}

// Error logs error messages.
func (g *gormHclogAdapter) Error(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= logger.Error && g.logger != nil {
		g.logger.Error(msg, data...)
	}
}

// Trace logs SQL queries and execution time.
func (g *gormHclogAdapter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.level <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	if err != nil && g.level >= logger.Error {
		g.logger.Error("database query failed",
			"error", err,
			"elapsed", elapsed,
			"rows", rows,
			"sql", sql,
		)
	} else if elapsed > 200*time.Millisecond && g.level >= logger.Warn {
		g.logger.Warn("slow database query",
			"elapsed", elapsed,
			"rows", rows,
			"sql", sql,
		)
	} else if g.level >= logger.Info {
		g.logger.Debug("database query",
			"elapsed", elapsed,
			"rows", rows,
			"sql", sql,
		)
	}
}
