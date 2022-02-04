// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sigv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"go.uber.org/zap"
)

// SigningRoundTripper is a custom RoundTripper that performs AWS Sigv4.
type SigningRoundTripper struct {
	transport  http.RoundTripper
	signer     *sigv4.Signer
	region     string
	service    string
	creds      *aws.Credentials
	awsSDKInfo string
	logger     *zap.Logger
}

// RoundTrip() executes a single HTTP transaction and returns an HTTP response, signing
// the request with Sigv4.
func (si *SigningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(reqBody)
	reqBody.Close()
	if err != nil {
		return nil, err
	}
	body := bytes.NewReader(content)

	// Clone request to ensure thread safety.
	req2 := cloneRequest(req)

	// Add the runtime information to the User-Agent header of the request
	ua := req2.Header.Get("User-Agent")
	if len(ua) > 0 {
		ua = ua + " " + si.awsSDKInfo
	} else {
		ua = si.awsSDKInfo
	}
	req2.Header.Set("User-Agent", ua)

	// Sign the request
	h := sha256.New()
	_, _ = io.Copy(h, body)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	// Use user provided service/region if specified, use inferred service/region if not
	service, region := si.inferServiceAndRegionFromRequestURL(req)
	if si.service != "" && si.region != "" {
		err = si.signer.SignHTTP(context.Background(), *si.creds, req2, payloadHash, si.service, si.region, time.Now())
	} else if si.service != "" && si.region == "" {
		err = si.signer.SignHTTP(context.Background(), *si.creds, req2, payloadHash, si.service, region, time.Now())
	} else if si.service == "" && si.region != "" {
		err = si.signer.SignHTTP(context.Background(), *si.creds, req2, payloadHash, service, si.region, time.Now())
	} else {
		err = si.signer.SignHTTP(context.Background(), *si.creds, req2, payloadHash, service, region, time.Now())
	}
	if err != nil {
		return nil, fmt.Errorf("error signing the request: %v", err)
	}

	// Send the request
	resp, err := si.transport.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// inferServiceAndRegionFromRequestURL attempts to infer a service
// and a region from an http.request, and returns either an empty
// string for both or a valid value for both.
func (si *SigningRoundTripper) inferServiceAndRegionFromRequestURL(r *http.Request) (service string, region string) {
	h := r.Host
	if strings.HasPrefix(h, "aps-workspaces") {
		service = "aps"
		rest := h[strings.Index(h, ".")+1:]
		region = rest[0:strings.Index(rest, ".")]
	} else if strings.HasPrefix(h, "search-") {
		service = "es"
		rest := h[strings.Index(h, ".")+1:]
		region = rest[0:strings.Index(rest, ".")]
	} else {
		si.logger.Warn("User must explicitly set service and region in configuration")
	}
	return service, region
}
