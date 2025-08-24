// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdktest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/sdktest"

import (
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var allow = []string{
	"go.opentelemetry.io/otel",
}

var Opts = cmp.Options{
	cmpopts.EquateApprox(0, 1e-9),
	cmp.Exporter(func(ty reflect.Type) bool {
		for _, prefix := range allow {
			if strings.HasPrefix(ty.PkgPath(), prefix) {
				return true
			}
		}
		return false
	}),
}

func Diff[T any](a, b T, opts ...cmp.Option) string {
	return cmp.Diff(a, b, Opts, cmp.Options(opts))
}
