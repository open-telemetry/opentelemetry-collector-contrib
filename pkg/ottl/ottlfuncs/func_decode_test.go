package ottlfuncs

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"testing"
)

func TestDecode(t *testing.T) {
	testString := "test string"
	encodings := []string{"us-ascii", "ISO-8859-1", "WINDOWS-1251", "WINDOWS-1252", "UTF-8", "GB2312"}

	for _, enc := range encodings {
		encoder, err := ianaindex.IANA.Encoding(enc)
		require.NoError(t, err)
		if encoder == nil {
			fmt.Println(fmt.Sprintf(`{ name:"decode %s encoded string; no decoder available", value: "%s", encoding: "%s", want: nil, wantErr: true},`, enc, testString, enc))
		} else {
			s, err := encoding.ReplaceUnsupported(encoder.NewEncoder()).String(testString)
			require.NoError(t, err)
			fmt.Println(fmt.Sprintf(`{ name:"decode %s encoded string", value: "%s", encoding: "%s", want: "%s", wantErr: false},`, enc, s, enc, testString))
		}

	}
	type testCase struct {
		name     string
		value    any
		encoding string
		want     any
		wantErr  bool
	}
	tests := []testCase{
		{
			name:     "convert base64 byte array",
			value:    []byte("dGVzdAo="),
			encoding: "base64",
			want:     "test\n",
			wantErr:  false,
		},
		{
			name:     "convert base64 string",
			value:    "dGVzdAo=",
			encoding: "base64",
			want:     "test\n",
			wantErr:  false,
		},
		{name: "decode us-ascii encoded string", value: "test string", encoding: "us-ascii", want: "test string", wantErr: false},
		{name: "decode ISO-8859-1 encoded string", value: "test string", encoding: "ISO-8859-1", want: "test string", wantErr: false},
		{name: "decode WINDOWS-1251 encoded string", value: "test string", encoding: "WINDOWS-1251", want: "test string", wantErr: false},
		{name: "decode WINDOWS-1252 encoded string", value: "test string", encoding: "WINDOWS-1252", want: "test string", wantErr: false},
		{name: "decode UTF-8 encoded string", value: "test string", encoding: "UTF-8", want: "test string", wantErr: false},
		{name: "decode GB2312 encoded string; no decoder available", value: "test string", encoding: "GB2312", want: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionFunc, err := createDecodeFunction[any](ottl.FunctionContext{}, &DecodeArguments[any]{
				Target: &ottl.StandardGetSetter[any]{
					Getter: func(ctx context.Context, tCtx any) (any, error) {
						return tt.value, nil
					},
				},
				Encoding: tt.encoding,
			})

			require.Nil(t, err)

			result, err := expressionFunc(nil, nil)
			require.NoError(t, err)

			require.Equal(t, tt.want, result)
		})
	}
}
