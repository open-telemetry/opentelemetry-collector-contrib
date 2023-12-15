// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func Test_ruleEval(t *testing.T) {
	type args struct {
		ruleStr  string
		endpoint observer.Endpoint
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		// Doesn't work yet. See comment in newRule.
		// {"unknown variable", args{`type == "port" && unknown_var == 1`, portEndpoint}, false, true},
		{"basic port", args{`type == "port" && name == "http" && pod.labels["app"] == "redis"`, portEndpoint}, true, false},
		{"basic hostport", args{`type == "hostport" && port == 1234 && process_name == "splunk"`, hostportEndpoint}, true, false},
		{"basic pod", args{`type == "pod" && labels["region"] == "west-1"`, podEndpoint}, true, false},
		{"basic service", args{`type == "k8s.service" && labels["region"] == "west-1"`, serviceEndpoint}, true, false},
		{"annotations", args{`type == "pod" && annotations["scrape"] == "true"`, podEndpoint}, true, false},
		{"basic container", args{`type == "container" && labels["region"] == "east-1"`, containerEndpoint}, true, false},
		{"basic k8s.node", args{`type == "k8s.node" && kubelet_endpoint_port == 10250`, k8sNodeEndpoint}, true, false},
		{"relocated type builtin", args{`type == "k8s.node" && typeOf("some string") == "string"`, k8sNodeEndpoint}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newRule(tt.args.ruleStr)
			require.NoError(t, err)
			require.NotNil(t, got)

			env, err := tt.args.endpoint.Env()
			require.NoError(t, err)

			match, err := got.eval(env)

			if (err != nil) != tt.wantErr {
				t.Errorf("eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, match, "expected eval to return %v but returned %v", tt.want, match)
		})
	}
}

func Test_newRule(t *testing.T) {
	type args struct {
		ruleStr string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"empty rule", args{""}, true},
		{"does not startMetrics with type", args{"port == 1234"}, true},
		{"invalid syntax", args{"port =="}, true},
		{"valid port", args{`type == "port" && port_name == "http"`}, false},
		{"valid pod", args{`type=="pod" && port_name == "http"`}, false},
		{"valid hostport", args{`type == "hostport" && port_name == "http"`}, false},
		{"valid container", args{`type == "container" && port == 8080`}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newRule(tt.args.ruleStr)
			if err == nil {
				assert.NotNil(t, got, "expected rule to be created when there was no error")
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("newRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
