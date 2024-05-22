// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlcommon

type SchemaURLItem interface {
	SchemaUrl() string
	SetSchemaUrl(v string)
}
