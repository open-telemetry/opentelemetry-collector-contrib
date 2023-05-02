// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		{"annotations", args{`type == "pod" && annotations["scrape"] == "true"`, podEndpoint}, true, false},
		{"basic container", args{`type == "container" && labels["region"] == "east-1"`, containerEndpoint}, true, false},
		{"basic k8s.node", args{`type == "k8s.node" && kubelet_endpoint_port == 10250`, k8sNodeEndpoint}, true, false},
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
		{"does not start with type", args{"port == 1234"}, true},
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
