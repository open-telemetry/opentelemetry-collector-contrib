package elasticsearchexporter

import (
	"bytes"
	"context"
)

type Request struct {
	bulkIndexer *esBulkIndexerCurrent
	Items       []BulkIndexerItem
}

func newRequest(bulkIndexer *esBulkIndexerCurrent) *Request {
	return &Request{bulkIndexer: bulkIndexer}
}

func (r *Request) Export(ctx context.Context) error {
	for _, item := range r.Items {
		if err := r.bulkIndexer.Add(ctx, item.Index, bytes.NewReader(item.Body)); err != nil {
			return err // FIXME: merge errors
		}
	}
	_, err := r.bulkIndexer.Flush(ctx)
	return err
}

func (r *Request) ItemsCount() int {
	return len(r.Items)
}

func (r *Request) Add(index string, body []byte) {
	r.Items = append(r.Items, BulkIndexerItem{Index: index, Body: body})
}

type BulkIndexerItem struct {
	Index string
	Body  []byte
}
