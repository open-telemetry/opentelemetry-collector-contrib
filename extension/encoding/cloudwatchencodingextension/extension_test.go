// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencodingextension

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestExtension_Start_Shutdown(t *testing.T) {
	extension := &cloudwatchExtension{}

	err := extension.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = extension.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestDecompress(t *testing.T) {
	tests := map[string]struct {
		encodings []contentEncoding
		buf       []byte
		result    []byte
		err       error
	}{
		base64Encoding: {
			encodings: []contentEncoding{base64Encoding},
			buf:       []byte("dGVzdA=="),
			result:    []byte("test"),
		},
		noEncoding: {
			encodings: []contentEncoding{},
			buf:       []byte("test"),
			result:    []byte("test"),
		},
		fmt.Sprintf("%s+%s", base64Encoding, gzipEncoding): {
			encodings: []contentEncoding{base64Encoding, gzipEncoding},
			buf:       []byte("H4sIAAAAAAAAAytJLS4BAAx+f9gEAAAA"),
			result:    []byte("test"),
		},
		"invalid": {
			encodings: []contentEncoding{"invalid"},
			buf:       []byte("test"),
			result:    nil,
			err:       errors.New("invalid content encoding"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := decompress(test.buf, test.encodings)
			//fmt.Println(string(result))
			require.Equal(t, test.err, err)
			require.Equal(t, test.result, result)
		})
	}

}
