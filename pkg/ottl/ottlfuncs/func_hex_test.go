// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestHex(t *testing.T) {
	type args struct {
		target ottl.ByteSliceLikeGetter[any]
	}
	type testCase struct {
		name     string
		args     args
		wantFunc func() any
		wantErr  error
	}
	tests := []testCase{
		{
			name: "int64",
			args: args{
				target: &ottl.StandardByteSliceLikeGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return int64(12), nil
					},
				},
			},
			wantFunc: func() any {
				return "000000000000000c"
			},
		},
		{
			name: "nil",
			args: args{
				target: &ottl.StandardByteSliceLikeGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return nil, nil
					},
				},
			},
			wantFunc: func() any {
				return ""
			},
			wantErr: nil,
		},
		{
			name: "error",
			args: args{
				target: &ottl.StandardByteSliceLikeGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return map[string]string{"hi": "hi"}, nil
					},
				},
			},
			wantFunc: func() any {
				return nil
			},
			wantErr: ottl.TypeError("unsupported type: map[string]string"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, _ := Hex(tt.args.target)
			got, err := expressionFunc(context.Background(), tt.args)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantFunc(), got)
		})
	}
}
