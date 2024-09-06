// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compare // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest/compare"

import (
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var Opts = []cmp.Option{
	cmpopts.EquateApprox(0, 1e-9),
	cmp.Exporter(func(ty reflect.Type) bool {
		return strings.HasPrefix(ty.PkgPath(), "go.opentelemetry.io/collector/pdata") || strings.HasPrefix(ty.PkgPath(), "github.com/open-telemetry/opentelemetry-collector-contrib")
	}),
}

func Equal[T any](a, b T, opts ...cmp.Option) bool {
	return cmp.Equal(a, b, append(Opts, opts...)...)
}

func Diff[T any](a, b T, opts ...cmp.Option) string {
	return cmp.Diff(a, b, append(Opts, opts...)...)
}
