// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs/handler"

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go/aws/request"
	"go.uber.org/zap"
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

func NewRequestCompressionHandler(opNames []string, logger *zap.Logger) request.NamedHandler {
	return request.NamedHandler{
		Name: "RequestCompressionHandler",
		Fn: func(req *request.Request) {
			match := false
			for _, opName := range opNames {
				if req.Operation.Name == opName {
					match = true
				}
			}

			if !match {
				return
			}

			buf := new(bytes.Buffer)
			g := gzipPool.Get().(*gzip.Writer)
			g.Reset(buf)
			size, err := io.Copy(g, req.GetBody())
			if err != nil {
				logger.Info("I! Error occurred when trying to compress payload, uncompressed request is sent, error:",
					zap.String("Operation", req.Operation.Name), zap.Error(err))
				req.ResetBody()
				return
			}
			g.Close()
			compressedSize := int64(buf.Len())

			if size <= compressedSize {
				logger.Debug("The payload is not compressed.",
					zap.Int64("original payload size", size), zap.Int64("compressed payload size", compressedSize))
				req.ResetBody()
				return
			}

			req.SetBufferBody(buf.Bytes())
			gzipPool.Put(g)
			req.HTTPRequest.ContentLength = compressedSize
			req.HTTPRequest.Header.Set("Content-Length", fmt.Sprintf("%d", compressedSize))
			req.HTTPRequest.Header.Set("Content-Encoding", "gzip")
		},
	}
}
