// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestMultilineBuild(t *testing.T) {
	tests := []struct {
		name           string
		splitterConfig tokenize.SplitterConfig
		encoding       encoding.Encoding
		maxLogSize     int
		wantErr        bool
	}{
		{
			name:           "default configuration",
			splitterConfig: tokenize.NewSplitterConfig(),
			encoding:       unicode.UTF8,
			maxLogSize:     1024,
			wantErr:        false,
		},
		{
			name: "Multiline  error",
			splitterConfig: tokenize.SplitterConfig{
				Flusher: tokenize.NewFlusherConfig(),
				Multiline: tokenize.MultilineConfig{
					LineStartPattern: "START",
					LineEndPattern:   "END",
				},
			},
			encoding:   unicode.UTF8,
			maxLogSize: 1024,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewMultilineFactory(tt.splitterConfig, tt.encoding, tt.maxLogSize, trim.Nop)
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
