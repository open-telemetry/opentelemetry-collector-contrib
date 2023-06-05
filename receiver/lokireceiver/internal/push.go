// Copyright The OpenTelemetry Authors
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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver/internal"

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"

	"github.com/grafana/loki/pkg/push"
)

var (
	contentType = http.CanonicalHeaderKey("Content-Type")
	contentEnc  = http.CanonicalHeaderKey("Content-Encoding")
)

const applicationJSON = "application/json"

func ParseRequest(req *http.Request) (*push.PushRequest, error) {
	var body io.Reader
	contentEncoding := req.Header.Get(contentEnc)

	switch contentEncoding {
	case "", "snappy":
		body = req.Body
	case "gzip":
		gzipReader, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		body = gzipReader
	case "deflate":
		flateReader := flate.NewReader(req.Body)
		defer flateReader.Close()
		body = flateReader
	default:
		return nil, fmt.Errorf("Content-Encoding %q not supported", contentEncoding)
	}

	var pushRequest push.PushRequest
	reqContentType := req.Header.Get(contentType)
	reqContentType, _ /* params */, err := mime.ParseMediaType(reqContentType)
	if err != nil {
		return nil, err
	}

	switch reqContentType {
	case applicationJSON:
		if err = decodePushRequest(body, &pushRequest); err != nil {
			return nil, err
		}

	default:
		// When no content-type header is set or when it is set to
		// `application/x-protobuf`: expect snappy compression.
		if err := parseProtoReader(body, int(req.ContentLength), math.MaxInt32, &pushRequest); err != nil {
			return nil, err
		}
		return &pushRequest, nil
	}

	return &pushRequest, nil
}
