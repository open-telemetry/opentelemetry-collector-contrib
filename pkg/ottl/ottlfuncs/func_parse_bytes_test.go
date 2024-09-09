// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseBytes(t *testing.T) {
	exprFunc := parseBytesFunc[any](&ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "1KB", nil
		},
	})

	result, err := exprFunc(nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, int64(1000), result)
}

func Test_ParseBytes_InvalidError(t *testing.T) {
	exprFunc := parseBytesFunc[any](&ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "not a byte string", nil
		},
	})

	result, err := exprFunc(nil, nil)

	assert.Nil(t, result)
	assert.Error(t, err)
}

func Test_ParseBytes_TooLargeError(t *testing.T) {
	exprFunc := parseBytesFunc[any](&ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "9223372037GB", nil
		},
	})

	result, err := exprFunc(nil, nil)

	assert.Nil(t, result)
	assert.Error(t, err)
}
