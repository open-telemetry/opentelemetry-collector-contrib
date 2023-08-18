// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func TestMultilineBuild(t *testing.T) {
	type args struct {
		maxLogSize int
	}
	tests := []struct {
		name           string
		splitterConfig helper.SplitterConfig
		args           args
		wantErr        bool
	}{
		{
			name:           "default configuration",
			splitterConfig: helper.NewSplitterConfig(),
			args: args{
				maxLogSize: 1024,
			},
			wantErr: false,
		},
		{
			name: "eoncoding error",
			splitterConfig: helper.SplitterConfig{
				Encoding:  "error",
				Flusher:   helper.NewFlusherConfig(),
				Multiline: helper.NewMultilineConfig(),
			},
			args: args{
				maxLogSize: 1024,
			},
			wantErr: true,
		},
		{
			name: "Multiline  error",
			splitterConfig: helper.SplitterConfig{
				Encoding: "utf-8",
				Flusher:  helper.NewFlusherConfig(),
				Multiline: helper.MultilineConfig{
					LineStartPattern: "START",
					LineEndPattern:   "END",
				},
			},
			args: args{
				maxLogSize: 1024,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewMultilineFactory(tt.splitterConfig)
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
