// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"bufio"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCustomFactory(t *testing.T) {
	tests := []struct {
		name        string
		splitter    bufio.SplitFunc
		flushPeriod time.Duration
		wantErr     bool
	}{
		{
			name: "default configuration",
			splitter: func(data []byte, atEOF bool) (advance int, token []byte, err error) {
				return len(data), data, nil
			},
			flushPeriod: 100 * time.Millisecond,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewCustomFactory(tt.splitter, tt.flushPeriod)
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
