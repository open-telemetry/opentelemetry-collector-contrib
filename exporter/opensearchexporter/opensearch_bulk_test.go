// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_pushDocuments(t *testing.T) {

	type args struct {
		index       string
		document    []byte
		maxAttempts int
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "simple",
			args: args{
				index:       "sample-index",
				document:    []byte(`{"field": "sample"}`),
				maxAttempts: 0,
			},
			wantErr: assert.NoError,
		},
	}

	ctx := context.Background()
	logger := zap.L()
	config := withDefaultConfig()
	client, _ := newOpenSearchClient(zap.L(), config)
	bulkIndexer, _ := newBulkIndexer(zap.L(), client, config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := tt.args.index
			document := tt.args.document
			attempts := tt.args.maxAttempts
			tt.wantErr(t, pushDocuments(ctx, logger, index, document, bulkIndexer, attempts),
				fmt.Sprintf("pushDocuments(%v, %v, %v, %v, %v, %v)",
					ctx, logger, index, document, bulkIndexer, attempts))
		})
	}
}

func Test_shouldRetryEvent(t *testing.T) {
	type args struct {
		status int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "internal server error",
			args: args{status: 500},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, shouldRetryEvent(tt.args.status), "shouldRetryEvent(%v)", tt.args.status)
		})
	}
}
