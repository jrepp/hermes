package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IndexerToken represents an authentication token for an indexer.
type IndexerToken struct {
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	Indexer       *Indexer       `gorm:"foreignKey:IndexerID" json:"-"`
	IndexerID     *uuid.UUID     `gorm:"type:uuid;index" json:"indexer_id,omitempty"`
	RevokedAt     *time.Time     `json:"revoked_at,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	TokenType     string         `gorm:"type:varchar(50);default:'api'" json:"token_type"`
	RevokedReason string         `gorm:"type:text" json:"revoked_reason,omitempty"`
	TokenHash     string         `gorm:"type:varchar(256);not null;uniqueIndex" json:"-"`
	Metadata      string         `gorm:"type:text" json:"metadata,omitempty"`
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Revoked       bool           `gorm:"default:false" json:"revoked"`
}

// BeforeCreate hook to generate UUID if not set.
func (t *IndexerToken) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (IndexerToken) TableName() string {
	return "service_tokens"
}

// IndexerTokens is a slice of indexer tokens.
type IndexerTokens []IndexerToken

// GenerateToken creates a new random token with the format:
// hermes-<type>-token-<uuid>-<random-suffix>
func GenerateToken(tokenType string) (string, error) {
	// Generate UUID
	id := uuid.New()

	// Generate random suffix (8 bytes = 16 hex characters)
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("error generating random bytes: %w", err)
	}
	suffix := hex.EncodeToString(randomBytes)

	// Construct token
	token := fmt.Sprintf("hermes-%s-token-%s-%s", tokenType, id.String(), suffix)
	return token, nil
}

// HashToken creates a SHA-256 hash of a token for secure storage.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Create creates a new token in the database.
// The token parameter should be the plaintext token to hash.
func (t *IndexerToken) Create(db *gorm.DB, token string) error {
	t.TokenHash = HashToken(token)
	return db.Create(t).Error
}

// Get retrieves a token by ID.
func (t *IndexerToken) Get(db *gorm.DB) error {
	return db.Preload("Indexer").First(t, "id = ?", t.ID).Error
}

// GetByHash retrieves a token by its hash.
func (t *IndexerToken) GetByHash(db *gorm.DB, tokenHash string) error {
	return db.Preload("Indexer").First(t, "token_hash = ?", tokenHash).Error
}

// GetByToken retrieves a token by its plaintext value.
func (t *IndexerToken) GetByToken(db *gorm.DB, token string) error {
	return t.GetByHash(db, HashToken(token))
}

// Revoke marks the token as revoked.
func (t *IndexerToken) Revoke(db *gorm.DB, reason string) error {
	now := time.Now()
	t.Revoked = true
	t.RevokedAt = &now
	t.RevokedReason = reason
	return db.Model(t).Updates(map[string]interface{}{
		"revoked":        true,
		"revoked_at":     now,
		"revoked_reason": reason,
	}).Error
}

// IsValid checks if the token is valid (not expired, not revoked).
func (t *IndexerToken) IsValid() bool {
	if t.Revoked {
		return false
	}

	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return false
	}

	return true
}

// FindAll retrieves all tokens.
func (ts *IndexerTokens) FindAll(db *gorm.DB) error {
	return db.Preload("Indexer").Find(ts).Error
}

// FindByIndexer retrieves all tokens for a specific indexer.
func (ts *IndexerTokens) FindByIndexer(db *gorm.DB, indexerID uuid.UUID) error {
	return db.Preload("Indexer").Where("indexer_id = ?", indexerID).Find(ts).Error
}

// FindValid retrieves all valid (not revoked, not expired) tokens.
func (ts *IndexerTokens) FindValid(db *gorm.DB) error {
	return db.Preload("Indexer").
		Where("revoked = ? AND (expires_at IS NULL OR expires_at > ?)", false, time.Now()).
		Find(ts).Error
}
