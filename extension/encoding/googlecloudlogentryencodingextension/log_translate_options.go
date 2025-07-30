// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

type fieldTranslateOptions struct {
	keyMappers  []func(string) string
	preserveDst bool
}

type fieldTranslateFn func(*fieldTranslateOptions)

func (opts fieldTranslateOptions) mapKey(s string) string {
	for _, mapper := range opts.keyMappers {
		s = mapper(s)
	}
	return s
}
