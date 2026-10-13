// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlerror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/require"
)

func Test_AsPosition(t *testing.T) {
	pos := lexer.Position{Line: 2, Column: 3, Offset: 4}
	converted := AsPosition(pos)

	require.Equal(t, pos.Line, converted.Line())
	require.Equal(t, pos.Column, converted.Column())
	require.Equal(t, pos.Offset, converted.Offset())
}

func Test_AsLexerPosition(t *testing.T) {
	pos := lexer.Position{Line: 2, Column: 3, Offset: 4}
	converted := AsPosition(pos)

	require.Equal(t, pos, AsLexerPosition(converted))
	require.Equal(t, lexer.Position{}, AsLexerPosition(nil))
}

func Test_Errorf(t *testing.T) {
	tests := []struct {
		name    string
		pos     lexer.Position
		wantErr string
	}{
		{
			name:    "line and column",
			pos:     lexer.Position{Line: 2, Column: 3, Offset: 4},
			wantErr: "2:3: invalid value 7",
		},
		{
			name:    "offset only",
			pos:     lexer.Position{Offset: 4},
			wantErr: "invalid value 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Errorf(tt.pos, "invalid value %d", 7)

			require.Equal(t, "invalid value 7", got.Message())
			require.Equal(t, tt.wantErr, got.Error())
			require.Equal(t, tt.pos, AsLexerPosition(got.Position()))
		})
	}
}

func Test_New(t *testing.T) {
	tests := []struct {
		name    string
		pos     lexer.Position
		wantErr string
	}{
		{
			name:    "line and column",
			pos:     lexer.Position{Line: 2, Column: 3, Offset: 4},
			wantErr: "2:3: invalid value",
		},
		{
			name:    "offset only",
			pos:     lexer.Position{Offset: 4},
			wantErr: "invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.pos, "invalid value")

			require.Equal(t, "invalid value", got.Message())
			require.Equal(t, tt.wantErr, got.Error())
			require.Equal(t, tt.pos, AsLexerPosition(got.Position()))
		})
	}
}

func Test_FromError_UsesProvidedPositionForPlainError(t *testing.T) {
	pos := lexer.Position{Line: 2, Column: 3, Offset: 4}
	err := errors.New("invalid value")

	got := FromError(pos, err)

	require.Equal(t, pos, AsLexerPosition(got.Position()))
	require.Equal(t, "invalid value", got.Message())
	require.EqualError(t, got, "2:3: invalid value")
	require.ErrorIs(t, got, err)
}

func Test_FromError_PreservesWrappedErrorPosition(t *testing.T) {
	innerPos := lexer.Position{Line: 2, Column: 3, Offset: 4}
	inner := Errorf(innerPos, "invalid value")
	wrapped := fmt.Errorf("wrapped: %w", inner)

	got := FromError(lexer.Position{Line: 10, Column: 11, Offset: 12}, wrapped)

	require.Equal(t, innerPos, AsLexerPosition(got.Position()))
	require.Equal(t, inner.Message(), got.Message())
	require.EqualError(t, got, inner.Error())
	require.ErrorIs(t, got, inner)
}

func Test_Wrap_PreservesWrappedErrorPosition(t *testing.T) {
	innerPos := lexer.Position{Line: 2, Column: 3, Offset: 4}
	inner := Errorf(innerPos, "invalid value")

	got := Wrap(lexer.Position{Line: 10, Column: 11, Offset: 12}, inner, "invalid argument")

	require.Equal(t, innerPos, AsLexerPosition(got.Position()))
	require.Equal(t, "invalid argument", got.Message())
	require.EqualError(t, got, "2:3: invalid argument")
	require.ErrorIs(t, got, inner)
}

func Test_Wrap_UsesProvidedPositionForPlainError(t *testing.T) {
	pos := lexer.Position{Line: 10, Column: 11, Offset: 12}
	err := errors.New("invalid value")

	got := Wrap(pos, err, "invalid argument")

	require.Equal(t, pos, AsLexerPosition(got.Position()))
	require.Equal(t, "invalid argument", got.Message())
	require.EqualError(t, got, "10:11: invalid argument")
	require.ErrorIs(t, got, err)
}

func Test_Wrapf_PreservesWrappedErrorPosition(t *testing.T) {
	innerPos := lexer.Position{Line: 2, Column: 3, Offset: 4}
	inner := Errorf(innerPos, "invalid value")
	wrapped := fmt.Errorf("wrapped: %w", inner)

	got := Wrapf(lexer.Position{Line: 10, Column: 11, Offset: 12}, wrapped, "argument %d", 1)

	require.Equal(t, innerPos, AsLexerPosition(got.Position()))
	require.Equal(t, "argument 1: invalid value", got.Message())
	require.EqualError(t, got, "2:3: argument 1: invalid value")
	require.ErrorIs(t, got, inner)
}

func Test_Wrapf_UsesProvidedPositionForPlainError(t *testing.T) {
	pos := lexer.Position{Line: 2, Column: 3, Offset: 4}
	err := errors.New("invalid value")

	got := Wrapf(pos, err, "argument %d", 1)

	require.Equal(t, pos, AsLexerPosition(got.Position()))
	require.Equal(t, "argument 1: invalid value", got.Message())
	require.EqualError(t, got, "2:3: argument 1: invalid value")
	require.ErrorIs(t, got, err)
}
