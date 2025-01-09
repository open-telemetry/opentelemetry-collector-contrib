// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestatsreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func Test_Config_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr error
	}{
		{
			name: "valid",
			cfg: &Config{
				Include:          "/var/log/*.log",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			wantErr: nil,
		},
		{
			name: "missing include pattern",
			cfg: &Config{
				Include:          "",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			wantErr: errors.New("include must not be empty"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
