package ports

import "context"

// IndexerLifecycle represents the operational lifecycle for an indexer implementation.
// This is used by the search server for wiring/startup concerns, while the app layer can
// depend only on the smaller Indexer interface.
type IndexerLifecycle interface {
	Indexer

	InitializeIndices() error
	StartBackgroundIndexing(ctx context.Context)
	Stop()
}
