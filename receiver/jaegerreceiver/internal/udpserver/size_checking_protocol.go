// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udpserver

import (
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
)

type sizeCheckingProtocol struct {
	thrift.TProtocol
}

// WrapProtocol wraps a thrift.TProtocol with container size checks.
func WrapProtocol(proto thrift.TProtocol) thrift.TProtocol {
	return &sizeCheckingProtocol{TProtocol: proto}
}

func (p *sizeCheckingProtocol) ReadListBegin(ctx context.Context) (elemType thrift.TType, size int, err error) {
	elemType, size, err = p.TProtocol.ReadListBegin(ctx)
	if err != nil {
		return elemType, size, err
	}
	if err := checkContainerSize(size, p.TProtocol.Transport()); err != nil {
		return elemType, 0, err
	}
	return elemType, size, nil
}

func (p *sizeCheckingProtocol) ReadSetBegin(ctx context.Context) (elemType thrift.TType, size int, err error) {
	elemType, size, err = p.TProtocol.ReadSetBegin(ctx)
	if err != nil {
		return elemType, size, err
	}
	if err := checkContainerSize(size, p.TProtocol.Transport()); err != nil {
		return elemType, 0, err
	}
	return elemType, size, nil
}

func (p *sizeCheckingProtocol) ReadMapBegin(ctx context.Context) (keyType thrift.TType, valType thrift.TType, size int, err error) {
	keyType, valType, size, err = p.TProtocol.ReadMapBegin(ctx)
	if err != nil {
		return keyType, valType, size, err
	}
	if err := checkContainerSize(size, p.TProtocol.Transport()); err != nil {
		return keyType, valType, 0, err
	}
	return keyType, valType, size, nil
}

func checkContainerSize(size int, trans thrift.TTransport) error {
	if size < 0 {
		return thrift.NewTProtocolExceptionWithType(thrift.NEGATIVE_SIZE, fmt.Errorf("negative container size: %d", size))
	}
	if size == 0 {
		return nil
	}

	// Check remaining bytes if the transport supports it
	if sizeProvider, ok := trans.(interface{ RemainingBytes() uint64 }); ok {
		remaining := sizeProvider.RemainingBytes()
		// Each element in a container must take at least 1 byte.
		// If the size is greater than the remaining bytes in the transport, it's invalid.
		if uint64(size) > remaining {
			return thrift.NewTProtocolExceptionWithType(thrift.SIZE_LIMIT, fmt.Errorf("container size %d exceeds remaining bytes %d", size, remaining))
		}
	} else {
		// Fallback safety limit for transports that don't provide remaining bytes
		const maxContainerSize = 10_000_000
		if size > maxContainerSize {
			return thrift.NewTProtocolExceptionWithType(thrift.SIZE_LIMIT, fmt.Errorf("container size %d exceeds fallback max limit %d", size, maxContainerSize))
		}
	}
	return nil
}
