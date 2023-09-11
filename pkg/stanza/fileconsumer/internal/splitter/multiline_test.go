// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestSplitFuncFactory(t *testing.T) {
	tests := []struct {
		name        string
		splitFunc   split.Config
		encoding    encoding.Encoding
		maxLogSize  int
		flushPeriod time.Duration
		wantErr     bool
	}{
		{
			name:        "default configuration",
			encoding:    unicode.UTF8,
			maxLogSize:  1024,
			flushPeriod: 100 * time.Millisecond,
			wantErr:     false,
		},
		{
			name: "Multiline  error",
			splitFunc: split.Config{
				LineStartPattern: "START",
				LineEndPattern:   "END",
			},
			flushPeriod: 100 * time.Millisecond,
			encoding:    unicode.UTF8,
			maxLogSize:  1024,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewSplitFuncFactory(tt.splitFunc, tt.encoding, tt.maxLogSize, trim.Nop, tt.flushPeriod)
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
