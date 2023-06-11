// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package infoscraper

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	tests := []struct {
		name string
		f    *Factory
		want internal.Config
	}{
		{
			f:    &Factory{},
			want: &Config{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.f.CreateDefaultConfig()
			assert.IsType(t, got, tt.want)
		})
	}
}

func TestFactory_CreateMetricsScraper(t *testing.T) {
	type args struct {
		settings receiver.CreateSettings
		config   internal.Config
	}
	tests := []struct {
		name    string
		f       *Factory
		args    args
		wantErr bool
	}{
		{
			f: &Factory{},
			args: args{
				settings: receiver.CreateSettings{},
				config:   &Config{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.CreateMetricsScraper(context.Background(), tt.args.settings, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Factory.CreateMetricsScraper() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NotNil(t, got)
		})
	}
}
