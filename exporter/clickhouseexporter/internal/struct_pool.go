// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

type ExporterStructPool[T any] struct {
	pool chan T
}

func NewExporterStructPool[T any](poolSize int, newInstance func() (T, error)) (*ExporterStructPool[T], error) {
	pool := ExporterStructPool[T]{
		pool: make(chan T, poolSize),
	}

	for i := 0; i < poolSize; i++ {
		instance, err := newInstance()
		if err != nil {
			return nil, err
		}

		pool.pool <- instance
	}

	return &pool, nil
}

func (p *ExporterStructPool[T]) Acquire() T {
	return <-p.pool
}

func (p *ExporterStructPool[T]) Release(v T) {
	p.pool <- v
}

func (p *ExporterStructPool[T]) Destroy() {
	for i := 0; i < cap(p.pool); i++ {
		<-p.pool
	}

	close(p.pool)
}
