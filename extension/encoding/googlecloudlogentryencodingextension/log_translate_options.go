// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import "github.com/iancoleman/strcase"

type fieldTranslateOptions struct {
	keyMappers  []func(string) string
	preserveDst bool
}

type fieldTranslateFn func(*fieldTranslateOptions)

func preserveDst(opts *fieldTranslateOptions) {
	opts.preserveDst = true
}

func snakeifyKeys(opts *fieldTranslateOptions) {
	opts.keyMappers = append(opts.keyMappers, func(s string) string {
		return strcase.ToSnakeWithIgnore(s, ".")
	})
}

func prefixKeys(p string) fieldTranslateFn {
	return func(opts *fieldTranslateOptions) {
		opts.keyMappers = append(opts.keyMappers, func(s string) string {
			return p + s
		})
	}
}

func (opts fieldTranslateOptions) mapKey(s string) string {
	for _, mapper := range opts.keyMappers {
		s = mapper(s)
	}
	return s
}
