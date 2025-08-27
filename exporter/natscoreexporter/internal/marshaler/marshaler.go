// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"

type Marshaler[T any] interface {
	Marshal(data T) ([]byte, error)
}

type NewMarshalerFunc[T any] func() (Marshaler[T], error)
