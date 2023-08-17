// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func TestCustomFactory(t *testing.T) {
	type fields struct {
		Flusher  helper.FlusherConfig
		Splitter bufio.SplitFunc
	}
	type args struct {
		maxLogSize int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "default configuration",
			fields: fields{
				Flusher: helper.NewFlusherConfig(),
				Splitter: func(data []byte, atEOF bool) (advance int, token []byte, err error) {
					return len(data), data, nil
				},
			},
			args: args{
				maxLogSize: 1024,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewCustomFactory(tt.fields.Flusher, tt.fields.Splitter)
			got, err := factory.Build(tt.args.maxLogSize)
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
