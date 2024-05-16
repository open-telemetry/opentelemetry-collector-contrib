// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal

type SchemaURLItem interface {
	SchemaUrl() string
	SetSchemaUrl(v string)
}
