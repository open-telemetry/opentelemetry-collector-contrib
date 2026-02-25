// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottltest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"

func Strp(s string) *string {
	return &s
}

func Floatp(f float64) *float64 {
	return &f
}

func Intp(i int64) *int64 {
	return &i
}

func Boolp(b bool) *bool {
	return &b
}
