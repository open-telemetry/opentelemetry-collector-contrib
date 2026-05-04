// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestHTTPServerSpan_NonErrorStatusCode_MarkedSuccess covers the case where a server span
// returns a 4xx that the user has listed in non_error_http_status_codes. The resulting
// RequestData.Success should be true even though 404 is outside the default 100-399 range.
func TestHTTPServerSpan_NonErrorStatusCode_MarkedSuccess(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	span.Attributes().PutInt("http.response.status_code", 404)

	policy := newHTTPStatusPolicy(&Config{NonErrorHTTPStatusCodes: []int{404}})
	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, policy, zap.NewNop())

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	assert.Equal(t, "404", data.ResponseCode)
	assert.True(t, data.Success)
}

// TestHTTPClientSpan_NonErrorStatusCode_MarkedSuccess mirrors the server-span test but
// for a client span emitted as RemoteDependencyData. The non-error list applies to both.
func TestHTTPClientSpan_NonErrorStatusCode_MarkedSuccess(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	span.Attributes().PutInt("http.response.status_code", 404)

	policy := newHTTPStatusPolicy(&Config{NonErrorHTTPStatusCodes: []int{404}})
	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, policy, zap.NewNop())

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	assert.Equal(t, "404", data.ResultCode)
	assert.True(t, data.Success)
}

// TestHTTPServerSpan_OTelSpecAlignment_4xxIsSuccess verifies that with
// align_http_server_span_success_with_otel_spec enabled, a 4xx server response is treated
// as Success=true even when not in the non-error list.
func TestHTTPServerSpan_OTelSpecAlignment_4xxIsSuccess(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	span.Attributes().PutInt("http.response.status_code", 404)

	policy := newHTTPStatusPolicy(&Config{AlignHTTPServerSpanSuccessWithOTelSpec: true})
	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, policy, zap.NewNop())

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	assert.Equal(t, "404", data.ResponseCode)
	assert.True(t, data.Success)
}

// TestHTTPClientSpan_OTelSpecAlignment_4xxStillError verifies the alignment flag is
// scoped to server spans only. Client spans keep the historical 4xx-as-error behavior,
// matching OTel HTTP semantic conventions for client status (4xx is Error).
func TestHTTPClientSpan_OTelSpecAlignment_4xxStillError(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	span.Attributes().PutInt("http.response.status_code", 404)

	policy := newHTTPStatusPolicy(&Config{AlignHTTPServerSpanSuccessWithOTelSpec: true})
	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, policy, zap.NewNop())

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	assert.Equal(t, "404", data.ResultCode)
	assert.False(t, data.Success)
}

// TestHTTPServerSpan_OTelSpecAlignment_5xxStillError verifies the alignment flag does
// not flip 5xx server responses to success; only 4xx is affected.
func TestHTTPServerSpan_OTelSpecAlignment_5xxStillError(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	span.Attributes().PutInt("http.response.status_code", 503)

	policy := newHTTPStatusPolicy(&Config{AlignHTTPServerSpanSuccessWithOTelSpec: true})
	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, policy, zap.NewNop())

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	assert.Equal(t, "503", data.ResponseCode)
	assert.False(t, data.Success)
}

// TestHTTPServerSpan_DefaultPolicy_4xxStillError pins the default behavior: with no
// configuration, a 4xx server response is still reported as Success=false. Guards
// against accidental change of the default during refactors.
func TestHTTPServerSpan_DefaultPolicy_4xxStillError(t *testing.T) {
	span := getDefaultHTTPServerSpan()
	span.Attributes().PutInt("http.response.status_code", 404)

	policy := newHTTPStatusPolicy(&Config{})
	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, policy, zap.NewNop())

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RequestData)
	assert.Equal(t, "404", data.ResponseCode)
	assert.False(t, data.Success)
}

// TestHTTPStatusPolicy_NonErrorOverridesAlignment verifies that a code in the non-error
// list short-circuits to Success=true even on a client span where the alignment flag
// would not apply. This is the only path that lets users mark client codes as success.
func TestHTTPStatusPolicy_NonErrorOverridesAlignment(t *testing.T) {
	span := getDefaultHTTPClientSpan()
	span.Attributes().PutInt("http.response.status_code", 409)

	policy := newHTTPStatusPolicy(&Config{
		NonErrorHTTPStatusCodes:                []int{409},
		AlignHTTPServerSpanSuccessWithOTelSpec: true,
	})
	envelopes, _ := spanToEnvelopes(defaultResource, defaultInstrumentationLibrary, span, true, policy, zap.NewNop())

	data := envelopes[0].Data.(*contracts.Data).BaseData.(*contracts.RemoteDependencyData)
	assert.Equal(t, "409", data.ResultCode)
	assert.True(t, data.Success)
}
