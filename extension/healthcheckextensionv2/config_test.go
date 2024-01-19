// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextensionv2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/http"
)

func TestConfig(t *testing.T) {
	grpcSettings := &grpc.Settings{
		GRPCServerSettings: configgrpc.GRPCServerSettings{
			NetAddr: confignet.NetAddr{
				Endpoint:  "127.0.0.1:5000",
				Transport: "tcp",
			},
		},
	}
	httpSettings := &http.Settings{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "127.0.0.1:5001",
		},
	}

	for _, tc := range []struct {
		name   string
		config *Config
		valid  bool
	}{
		{
			name: "Valid GRPC Settings Only",
			config: &Config{
				GRPCSettings: grpcSettings,
			},
			valid: true,
		},
		{
			name: "Invalid GRPC Settings Only",
			config: &Config{
				GRPCSettings: &grpc.Settings{},
			},
			valid: false,
		},
		{
			name: "Valid HTTP Settings Only",
			config: &Config{
				HTTPSettings: httpSettings,
			},
			valid: true,
		},
		{
			name: "Invalid HTTP Settings Only",
			config: &Config{
				HTTPSettings: &http.Settings{},
			},
			valid: false,
		},
		{
			name: "GRPC and HTTP Settings",
			config: &Config{
				GRPCSettings: grpcSettings,
				HTTPSettings: httpSettings,
			},
			valid: true,
		},
		{
			name: "GRPC and HTTP Settings both invalid",
			config: &Config{
				GRPCSettings: &grpc.Settings{},
				HTTPSettings: &http.Settings{},
			},
			valid: false,
		},
		{
			name:   "Neither GRPC nor HTTP Settings",
			config: &Config{},
			valid:  false,
		},
	} {
		err := tc.config.Validate()
		assert.Equal(t, tc.valid, err == nil)
	}
}
