// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestJoinHostPortTemplateFunction(t *testing.T) {
	const prometheusEndpoint = "http://`joinHostPort(endpoint, \"prometheus.io/port\" in annotations ? " +
		"annotations[\"prometheus.io/port\"] : 9090)``\"prometheus.io/path\" in annotations ? " +
		"annotations[\"prometheus.io/path\"] : \"/metrics\"`"

	tests := []struct {
		name        string
		configValue string
		env         observer.EndpointEnv
		want        string
	}{
		{
			name:        "DNS hostname and integer",
			configValue: "`joinHostPort(endpoint, 8080)`",
			env:         observer.EndpointEnv{"endpoint": "metrics.default.svc.cluster.local"},
			want:        "metrics.default.svc.cluster.local:8080",
		},
		{
			name:        "IPv4 and string",
			configValue: "`joinHostPort(endpoint, port)`",
			env:         observer.EndpointEnv{"endpoint": "192.0.2.10", "port": "9090"},
			want:        "192.0.2.10:9090",
		},
		{
			name:        "IPv6 and observer port",
			configValue: "`joinHostPort(endpoint, kubelet_endpoint_port)`",
			env:         observer.EndpointEnv{"endpoint": "2001:db8::10", "kubelet_endpoint_port": uint16(10250)},
			want:        "[2001:db8::10]:10250",
		},
		{
			name:        "IPv6 zone",
			configValue: "`joinHostPort(host, port)`",
			env:         observer.EndpointEnv{"host": "fe80::1%eth0", "port": uint16(4317)},
			want:        "[fe80::1%eth0]:4317",
		},
		{
			name:        "environment data cannot shadow function",
			configValue: "`joinHostPort(endpoint, 8080)`",
			env: observer.EndpointEnv{
				"endpoint":     "localhost",
				"joinHostPort": func(...any) string { return "unexpected" },
			},
			want: "localhost:8080",
		},
		{
			name:        "Prometheus annotations",
			configValue: prometheusEndpoint,
			env: observer.EndpointEnv{
				"endpoint": "2001:db8::20",
				"annotations": map[string]string{
					"prometheus.io/port": "9464",
					"prometheus.io/path": "/custom-metrics",
				},
			},
			want: "http://[2001:db8::20]:9464/custom-metrics",
		},
		{
			name:        "Prometheus defaults",
			configValue: prometheusEndpoint,
			env: observer.EndpointEnv{
				"endpoint":    "2001:db8::30",
				"annotations": map[string]string{},
			},
			want: "http://[2001:db8::30]:9090/metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalBackticksInConfigValue(tt.configValue, tt.env)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestJoinHostPortDockerContainerEndpoint(t *testing.T) {
	endpoint := observer.Endpoint{
		Target: "2001:db8::40",
		Details: &observer.Container{
			Host:   "2001:db8::40",
			Labels: map[string]string{"metrics.port": "9090"},
		},
	}
	env, err := endpoint.Env()
	require.NoError(t, err)
	require.Equal(t, uint16(0), env["port"])

	got, err := evalBackticksInConfigValue("`joinHostPort(endpoint, labels[\"metrics.port\"])`", env)
	require.NoError(t, err)
	assert.Equal(t, "[2001:db8::40]:9090", got)
}

func TestJoinHostPortTemplateFunctionRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		env        observer.EndpointEnv
	}{
		{"empty host", "joinHostPort(host, 80)", observer.EndpointEnv{"host": ""}},
		{"non-string host", "joinHostPort(host, 80)", observer.EndpointEnv{"host": 1}},
		{"bracketed host", "joinHostPort(host, 80)", observer.EndpointEnv{"host": "[2001:db8::1]"}},
		{"port-qualified host", "joinHostPort(host, 80)", observer.EndpointEnv{"host": "localhost:8080"}},
		{"URL host", "joinHostPort(host, 80)", observer.EndpointEnv{"host": "http://localhost"}},
		{"invalid IPv6 zone", "joinHostPort(host, 80)", observer.EndpointEnv{"host": "fe80::1%eth0]"}},
		{"zero port", "joinHostPort(host, port)", observer.EndpointEnv{"host": "localhost", "port": 0}},
		{"out-of-range port", "joinHostPort(host, 65536)", observer.EndpointEnv{"host": "localhost"}},
		{"non-decimal port", "joinHostPort(host, port)", observer.EndpointEnv{"host": "localhost", "port": "8x"}},
		{"unsupported port type", "joinHostPort(host, port)", observer.EndpointEnv{"host": "localhost", "port": 80.0}},
		{"wrong argument count", "joinHostPort(host)", observer.EndpointEnv{"host": "localhost"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalConfigExpression(tt.expression, tt.env)
			require.Error(t, err)
		})
	}
}
