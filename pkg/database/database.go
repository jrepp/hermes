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

	if log != nil {
		log.Info("connected to database",
			"host", cfg.Host,
			"database", cfg.DBName,
		)
	}

	return db, nil
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
