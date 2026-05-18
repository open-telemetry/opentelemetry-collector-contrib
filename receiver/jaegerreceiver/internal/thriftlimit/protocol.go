// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package thriftlimit

import (
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
)

const maxCollectionElements = 1 << 20 // Allocation guard for untrusted thrift collection counts.

type protocol struct {
	thrift.TProtocol
}

type protocolFactory struct {
	delegate thrift.TProtocolFactory
}

// NewProtocol wraps delegate with Jaeger-local collection size limits.
func NewProtocol(delegate thrift.TProtocol) thrift.TProtocol {
	return &protocol{TProtocol: delegate}
}

// NewProtocolFactory wraps protocols from delegate with Jaeger-local collection size limits.
func NewProtocolFactory(delegate thrift.TProtocolFactory) thrift.TProtocolFactory {
	return &protocolFactory{delegate: delegate}
}

func (f *protocolFactory) GetProtocol(trans thrift.TTransport) thrift.TProtocol {
	return NewProtocol(f.delegate.GetProtocol(trans))
}

func (f *protocolFactory) SetTConfiguration(conf *thrift.TConfiguration) {
	thrift.PropagateTConfiguration(f.delegate, conf)
}

func (p *protocol) ReadMapBegin(ctx context.Context) (thrift.TType, thrift.TType, int, error) {
	keyType, valueType, size, err := p.TProtocol.ReadMapBegin(ctx)
	if err != nil {
		return keyType, valueType, size, err
	}
	if err := p.checkCollectionSize("map", size); err != nil {
		return keyType, valueType, size, err
	}
	return keyType, valueType, size, nil
}

func (p *protocol) ReadListBegin(ctx context.Context) (thrift.TType, int, error) {
	elemType, size, err := p.TProtocol.ReadListBegin(ctx)
	if err != nil {
		return elemType, size, err
	}
	if err := p.checkCollectionSize("list", size); err != nil {
		return elemType, size, err
	}
	return elemType, size, nil
}

func (p *protocol) ReadSetBegin(ctx context.Context) (thrift.TType, int, error) {
	elemType, size, err := p.TProtocol.ReadSetBegin(ctx)
	if err != nil {
		return elemType, size, err
	}
	if err := p.checkCollectionSize("set", size); err != nil {
		return elemType, size, err
	}
	return elemType, size, nil
}

func (p *protocol) Skip(ctx context.Context, fieldType thrift.TType) error {
	return thrift.SkipDefaultDepth(ctx, p, fieldType)
}

func (p *protocol) SetTConfiguration(conf *thrift.TConfiguration) {
	thrift.PropagateTConfiguration(p.TProtocol, conf)
}

func (p *protocol) Reset() {
	if resetter, ok := p.TProtocol.(interface{ Reset() }); ok {
		resetter.Reset()
	}
}

func (p *protocol) checkCollectionSize(kind string, size int) error {
	if size < 0 {
		return thrift.NewTProtocolExceptionWithType(thrift.NEGATIVE_SIZE, fmt.Errorf("thrift %s declares negative size %d", kind, size))
	}
	if size > maxCollectionElements {
		return thrift.NewTProtocolExceptionWithType(thrift.SIZE_LIMIT, fmt.Errorf("thrift %s declares %d elements, limit is %d", kind, size, maxCollectionElements))
	}
	if remaining := p.Transport().RemainingBytes(); uint64(size) > remaining {
		return thrift.NewTProtocolExceptionWithType(thrift.SIZE_LIMIT, fmt.Errorf("thrift %s declares %d elements, only %d payload bytes remain", kind, size, remaining))
	}
	return nil
}

var (
	_ thrift.TProtocol            = (*protocol)(nil)
	_ thrift.TProtocolFactory     = (*protocolFactory)(nil)
	_ thrift.TConfigurationSetter = (*protocol)(nil)
	_ thrift.TConfigurationSetter = (*protocolFactory)(nil)
)
