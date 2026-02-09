// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

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
		{"does not start with type", args{"port == 1234"}, true},
		{"invalid syntax", args{"port =="}, true},
		{"valid port", args{`type == "port" && name == "http"`}, false},
		{"valid pod", args{`type == "pod" && labels["region"] == "west-1"`}, false},
		{"valid pod no spaces", args{`type=="pod" && labels["region"] == "west-1"`}, false},
		{"valid hostport", args{`type == "hostport" && port == 1234`}, false},
		{"valid container", args{`type == "container" && port == 8080`}, false},
		{"valid k8s.service", args{`type == "k8s.service" && labels["app"] == "redis"`}, false},
		{"valid k8s.ingress", args{`type == "k8s.ingress" && host == "example.com"`}, false},
		{"valid k8s.node", args{`type == "k8s.node" && kubelet_endpoint_port == 10250`}, false},
		{"valid k8s.crd", args{`type == "k8s.crd" && kind == "Exporter"`}, false},
		{"valid jsonfile", args{`type == "jsonfile" && labels["service"] == "test"`}, false},
		{"valid kafka.topics", args{`type == "kafka.topics"`}, false},
		{"valid pod.container", args{`type == "pod.container" && container_image == "redis"`}, false},
		{"invalid endpoint type", args{`type == "invalid" && port == 1234`}, true},
		{"missing type check", args{`name == "http"`}, true},
		{"type check not at start", args{`name == "http" && type == "port"`}, true},
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
		// Port endpoint tests
		{
			"basic port match",
			args{`type == "port" && name == "http"`, portEndpoint},
			true,
			false,
		},
		{
			"port with pod labels",
			args{`type == "port" && pod.labels["app"] == "redis"`, portEndpoint},
			true,
			false,
		},
		{
			"port non-match",
			args{`type == "port" && name == "https"`, portEndpoint},
			false,
			false,
		},
		{
			"port with port number",
			args{`type == "port" && port == 1234`, portEndpoint},
			true,
			false,
		},
		// Pod endpoint tests
		{
			"basic pod match",
			args{`type == "pod" && labels["region"] == "west-1"`, podEndpoint},
			true,
			false,
		},
		{
			"pod with annotations",
			args{`type == "pod" && annotations["scrape"] == "true"`, podEndpoint},
			true,
			false,
		},
		{
			"pod non-match",
			args{`type == "pod" && labels["region"] == "east-1"`, podEndpoint},
			false,
			false,
		},
		// Service endpoint tests
		{
			"basic service match",
			args{`type == "k8s.service" && labels["region"] == "west-1"`, serviceEndpoint},
			true,
			false,
		},
		{
			"service non-match",
			args{`type == "k8s.service" && labels["app"] == "mysql"`, serviceEndpoint},
			false,
			false,
		},
		// HostPort endpoint tests
		{
			"basic hostport match",
			args{`type == "hostport" && port == 1234 && process_name == "splunk"`, hostportEndpoint},
			true,
			false,
		},
		{
			"hostport non-match",
			args{`type == "hostport" && port == 5678`, hostportEndpoint},
			false,
			false,
		},
		// Container endpoint tests
		{
			"basic container match",
			args{`type == "container" && labels["region"] == "east-1"`, containerEndpoint},
			true,
			false,
		},
		{
			"container with port",
			args{`type == "container" && port == 8080`, containerEndpoint},
			true,
			false,
		},
		{
			"container non-match",
			args{`type == "container" && labels["region"] == "west-1"`, containerEndpoint},
			false,
			false,
		},
		// K8s Node endpoint tests
		{
			"basic k8s.node match",
			args{`type == "k8s.node" && kubelet_endpoint_port == 10250`, k8sNodeEndpoint},
			true,
			false,
		},
		{
			"k8s.node with labels",
			args{`type == "k8s.node" && labels["kubernetes.io/arch"] == "amd64"`, k8sNodeEndpoint},
			true,
			false,
		},
		{
			"k8s.node non-match",
			args{`type == "k8s.node" && kubelet_endpoint_port == 10251`, k8sNodeEndpoint},
			false,
			false,
		},
		// PodContainer endpoint tests
		{
			"pod.container match",
			args{`type == "pod.container" && container_image == "redis"`, podContainerEndpoint},
			true,
			false,
		},
		{
			"pod.container with pod labels",
			args{`type == "pod.container" && pod.labels["app"] == "redis"`, podContainerEndpoint},
			true,
			false,
		},
		// Kafka Topics endpoint tests
		{
			"kafka.topics match",
			args{`type == "kafka.topics"`, kafkaTopicsEndpoint},
			true,
			false,
		},
		// CRD endpoint tests
		{
			"k8s.crd match",
			args{`type == "k8s.crd" && kind == "Exporter"`, crdEndpoint},
			true,
			false,
		},
		{
			"k8s.crd with spec",
			args{`type == "k8s.crd" && spec["exporterType"] == "prometheusremotewrite"`, crdEndpoint},
			true,
			false,
		},
		{
			"k8s.crd with labels",
			args{`type == "k8s.crd" && labels["app"] == "myapp"`, crdEndpoint},
			true,
			false,
		},
		// JSON File endpoint tests
		{
			"jsonfile match",
			args{`type == "jsonfile" && labels["service"] == "test-service"`, jsonFileEndpoint},
			true,
			false,
		},
		{
			"jsonfile with name",
			args{`type == "jsonfile" && name == "Test Service"`, jsonFileEndpoint},
			true,
			false,
		},
		{
			"jsonfile non-match",
			args{`type == "jsonfile" && labels["service"] == "other-service"`, jsonFileEndpoint},
			false,
			false,
		},
		// TypeOf function tests
		{
			"relocated type builtin",
			args{`type == "k8s.node" && typeOf("some string") == "string"`, k8sNodeEndpoint},
			true,
			false,
		},
		{
			"typeOf with number",
			args{`type == "port" && typeOf(1234) == "int"`, portEndpoint},
			true,
			false,
		},
		// Complex expressions
		{
			"complex AND expression",
			args{`type == "port" && name == "http" && port == 1234 && pod.labels["app"] == "redis"`, portEndpoint},
			true,
			false,
		},
		{
			"complex OR expression",
			args{`type == "pod" && (labels["region"] == "west-1" || labels["region"] == "east-1")`, podEndpoint},
			true,
			false,
		},
		// Error cases
		{
			"endpoint without details",
			args{`type == "port"`, unsupportedEndpoint},
			false,
			true,
		},
		{
			"wrong type",
			args{`type == "pod"`, portEndpoint},
			false,
			false,
		},
		{
			"invalid expression",
			args{`type == "port" && invalid_field["key"]`, portEndpoint},
			false,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newRule(tt.args.ruleStr)
			require.NoError(t, err)
			require.NotNil(t, got)

			env, err := tt.args.endpoint.Env()
			if err != nil {
				if !tt.wantErr {
					t.Errorf("endpoint.Env() error = %v, expected no error", err)
				}
				return
			}

			match, err := got.eval(env)

			if (err != nil) != tt.wantErr {
				t.Errorf("eval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, match, "expected eval to return %v but returned %v", tt.want, match)
		})
	}
}

func Test_ruleEval_TypeMismatch(t *testing.T) {
	// Test that rules correctly filter by type
	tests := []struct {
		name     string
		ruleStr  string
		endpoint observer.Endpoint
		want     bool
	}{
		{"pod rule with port endpoint", `type == "pod"`, portEndpoint, false},
		{"port rule with pod endpoint", `type == "port"`, podEndpoint, false},
		{"service rule with pod endpoint", `type == "k8s.service"`, podEndpoint, false},
		{"correct type match", `type == "port"`, portEndpoint, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := newRule(tt.ruleStr)
			require.NoError(t, err)

			env, err := tt.endpoint.Env()
			require.NoError(t, err)

			match, err := r.eval(env)
			require.NoError(t, err)
			assert.Equal(t, tt.want, match)
		})
	}
}
