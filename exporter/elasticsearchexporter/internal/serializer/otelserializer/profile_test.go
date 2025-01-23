// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestSerializeProfile(t *testing.T) {
	tests := []struct {
		name              string
		profileCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record pprofile.Profile)
		wantErr           bool
		expected          any
	}{
		{
			name: "with a simple sample",
			profileCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, profile pprofile.Profile) {
				sample := profile.Sample().AppendEmpty()
				sample.TimestampsUnixNano().Append(0)
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp": "0.0",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles := pprofile.NewProfiles()
			resource := profiles.ResourceProfiles().AppendEmpty()
			scope := resource.ScopeProfiles().AppendEmpty()
			profile := scope.Profiles().AppendEmpty()
			tt.profileCustomizer(resource.Resource(), scope.Scope(), profile)
			profiles.MarkReadOnly()

			var buf bytes.Buffer
			err := SerializeProfile(resource.Resource(), "", scope.Scope(), "", profile, elasticsearch.Index{}, &buf)
			if !tt.wantErr {
				require.NoError(t, err)
			}

			fmt.Fprintf(os.Stdout, "== %s\n", buf.String())

			b := buf.Bytes()
			eventAsJSON := string(b)
			var result any
			decoder := json.NewDecoder(bytes.NewBuffer(b))
			decoder.UseNumber()
			err = decoder.Decode(&result)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result, eventAsJSON)
		})
	}
}
