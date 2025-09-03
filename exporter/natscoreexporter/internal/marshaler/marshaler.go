// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"

import "go.opentelemetry.io/collector/component"

type GenericMarshaler any

type Resolver interface {
	Resolve(host component.Host) (GenericMarshaler, error)
}

type MarshalFunc[T any] func(data T) ([]byte, error)

type PickFunc[T any] func(genericMarshaler GenericMarshaler) (MarshalFunc[T], error)

type Marshaler[T any] struct {
	resolver Resolver
	pick     PickFunc[T]
	marshal  MarshalFunc[T]
}

func (m *Marshaler[T]) Resolve(host component.Host) error {
	genericMarshaler, err := m.resolver.Resolve(host)
	if err != nil {
		return err
	}

	m.marshal, err = m.pick(genericMarshaler)
	if err != nil {
		return err
	}

	return nil
}

func (m *Marshaler[T]) Marshal(data T) ([]byte, error) {
	return m.marshal(data)
}

func NewMarshaler[T any](resolver Resolver, pick PickFunc[T]) Marshaler[T] {
	return Marshaler[T]{
		resolver: resolver,
		pick:     pick,
	}
}
