// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		err  error
	}{
		{name: "empty config", cfg: Config{}, err: nil},
		{
			name: "signal using invalid subject",
			cfg: Config{
				Traces: TracesConfig{
					Subject: "invalid{{",
				},
			},
			err: fmt.Errorf("error parsing subject: template: subject:1: unclosed action"),
		},
		{
			name: "auth using invalid configuration",
			cfg: Config{
				Auth: AuthConfig{
					Token: "token",
					Seed:  "seed",
				},
			},
			err: errors.New("invalid auth configuration"),
		},
		{
			name: "signal using marshaler and encoder simultaneously",
			cfg: Config{
				Traces: TracesConfig{
					Marshaler: "otlp_proto",
					Encoder:   "proto",
				},
			},
			err: errors.New("marshaler and encoder cannot be configured simultaneously"),
		},
		{
			name: "non-log signal using logs_body marshaler",
			cfg: Config{
				Traces: TracesConfig{
					Marshaler: "log_body",
				},
			},
			err: fmt.Errorf("unsupported marshaler: %s", "log_body"),
		},
		{
			name: "marshaler using unsupported type",
			cfg: Config{
				Traces: TracesConfig{
					Marshaler: "invalid",
				},
			},
			err: fmt.Errorf("unsupported marshaler: %s", "invalid"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.err == nil {
				assert.NoError(t, err, tt.name)
			} else {
				assert.Equal(t, tt.err, err, tt.name)
			}
		})
	}
}
