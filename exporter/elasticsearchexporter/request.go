package elasticsearchexporter

import (
	"bytes"
	"context"
)

type request struct {
	bulkIndexer *esBulkIndexerCurrent
	Items       []bulkIndexerItem
}

func newRequest(bulkIndexer *esBulkIndexerCurrent) *request {
	return &request{bulkIndexer: bulkIndexer}
}

func (r *request) Export(ctx context.Context) error {
	batch := make([]esBulkIndexerItem, len(r.Items))
	for i, item := range r.Items {
		batch[i] = esBulkIndexerItem{
			Index: item.Index,
			Body:  bytes.NewReader(item.Body),
		}
	}
	return r.bulkIndexer.AddBatchAndFlush(ctx, batch)
}

func (r *request) ItemsCount() int {
	return len(r.Items)
}

func (r *request) add(item bulkIndexerItem) {
	r.Items = append(r.Items, item)
}

type bulkIndexerItem struct {
	Index string
	Body  []byte
}
