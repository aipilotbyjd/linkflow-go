package indexer

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/linkflow-go/pkg/logger"
)

type Indexer struct {
	client *elasticsearch.Client
	logger logger.Logger
	stopCh chan struct{}
}

func NewIndexer(client *elasticsearch.Client, logger logger.Logger) *Indexer {
	return &Indexer{
		client: client,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (i *Indexer) InitializeIndices() error {
	indices := []string{"workflows", "executions", "nodes", "users", "audit"}
	
	for _, index := range indices {
		mapping := getIndexMapping(index)
		req := esapi.IndicesCreateRequest{
			Index: index,
			Body:  strings.NewReader(mapping),
		}
		
		res, err := req.Do(context.Background(), i.client)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		
		if res.IsError() && res.StatusCode != 400 { // 400 means index already exists
			return nil
		}
	}
	
	return nil
}

func (i *Indexer) IndexDocument(ctx context.Context, doc interface{}) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	
	req := esapi.IndexRequest{
		Index: "documents",
		Body:  strings.NewReader(string(data)),
	}
	
	res, err := req.Do(ctx, i.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	
	return nil
}

func (i *Indexer) StartBackgroundIndexing(ctx context.Context) {
	// Background indexing logic
	i.logger.Info("Starting background indexer")
}

func (i *Indexer) Stop() {
	close(i.stopCh)
}

func getIndexMapping(index string) string {
	// Return appropriate mapping for each index type
	return `{
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"name": {"type": "text"},
				"description": {"type": "text"},
				"created_at": {"type": "date"},
				"updated_at": {"type": "date"}
			}
		}
	}`
}
