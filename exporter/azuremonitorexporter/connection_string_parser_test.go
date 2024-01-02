// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
)

func TestParseConnectionString(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		want      *ConnectionVars
		wantError bool
	}{
		{
			name: "Valid connection string and instrumentation key",
			config: &Config{
				ConnectionString:   "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://ingestion.azuremonitor.com/",
				InstrumentationKey: "00000000-0000-0000-0000-00000000IKEY",
			},
			want: &ConnectionVars{
				InstrumentationKey: "00000000-0000-0000-0000-000000000000",
				IngestionURL:       "https://ingestion.azuremonitor.com/v2.1/track",
			},
			wantError: false,
		},
		{
			name: "Empty connection string with valid instrumentation key",
			config: &Config{
				InstrumentationKey: "00000000-0000-0000-0000-000000000000",
			},
			want: &ConnectionVars{
				InstrumentationKey: "00000000-0000-0000-0000-000000000000",
				IngestionURL:       DefaultIngestionEndpoint + "v2.1/track",
			},
			wantError: false,
		},
		{
			name: "Valid connection string with empty instrumentation key",
			config: &Config{
				ConnectionString: "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://ingestion.azuremonitor.com/",
			},
			want: &ConnectionVars{
				InstrumentationKey: "00000000-0000-0000-0000-000000000000",
				IngestionURL:       "https://ingestion.azuremonitor.com/v2.1/track",
			},
			wantError: false,
		},
		{
			name: "Empty connection string and instrumentation key",
			config: &Config{
				ConnectionString:   "",
				InstrumentationKey: "",
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "Invalid connection string format",
			config: &Config{
				ConnectionString: "InvalidConnectionString",
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "Missing InstrumentationKey in connection string",
			config: &Config{
				ConnectionString: "IngestionEndpoint=https://ingestion.azuremonitor.com/",
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "Empty InstrumentationKey in connection string",
			config: &Config{
				ConnectionString: "InstrumentationKey=;IngestionEndpoint=https://ingestion.azuremonitor.com/",
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "Extra parameters in connection string",
			config: &Config{
				ConnectionString: "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://ingestion.azuremonitor.com/;ExtraParam=extra",
			},
			want: &ConnectionVars{
				InstrumentationKey: "00000000-0000-0000-0000-000000000000",
				IngestionURL:       "https://ingestion.azuremonitor.com/v2.1/track",
			},
			wantError: false,
		},
		{
			name: "Spaces around equals in connection string",
			config: &Config{
				ConnectionString: "InstrumentationKey = 00000000-0000-0000-0000-000000000000 ; IngestionEndpoint = https://ingestion.azuremonitor.com/",
			},
			want: &ConnectionVars{
				InstrumentationKey: "00000000-0000-0000-0000-000000000000",
				IngestionURL:       "https://ingestion.azuremonitor.com/v2.1/track",
			},
			wantError: false,
		},
		{
			name: "Connection string too long",
			config: &Config{
				ConnectionString: configopaque.String(strings.Repeat("a", ConnectionStringMaxLength+1)),
			},
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConnectionString(tt.config)
			if tt.wantError {
				require.Error(t, err, "Expected an error but got none")
			} else {
				require.NoError(t, err, "Unexpected error: %v", err)
				require.NotNil(t, got, "Expected a non-nil result")
				assert.Equal(t, tt.want.InstrumentationKey, got.InstrumentationKey, "InstrumentationKey does not match")
				assert.Equal(t, tt.want.IngestionURL, got.IngestionURL, "IngestionEndpoint does not match")
			}
		})
	}
}
