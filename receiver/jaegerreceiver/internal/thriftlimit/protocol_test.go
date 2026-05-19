// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package thriftlimit

import (
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolRejectsOversizedCollections(t *testing.T) {
	factories := map[string]thrift.TProtocolFactory{
		"binary":  thrift.NewTBinaryProtocolFactoryConf(nil),
		"compact": thrift.NewTCompactProtocolFactoryConf(nil),
	}

	for factoryName, factory := range factories {
		for collectionName, collection := range map[string]struct {
			write func(thrift.TProtocol) error
			read  func(thrift.TProtocol) error
		}{
			"list": {
				write: func(p thrift.TProtocol) error {
					return p.WriteListBegin(t.Context(), thrift.STRUCT, maxCollectionElements+1)
				},
				read: func(p thrift.TProtocol) error {
					_, _, err := p.ReadListBegin(t.Context())
					return err
				},
			},
			"map": {
				write: func(p thrift.TProtocol) error {
					return p.WriteMapBegin(t.Context(), thrift.STRING, thrift.STRUCT, maxCollectionElements+1)
				},
				read: func(p thrift.TProtocol) error {
					_, _, _, err := p.ReadMapBegin(t.Context())
					return err
				},
			},
			"set": {
				write: func(p thrift.TProtocol) error {
					return p.WriteSetBegin(t.Context(), thrift.STRUCT, maxCollectionElements+1)
				},
				read: func(p thrift.TProtocol) error {
					_, _, err := p.ReadSetBegin(t.Context())
					return err
				},
			},
		} {
			t.Run(factoryName+"/"+collectionName, func(t *testing.T) {
				trans := thrift.NewTMemoryBuffer()
				require.NoError(t, collection.write(factory.GetProtocol(trans)))

				err := collection.read(NewProtocolFactory(factory).GetProtocol(trans))
				require.Error(t, err)
				assert.Contains(t, err.Error(), "declares 1048577 elements")
			})
		}
	}
}

func TestProtocolRejectsCollectionLargerThanRemainingPayload(t *testing.T) {
	trans := thrift.NewTMemoryBuffer()
	require.NoError(t, thrift.NewTBinaryProtocolConf(trans, &thrift.TConfiguration{}).WriteListBegin(t.Context(), thrift.STRUCT, 1))

	_, _, err := NewProtocol(thrift.NewTBinaryProtocolConf(trans, &thrift.TConfiguration{})).ReadListBegin(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only 0 payload bytes remain")
}
