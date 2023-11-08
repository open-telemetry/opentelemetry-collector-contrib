package ottlfuncs

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "go.opentelemetry.io/collector/pkg/ottl"
)

func Test_IsInt(t *testing.T) {
    tests := []struct {
        name     string
        value    interface{}
        expected bool
    }{
        {
            name:     "int",
            value:    1,
            expected: true,
        },
        {
            name:     "int8",
            value:    int8(1),
            expected: true,
        },
        {
            name:     "int16",
            value:    int16(1),
            expected: true,
        },
        {
            name:     "int32",
            value:    int32(1),
            expected: true,
        },
        {
            name:     "int64",
            value:    int64(1),
            expected: true,
        },
        {
            name:     "uint",
            value:    uint(1),
            expected: true,
        },
        {
            name:     "uint8",
            value:    uint8(1),
            expected: true,
        },
        {
            name:     "uint16",
            value:    uint16(1),
            expected: true,
        },
        {
            name:     "uint32",
            value:    uint32(1),
            expected: true,
        },
        {
            name:     "uint64",
            value:    uint64(1),
            expected: true,
        },
        {
            name:     "float32",
            value:    float32(1.0),
            expected: false,
        },
        {
            name:     "float64",
            value:    float64(1.0),
            expected: false,
        },
        {
            name:     "string",
            value:    "1",
            expected: false,
        },
        {
            name:     "bool",
            value:    true,
            expected: false,
        },
        {
            name:     "nil",
            value:    nil,
            expected: false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            exprFunc := isInt[any](&ottl.StandardStringGetter[any]{
                Getter: func(context.Context, interface{}) (interface{}, error) {
                    return tt.value, nil
                },
            })
            result, err := exprFunc(context.Background(), nil)
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}

// nolint:errorlint
func Test_IsInt_Error(t *testing.T) {
    exprFunc := isInt[any](&ottl.StandardStringGetter[any]{
        Getter: func(context.Context, interface{}) (interface{}, error) {
            return nil, ottl.TypeError("")
        },
    })
    result, err := exprFunc(context.Background(), nil)
    assert.Equal(t, false, result)
    assert.Error(t, err)
    _, ok := err.(ottl.TypeError)
    assert.False(t, ok)
}
