-- Add missing fields to document_types table
-- These fields exist in the DocumentType model but were omitted from initial migration

ALTER TABLE document_types 
  ADD COLUMN IF NOT EXISTS flight_icon TEXT,
  ADD COLUMN IF NOT EXISTS more_info_link_text TEXT,
  ADD COLUMN IF NOT EXISTS more_info_link_url TEXT,
  ADD COLUMN IF NOT EXISTS checks JSONB;

-- Note: Using JSONB for checks field (PostgreSQL)
-- SQLite will use TEXT for JSON storage automatically
