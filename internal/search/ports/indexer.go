package ports

import "context"

type Indexer interface {
	IndexDocument(ctx context.Context, doc interface{}) error
}
