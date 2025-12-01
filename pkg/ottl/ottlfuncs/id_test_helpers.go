// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type idExprBuilder func(ottl.ByteSliceLikeGetter[any]) ottl.ExprFunc[any]

type idSuccessTestCase struct {
	name  string
	value []byte
	want  any
}

type idErrorTestCase struct {
	name  string
	value []byte
	err   error
}

// makeIDGetter creates a ByteSliceLikeGetter for testing purposes.
// This is a shared helper used by TraceID, SpanID, and ProfileID tests.
func makeIDGetter(bytes []byte) ottl.ByteSliceLikeGetter[any] {
	return &idGetter{bytes: bytes}
}

type idGetter struct {
	bytes []byte
}

func (t *idGetter) Get(_ context.Context, _ any) ([]byte, error) {
	return t.bytes, nil
}

func runIDSuccessTests(t *testing.T, builder idExprBuilder, cases []idSuccessTestCase) {
	t.Helper()

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			expr := builder(makeIDGetter(tt.value))
			result, err := expr(context.Background(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func runIDErrorTests(t *testing.T, builder idExprBuilder, cases []idErrorTestCase) {
	t.Helper()

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			expr := builder(makeIDGetter(tt.value))
			result, err := expr(context.Background(), nil)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, tt.err)
		})
	}
}
