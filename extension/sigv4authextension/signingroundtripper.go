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

// signingRoundTripper is a custom RoundTripper that performs AWS Sigv4.
type signingRoundTripper struct {
	transport     http.RoundTripper
	signer        *sigv4.Signer
	region        string
	service       string
	credsProvider *aws.CredentialsProvider
	awsSDKInfo    string
	logger        *zap.Logger
}

// RoundTrip() executes a single HTTP transaction and returns an HTTP response, signing
// the request with Sigv4.
func (si *signingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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

	// Hash the request
	h := sha256.New()
	_, _ = io.Copy(h, body)
	payloadHash := hex.EncodeToString(h.Sum(nil))

	// Use user provided service/region if specified, use inferred service/region if not, then sign the request
	service, region := si.inferServiceAndRegion(req)
	creds, err := (*si.credsProvider).Retrieve(req.Context())
	if err != nil {
		return nil, errBadCreds
	}
	err = si.signer.SignHTTP(req.Context(), creds, req2, payloadHash, service, region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error signing the request: %w", err)
	}

	// Send the request
	return si.transport.RoundTrip(req2)
}

// inferServiceAndRegion attempts to infer a service
// and a region from an http.request, and returns either an empty
// string for both or a valid value for both.
func (si *signingRoundTripper) inferServiceAndRegion(r *http.Request) (service string, region string) {
	service = si.service
	region = si.region

	h := r.Host
	if strings.HasPrefix(h, "aps-workspaces") {
		if service == "" {
			service = "aps"
		}
		rest := h[strings.Index(h, ".")+1:]
		if region == "" {
			region = rest[0:strings.Index(rest, ".")]
		}
	} else if strings.HasPrefix(h, "search-") {
		if service == "" {
			service = "es"
		}
		rest := h[strings.Index(h, ".")+1:]
		if region == "" {
			region = rest[0:strings.Index(rest, ".")]
		}
	} 
	
	if service == "" || region == "" {
		si.logger.Warn("Unable to infer region and/or service from the URL. Please provide values for region and/or service in the collector configuration.")
	}
	return service, region
}

// cloneRequest() is a helper function that makes a shallow copy of the request and a
// deep copy of the header, for thread safety purposes.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}
