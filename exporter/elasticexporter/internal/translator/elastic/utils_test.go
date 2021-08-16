// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elastic_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/transport"
	"go.elastic.co/fastjson"
)

func sendStream(t *testing.T, w *fastjson.Writer, transport transport.Transport) {
	var buf bytes.Buffer
	zw, err := zlib.NewWriterLevel(&buf, zlib.DefaultCompression)
	require.NoError(t, err)
	_, err = zw.Write(w.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, transport.SendStream(context.Background(), &buf))
}
