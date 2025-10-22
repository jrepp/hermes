package indexer
package indexer

import "context"

// Command represents a single operation in the indexer pipeline.
// Each command performs a specific transformation or action on a document context.
type Command interface {
	// Execute performs the command operation on a document context.
	Execute(ctx context.Context, doc *DocumentContext) error

	// Name returns the command name for logging and debugging.
	Name() string
}

// BatchCommand is an optional interface that commands can implement
// to process multiple documents more efficiently.
// For example, indexing commands can batch multiple documents into
// a single search index operation.
type BatchCommand interface {
	Command

	// ExecuteBatch processes multiple documents at once.
	// Returns an error if any document fails processing.
	ExecuteBatch(ctx context.Context, docs []*DocumentContext) error
}

// DiscoverCommand is a special command that discovers documents
// rather than processing existing document contexts.
type DiscoverCommand interface {
	Command

	// Discover returns documents that match the command's criteria.
	Discover(ctx context.Context) ([]*DocumentContext, error)
}
