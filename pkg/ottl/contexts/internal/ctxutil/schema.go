// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package ctxutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"

//revive:disable:var-naming The methods in this interface are defined by pdata types.
type SchemaURLItem interface {
	SchemaUrl() string
	SetSchemaUrl(v string)
}

//revive:enable:var-naming
