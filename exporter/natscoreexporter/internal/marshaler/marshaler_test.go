// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

type mockGenericMarshaler struct{}

func (m *mockGenericMarshaler) MarshalString(sd string) ([]byte, error) {
	return []byte(sd), nil
}

var _ GenericMarshaler = (*mockGenericMarshaler)(nil)

type mockResolver struct{}

func (r *mockResolver) Resolve(host component.Host) (GenericMarshaler, error) {
	return &mockGenericMarshaler{}, nil
}

var _ Resolver = (*mockResolver)(nil)

func mockPick(genericMarshaler GenericMarshaler) (MarshalFunc[string], error) {
	mockGenericMarshaler, ok := genericMarshaler.(*mockGenericMarshaler)
	if !ok {
		return nil, errors.New("genericMarshaler is not a mockGenericMarshaler")
	}
	return mockGenericMarshaler.MarshalString, nil
}

var _ PickFunc[string] = mockPick

func TestMarshaler(t *testing.T) {
	t.Parallel()

	t.Run("composes resolver and pick", func(t *testing.T) {
		marshaler := NewMarshaler(&mockResolver{}, mockPick)

		err := marshaler.Resolve(componenttest.NewNopHost())
		assert.NoError(t, err)

		marshaled, err := marshaler.Marshal("test")
		assert.NoError(t, err)
		assert.Equal(t, []byte("test"), marshaled)
	})
}
