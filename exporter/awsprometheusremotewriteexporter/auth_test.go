// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsprometheusremotewriteexporter

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

func Test_RequestSignature(t *testing.T) {
	// Some form of AWS credentials must be set up for tests to succeed
	os.Setenv("AWS_ACCESS_KEY", "string")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "string2")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := v4.GetSignedRequestSignature(r)
		assert.NoError(t, err)
		w.WriteHeader(200)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	setting := confighttp.HTTPClientSettings{
		Endpoint:        serverURL.String(),
		TLSSetting:      configtls.TLSClientSetting{},
		ReadBufferSize:  0,
		WriteBufferSize: 0,
		Timeout:         0,
		CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
			settings := AuthSettings{Region: "region", Service: "service"}
			return newSigningRoundTripper(settings, next)
		},
	}
	client, _ := setting.ToClient()
	req, err := http.NewRequest("POST", setting.Endpoint, strings.NewReader("a=1&b=2"))
	assert.NoError(t, err)
	client.Do(req)

}
func Test_newSigningRoundTripper(t *testing.T) {

	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())

	// Some form of AWS credentials must be set up for tests to succeed
	os.Setenv("AWS_ACCESS_KEY", "string")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "string2")

	tests := []struct {
		name         string
		roundTripper http.RoundTripper
		settings     AuthSettings
		authApplied  bool
		returnError  bool
	}{
		{
			"success_case",
			defaultRoundTripper,
			AuthSettings{Region: "region", Service: "service"},
			true,
			false,
		},
		{
			"success_case_no_auth_applied",
			defaultRoundTripper,
			AuthSettings{Region: "", Service: ""},
			false,
			false,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rtp, err := newSigningRoundTripper(tt.settings, tt.roundTripper)
			if tt.returnError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.authApplied {
				sRtp := rtp.(*signingRoundTripper)
				assert.Equal(t, sRtp.transport, tt.roundTripper)
				assert.Equal(t, tt.settings.Service, sRtp.service)
			} else {
				assert.Equal(t, rtp, tt.roundTripper)
			}
		})
	}
}
