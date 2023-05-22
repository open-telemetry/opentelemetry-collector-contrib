// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
