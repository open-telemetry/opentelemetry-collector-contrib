// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestGetFirstMatchingValue(t *testing.T) {
	attr1 := pcommon.NewMap()
	attr1.PutStr("key1", "value1")
	attr1.PutStr("key2", "value2")
	attrs := []pcommon.Map{attr1}

	tests := []struct {
		name      string
		keys      []string
		want      string
		wantFound bool
	}{
		{
			name:      "Found in first attribute",
			keys:      []string{"key1"},
			want:      "value1",
			wantFound: true,
		},
		{
			name:      "Found in second attribute",
			keys:      []string{"key2"},
			want:      "value2",
			wantFound: true,
		},
		{
			name:      "Not found",
			keys:      []string{"key3"},
			want:      "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotFound := getFirstMatchingValue(tt.keys, attrs...)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantFound, gotFound)
		})
	}
}
