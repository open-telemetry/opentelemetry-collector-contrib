// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogfleetautomationextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogfleetautomationextension"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
				API: APIConfig{
					Site: defaultSite,
					Key:  "1234567890abcdef1234567890abcdef",
				},
			},
			wantErr: nil,
		},
		{
			name: "Empty site",
			config: Config{
				API: APIConfig{
					Site: "",
					Key:  "1234567890abcdef1234567890abcdef",
				},
			},
			wantErr: errEmptyEndpoint,
		},
		{
			name: "Unset API key",
			config: Config{
				API: APIConfig{
					Site: defaultSite,
					Key:  "",
				},
			},
			wantErr: errUnsetAPIKey,
		},
		{
			name: "Invalid API key characters",
			config: Config{
				API: APIConfig{
					Site: defaultSite,
					Key:  "1234567890abcdef1234567890abcdeg",
				},
			},
			wantErr: fmt.Errorf("%w: invalid characters: %s", errAPIKeyFormat, "g"),
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
