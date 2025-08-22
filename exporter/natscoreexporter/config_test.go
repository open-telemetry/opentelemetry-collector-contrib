// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter

import (
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
			name: "auth using incomplete username/password",
			cfg: Config{
				Auth: AuthConfig{
					Username: "username",
				},
			},
			err: fmt.Errorf("username/password auth requires username and password"),
		},
		{
			name: "auth using incomplete nkey",
			cfg: Config{
				Auth: AuthConfig{
					NKey: "nkey",
				},
			},
			err: fmt.Errorf("NKey auth requires seed and exactly one of public key or JWT"),
		},
		{
			name: "auth using nkey and jwt",
			cfg: Config{
				Auth: AuthConfig{
					NKey: "nkey",
					JWT:  "jwt",
					Seed: "seed",
				},
			},
			err: fmt.Errorf("NKey auth requires seed and exactly one of public key or JWT"),
		},
		{
			name: "signal using marshaler and encoder",
			cfg: Config{
				Traces: TracesConfig{
					Marshaler: "otlp_proto",
					Encoder:   "proto",
				},
			},
			err: fmt.Errorf("marshaler and encoder are mutually exclusive"),
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
			name: "invalid marshaler",
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
