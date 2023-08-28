// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"bufio"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestCustomFactory(t *testing.T) {
	tests := []struct {
		name        string
		splitFunc   bufio.SplitFunc
		flushPeriod time.Duration
		wantErr     bool
	}{
		{
			name: "default configuration",
			splitFunc: func(data []byte, atEOF bool) (advance int, token []byte, err error) {
				return len(data), data, nil
			},
			flushPeriod: 500 * time.Millisecond,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewCustomSplitFuncFactory(tt.splitFunc, trim.Whitespace, tt.flushPeriod)
			got, err := factory.SplitFunc()
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
