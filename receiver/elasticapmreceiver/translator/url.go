package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func parseURL(url *modelpb.URL, attrs pcommon.Map) {
	if url == nil {
		return
	}

	PutOptionalStr(attrs, "url.original", &url.Original)
	PutOptionalStr(attrs, "url.scheme", &url.Scheme)
	PutOptionalStr(attrs, conventions.AttributeHTTPURL, &url.Full)
	PutOptionalStr(attrs, "url.domain", &url.Domain)
	PutOptionalStr(attrs, "url.path", &url.Path)
	PutOptionalStr(attrs, "url.query", &url.Query)
	PutOptionalStr(attrs, "url.fragment", &url.Fragment)
	PutOptionalInt(attrs, "url.port", &url.Port)

	path := url.GetPath()
	query := url.GetQuery()
	if path != "" && query != "" {
		attrs.PutStr(conventions.AttributeHTTPTarget, path+query)
	}
}
