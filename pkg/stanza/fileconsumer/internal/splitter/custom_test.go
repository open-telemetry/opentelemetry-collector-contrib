// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"
)

func TestCustomFactory(t *testing.T) {
	type fields struct {
		Flusher  tokenize.FlusherConfig
		Splitter bufio.SplitFunc
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "default configuration",
			fields: fields{
				Flusher: tokenize.NewFlusherConfig(),
				Splitter: func(data []byte, atEOF bool) (advance int, token []byte, err error) {
					return len(data), data, nil
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewCustomFactory(tt.fields.Flusher, tt.fields.Splitter)
			got, err := factory.Build()
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}
