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
		Endpoint:           serverURL.String(),
		TLSSetting:         configtls.TLSClientSetting{},
		ReadBufferSize:     0,
		WriteBufferSize:    0,
		Timeout:            0,
		CustomRoundTripper: signingRoundTripper,
	}
	client, _ := setting.ToClient()
	req, err := http.NewRequest("POST", setting.Endpoint, strings.NewReader("a=1&b=2"))
	assert.NoError(t, err)
	client.Do(req)

}
func Test_signingRoundTripper(t *testing.T) {

	defaultRoundTripper := (http.RoundTripper)(http.DefaultTransport.(*http.Transport).Clone())

	// Some form of AWS credentials must be set up for tests to succeed
	os.Setenv("AWS_ACCESS_KEY", "string")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "string2")

	tests := []struct {
		name         string
		roundTripper http.RoundTripper
		returnError  bool
	}{
		{
			"success_case",
			defaultRoundTripper,
			false,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := signingRoundTripper(tt.roundTripper)
			if tt.returnError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
