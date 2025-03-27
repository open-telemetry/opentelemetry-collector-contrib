package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseHTTP(http *modelpb.HTTP, attrs pcommon.Map) {
	if http == nil {
		return
	}

	parseHTTPRequest(http.Request, attrs)
	parseHTTPResponse(http.Response, attrs)
	PutOptionalStr(attrs, "http.version", &http.Version)
}

func parseHTTPRequest(request *modelpb.HTTPRequest, attrs pcommon.Map) {
	if request == nil {
		return
	}

	// TODO: request.Body
	// TODO: request.Headers
	// TODO: request.Env
	// TODO: request.Cookies
	PutOptionalStr(attrs, "http.request.id", &request.Id)
	PutOptionalStr(attrs, conventions.AttributeHTTPMethod, &request.Method)
	PutOptionalStr(attrs, "http.request.referrer", &request.Referrer)
}

func parseHTTPResponse(response *modelpb.HTTPResponse, attrs pcommon.Map) {
	if response == nil {
		return
	}

	// TODO: response.Headers
	PutOptionalBool(attrs, "http.response.finished", response.Finished)
	PutOptionalBool(attrs, "http.response.headers_sent", response.HeadersSent)
	PutOptionalInt(attrs, "http.response.transfer_size", response.TransferSize)
	PutOptionalInt(attrs, "http.response.encoded_body_size", response.EncodedBodySize)
	PutOptionalInt(attrs, "http.response.decoded_body_size", response.DecodedBodySize)
	PutOptionalInt(attrs, "http.response.status_code", &response.StatusCode)
}
