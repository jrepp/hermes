package indexer

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
)

// Pipeline executes a sequence of commands on documents.
// It handles filtering, parallel processing, and error collection.
type Pipeline struct {
	Name        string
	Description string
	Commands    []Command
	Filter      DocumentFilter
	Logger      hclog.Logger

	// Configuration
	MaxParallel int // Maximum number of documents to process in parallel
}

// Execute runs the pipeline on a set of documents.
// Documents are filtered first, then processed through each command
// in sequence. Commands that implement BatchCommand can process
// multiple documents at once for efficiency.
func (p *Pipeline) Execute(ctx context.Context, docs []*DocumentContext) error {
	p.Logger.Info("starting pipeline",
		"name", p.Name,
		"documents", len(docs),
	)

	// Apply filter
	filtered := docs
	if p.Filter != nil {
		filtered = make([]*DocumentContext, 0, len(docs))
		for _, doc := range docs {
			if p.Filter(doc) {
				filtered = append(filtered, doc)
			}
		}
		if len(filtered) < len(docs) {
			p.Logger.Info("filtered documents",
				"before", len(docs),
				"after", len(filtered),
			)
		}
	}

	if len(filtered) == 0 {
		p.Logger.Info("no documents to process after filtering")
		return nil
	}

	// Execute commands in sequence
	for _, cmd := range p.Commands {
		p.Logger.Debug("executing command",
			"name", cmd.Name(),
			"documents", len(filtered),
		)

		// Check if this is a discover command
		if discoverCmd, ok := cmd.(DiscoverCommand); ok {
			discovered, err := discoverCmd.Discover(ctx)
			if err != nil {
				return fmt.Errorf("discover command %s failed: %w", cmd.Name(), err)
			}
			filtered = discovered
			p.Logger.Info("discovered documents",
				"command", cmd.Name(),
				"count", len(discovered),
			)
			continue
		}

		// Check if command supports batch processing
		if batchCmd, ok := cmd.(BatchCommand); ok {
			if err := batchCmd.ExecuteBatch(ctx, filtered); err != nil {
				return fmt.Errorf("batch command %s failed: %w", cmd.Name(), err)
			}
		} else {
			// Process documents in parallel
			if err := p.executeParallel(ctx, cmd, filtered); err != nil {
				return fmt.Errorf("command %s failed: %w", cmd.Name(), err)
			}
		}
	}

	// Count documents with errors
	errorCount := 0
	for _, doc := range filtered {
		if doc.HasErrors() {
			errorCount++
		}
	}

	p.Logger.Info("pipeline completed",
		"name", p.Name,
		"documents", len(filtered),
		"errors", errorCount,
	)

	if errorCount > 0 {
		return fmt.Errorf("pipeline completed with %d documents having errors", errorCount)
	}

	return nil
}

// executeParallel runs a command on multiple documents in parallel
// using a worker pool pattern.
func (p *Pipeline) executeParallel(ctx context.Context, cmd Command, docs []*DocumentContext) error {
	maxParallel := p.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 5 // Default
	}

	return ParallelProcess(ctx, docs, cmd.Execute, maxParallel)
}

// ParallelProcess processes items in parallel using a worker pool.
// This is a generic helper that can be used by any command.
func ParallelProcess[T any](ctx context.Context, items []T, fn func(context.Context, T) error, maxWorkers int) error {
	if len(items) == 0 {
		return nil
	}

	// Create worker pool
	workers := maxWorkers
	if len(items) < workers {
		workers = len(items)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	ch := make(chan T, len(items))

	// Start workers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-ch:
					if !ok {
						return
					}
					if err := fn(ctx, item); err != nil {
						mu.Lock()
						errs = append(errs, err)
						mu.Unlock()
					}
				}
			}
		}()
	}

	// Send items to workers
	for _, item := range items {
		select {
		case <-ctx.Done():
			close(ch)
			wg.Wait()
			return ctx.Err()
		case ch <- item:
		}
	}
	close(ch)

	// Wait for completion
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("parallel processing had %d errors: %v", len(errs), errs[0])
	}

	return nil
}
