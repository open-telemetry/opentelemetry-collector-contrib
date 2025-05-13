// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "Valid configuration",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
			},
			wantErr: nil,
		},
		{
			name: "Empty site",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: "",
					Key:  "1234567890abcdef1234567890abcdef",
				},
			},
			wantErr: datadogconfig.ErrEmptyEndpoint,
		},
		{
			name: "Unset API key",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "",
				},
			},
			wantErr: datadogconfig.ErrUnsetAPIKey,
		},
		{
			name: "Invalid API key characters",
			config: Config{
				API: datadogconfig.APIConfig{
					Site: datadogconfig.DefaultSite,
					Key:  "1234567890abcdef1234567890abcdeg",
				},
			},
			wantErr: fmt.Errorf("%w: invalid characters: %s", datadogconfig.ErrAPIKeyFormat, "g"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
