// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkin // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinthriftconverter"

import (
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
)

const maxThriftCollectionElements = 1 << 20

// sizeLimitedProtocol rejects forged collection sizes before generated thrift readers preallocate slices.
type sizeLimitedProtocol struct {
	thrift.TProtocol
}

func newSizeLimitedBinaryProtocol(buffer *thrift.TMemoryBuffer) thrift.TProtocol {
	return newSizeLimitedProtocol(thrift.NewTBinaryProtocolConf(buffer, &thrift.TConfiguration{}))
}

func newSizeLimitedProtocol(delegate thrift.TProtocol) thrift.TProtocol {
	return &sizeLimitedProtocol{TProtocol: delegate}
}

func (p *sizeLimitedProtocol) ReadMapBegin(ctx context.Context) (thrift.TType, thrift.TType, int, error) {
	keyType, valueType, size, err := p.TProtocol.ReadMapBegin(ctx)
	if err != nil {
		return keyType, valueType, size, err
	}
	if err := p.checkCollectionSize("map", size, 2); err != nil {
		return keyType, valueType, size, err
	}
	return keyType, valueType, size, nil
}

func (p *sizeLimitedProtocol) ReadListBegin(ctx context.Context) (thrift.TType, int, error) {
	elemType, size, err := p.TProtocol.ReadListBegin(ctx)
	if err != nil {
		return elemType, size, err
	}
	if err := p.checkCollectionSize("list", size, 1); err != nil {
		return elemType, size, err
	}
	return elemType, size, nil
}

func (p *sizeLimitedProtocol) ReadSetBegin(ctx context.Context) (thrift.TType, int, error) {
	elemType, size, err := p.TProtocol.ReadSetBegin(ctx)
	if err != nil {
		return elemType, size, err
	}
	if err := p.checkCollectionSize("set", size, 1); err != nil {
		return elemType, size, err
	}
	return elemType, size, nil
}

func (p *sizeLimitedProtocol) Skip(ctx context.Context, fieldType thrift.TType) error {
	return thrift.SkipDefaultDepth(ctx, p, fieldType)
}

func (p *sizeLimitedProtocol) SetTConfiguration(conf *thrift.TConfiguration) {
	thrift.PropagateTConfiguration(p.TProtocol, conf)
}

func (p *sizeLimitedProtocol) Reset() {
	if resetter, ok := p.TProtocol.(interface{ Reset() }); ok {
		resetter.Reset()
	}
}

func (p *sizeLimitedProtocol) checkCollectionSize(kind string, size, minElementBytes int) error {
	if size < 0 {
		return thrift.NewTProtocolExceptionWithType(thrift.NEGATIVE_SIZE, fmt.Errorf("thrift %s declares negative size %d", kind, size))
	}
	if size > maxThriftCollectionElements {
		return thrift.NewTProtocolExceptionWithType(thrift.SIZE_LIMIT, fmt.Errorf("thrift %s declares %d elements, limit is %d", kind, size, maxThriftCollectionElements))
	}
	if size == 0 {
		return nil
	}
	if minElementBytes <= 0 {
		minElementBytes = 1
	}
	if remaining := p.Transport().RemainingBytes(); uint64(size)*uint64(minElementBytes) > remaining {
		return thrift.NewTProtocolExceptionWithType(thrift.SIZE_LIMIT, fmt.Errorf("thrift %s declares %d elements, only %d payload bytes remain", kind, size, remaining))
	}
	return nil
}

var (
	_ thrift.TProtocol            = (*sizeLimitedProtocol)(nil)
	_ thrift.TConfigurationSetter = (*sizeLimitedProtocol)(nil)
)
